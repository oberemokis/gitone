package git

import (
	"os"
	"strings"

	"gitone/internal/i18n"
)

// Commit записывает индекс с темой subject и описанием description. Описание
// идёт вторым -m, git ставит между ним и темой пустую строку; пробелы по краям
// обрезаются, а пустое описание тела не даёт. Идёт обычным git commit: хуки
// выполняются, автор берётся из git config, редактор не открывается. Отказ хука
// приходит ошибкой git. Пустой индекс проверяется заранее: на пустом git commit
// пишет отчёт `git status`, и причина стоит в нём не первой строкой. Пока идёт
// слияние, cherry-pick или revert, проверки нет.
func Commit(dir string, subject string, description string) error {
	return failed("commit", commit(dir, subject, description))
}

func commit(dir string, subject string, description string) error {
	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	// код 0 значит, что индекс не отличается от HEAD. Любая другая ошибка
	// проверки идёт дальше: git commit назовёт причину сам. Незавершённое
	// слияние, cherry-pick или revert коммитят и с индексом, равным HEAD
	// (`merge -s ours`): их git commit решает сам, отказ приходит его ошибкой.
	if !inProgress(root) {
		if _, err := run(call{dir: root}, "diff", "--cached", "--quiet"); err == nil {
			return i18n.New("git.nothingToCommit", nil)
		}
	}

	_, err = run(call{dir: root}, messageArgs([]string{"commit"}, subject, description)...)
	return err
}

// messageArgs добавляет к команде тему и описание вторым -m: git ставит между
// ними пустую строку, а пустое описание тела не даёт.
func messageArgs(args []string, subject, description string) []string {
	args = append(args, "-m", subject)
	if body := strings.TrimSpace(description); body != "" {
		args = append(args, "-m", body)
	}
	return args
}

// LastMessage возвращает тему и описание последнего коммита (#218): окно
// amend открывается с ними. В репозитории без коммитов исправлять нечего.
func LastMessage(dir string) (subject, description string, err error) {
	out, err := inRoot(dir, "log", "-1", "--format=%s%x00%b")
	if err != nil {
		if !hasCommits(dir) {
			err = i18n.New("git.noCommits", nil)
		}
		return "", "", failed("log", err)
	}

	subject, description, _ = strings.Cut(strings.TrimRight(out, "\n"), "\x00")
	return subject, strings.TrimSpace(description), nil
}

// Amend переписывает последний коммит (#218): индекс входит в него, тема и
// описание заменяются. published говорит, что прежний коммит уже лежал в
// upstream: после amend ветка разошлась с remote, и обычный push её не
// отправит. Хуки выполняются, как у Commit.
func Amend(dir, subject, description string) (published bool, err error) {
	published, err = amend(dir, subject, description)
	return published, failed("commit", err)
}

func amend(dir, subject, description string) (bool, error) {
	root, err := topLevel(dir)
	if err != nil {
		return false, err
	}

	_, pushedErr := run(call{dir: root}, "merge-base", "--is-ancestor", "HEAD", "@{upstream}")
	_, err = run(call{dir: root}, messageArgs([]string{"commit", "--amend"}, subject, description)...)
	return err == nil && pushedErr == nil, headline(err)
}

// inProgress говорит, что в репозитории не завершены слияние, cherry-pick или
// revert: об этом знают псевдоссылки MERGE_HEAD, CHERRY_PICK_HEAD и REVERT_HEAD.
func inProgress(root string) bool {
	for _, head := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD"} {
		if _, err := run(call{dir: root}, "rev-parse", "--quiet", "--verify", head); err == nil {
			return true
		}
	}

	return false
}

// rebasing говорит, что rebase остановился и ждёт продолжения: git держит его
// состояние в каталоге rebase-merge (или rebase-apply у старого движка).
func rebasing(root string) bool {
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		path, err := run(call{dir: root}, "rev-parse", "--path-format=absolute", "--git-path", name)
		if err != nil {
			continue
		}
		if _, err := os.Stat(strings.TrimSpace(path)); err == nil {
			return true
		}
	}

	return false
}

// unfinished говорит, что операция git остановилась посредине: слияние,
// cherry-pick, revert или rebase.
func unfinished(root string) bool {
	return inProgress(root) || rebasing(root)
}

// stopping запускает операцию, которая может остановиться на конфликте, из
// корня репозитория. Остановку она сводит к ошибке из словаря: строка git
// называет один файл, а знать нужно, что операция не закончена. Отказ до
// начала (грязное дерево, незавершённая прежняя операция) сводится к одной
// строке (headline).
func stopping(dir, operation string, args ...string) error {
	root, err := topLevel(dir)
	if err != nil {
		return err
	}
	if unfinished(root) {
		_, err = run(call{dir: root}, args...)
		return headline(err)
	}

	_, err = run(call{dir: root}, args...)
	if err != nil && unfinished(root) {
		return i18n.New("git.stoppedOnConflict", i18n.Args{"operation": operation})
	}
	return headline(err)
}

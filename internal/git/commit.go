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
func (r Repo) Commit(subject string, description string) error {
	return failed("commit", r.commit(subject, description))
}

func (r Repo) commit(subject string, description string) error {
	// код 0 значит, что индекс не отличается от HEAD. Любая другая ошибка
	// проверки идёт дальше: git commit назовёт причину сам. Незавершённое
	// слияние, cherry-pick или revert коммитят и с индексом, равным HEAD
	// (`merge -s ours`): их git commit решает сам, отказ приходит его ошибкой.
	if !inProgress(r.root) {
		if _, err := r.inRoot("diff", "--cached", "--quiet"); err == nil {
			return i18n.New("git.nothingToCommit", nil)
		}
	}

	_, err := r.inRoot(messageArgs([]string{"commit"}, subject, description)...)
	return err
}

// messageArgs добавляет к команде тему и описание вторым -m: git ставит между
// ними пустую строку, а пустое описание тела не даёт. Режим очистки закреплён:
// commit.cleanup=strip из конфига пользователя вырезал бы тему «#42 fix crash»,
// а у amend оставил бы пустое сообщение (#381).
func messageArgs(args []string, subject, description string) []string {
	args = append(args, "--cleanup=whitespace", "-m", subject)
	if body := strings.TrimSpace(description); body != "" {
		args = append(args, "-m", body)
	}
	return args
}

// LastMessage возвращает тему и описание последнего коммита (#218): окно
// amend открывается с ними. В репозитории без коммитов исправлять нечего.
func (r Repo) LastMessage() (subject, description string, err error) {
	return r.Message("HEAD")
}

// Message возвращает тему и описание коммита rev: amend и reword (#219)
// открывают окно коммита с ними. Тема это первая строка сообщения, описание
// всё после неё: %s склеил бы первый абзац из нескольких строк в одну. Без
// --no-show-signature log.showSignature ставит проверку подписи первой строкой,
// и amend без правок записал бы её в историю (#381).
func (r Repo) Message(rev string) (subject, description string, err error) {
	out, err := r.inRoot("log", "-1", "--no-show-signature", "--format=%B", rev, "--")
	if err != nil {
		if !hasCommits(r.root) {
			err = i18n.New("git.noCommits", nil)
		}
		return "", "", failed("log", err)
	}

	subject, description, _ = strings.Cut(strings.TrimRight(out, "\n"), "\n")
	return subject, strings.TrimSpace(description), nil
}

// Amend переписывает последний коммит (#218): индекс входит в него, тема и
// описание заменяются. Пустая тема оставляет сообщение как есть, байт в байт
// (--no-edit). published говорит, что прежний коммит уже лежал в upstream:
// после amend ветка разошлась с remote, и обычный push её не отправит. Хуки
// выполняются, как у Commit.
func (r Repo) Amend(subject, description string) (published bool, err error) {
	published, err = r.amend(subject, description)
	return published, failed("commit", err)
}

func (r Repo) amend(subject, description string) (bool, error) {
	args := []string{"commit", "--amend", "--no-edit"}
	if subject != "" {
		args = messageArgs([]string{"commit", "--amend"}, subject, description)
	}
	_, pushedErr := r.inRoot("merge-base", "--is-ancestor", "HEAD", "@{upstream}")
	_, err := r.inRoot(args...)
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
// состояние в каталоге rebase-merge (или rebase-apply у старого движка). Тот
// же rebase-apply держит остановленный git am, его отличает файл applying.
func rebasing(root string) bool {
	return gitPathExists(root, "rebase-merge") || gitPathExists(root, "rebase-apply") && !applying(root)
}

// applying говорит, что git am остановился и ждёт продолжения.
func applying(root string) bool {
	return gitPathExists(root, "rebase-apply/applying")
}

// gitPathExists говорит, что в каталоге git есть файл или каталог name.
func gitPathExists(root, name string) bool {
	path, err := run(call{dir: root}, "rev-parse", "--path-format=absolute", "--git-path", name)
	if err != nil {
		return false
	}
	_, err = os.Stat(strings.TrimSpace(path))
	return err == nil
}

// unfinished говорит, что операция git остановилась посредине: слияние,
// cherry-pick, revert, rebase или am.
func unfinished(root string) bool {
	return inProgress(root) || rebasing(root) || applying(root)
}

// stopping запускает операцию, которая может остановиться на конфликте, из
// корня репозитория. Остановку она сводит к ошибке из словаря: строка git
// называет один файл, а знать нужно, что операция не закончена. Отказ до
// начала (грязное дерево, незавершённая прежняя операция) сводится к одной
// строке (headline).
func (r Repo) stopping(operation string, args ...string) error {
	if unfinished(r.root) {
		_, err := r.inRoot(args...)
		return headline(err)
	}

	_, err := r.inRoot(args...)
	if err != nil && unfinished(r.root) {
		return i18n.New("git.stoppedOnConflict", i18n.Args{"operation": operation})
	}
	return headline(err)
}

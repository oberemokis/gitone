package git

import (
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

	args := []string{"commit", "-m", subject}
	if body := strings.TrimSpace(description); body != "" {
		args = append(args, "-m", body)
	}

	_, err = run(call{dir: root}, args...)
	return err
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

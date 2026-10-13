package git

import "gitone/internal/i18n"

// Commit записывает индекс с темой subject. Идёт обычным git commit: хуки
// выполняются, автор берётся из git config, редактор не открывается. Отказ хука
// приходит ошибкой git. Пустой индекс проверяется заранее: на пустом git commit
// пишет отчёт `git status`, и причина стоит в нём не первой строкой.
func Commit(dir string, subject string) error {
	return failed("commit", commit(dir, subject))
}

func commit(dir string, subject string) error {
	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	// код 0 значит, что индекс не отличается от HEAD. Любая другая ошибка
	// проверки идёт дальше: git commit назовёт причину сам.
	if _, err := run(call{dir: root}, "diff", "--cached", "--quiet"); err == nil {
		return i18n.New("git.nothingToCommit", nil)
	}

	_, err = run(call{dir: root}, "commit", "-m", subject)
	return err
}

package git

// Commit записывает индекс с темой subject. Идёт обычным git commit: хуки
// выполняются, автор берётся из git config, редактор не открывается. Отказ хука
// или пустой индекс приходят ошибкой git.
func Commit(dir string, subject string) error {
	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	_, err = run(call{dir: root}, "commit", "-m", subject)
	return err
}

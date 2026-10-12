package git

// Commit записывает индекс с темой subject. Идёт обычным git commit: хуки
// выполняются, автор берётся из git config, редактор не открывается. Отказ хука
// или пустой индекс приходят ошибкой git.
func Commit(dir string, subject string) error {
	_, err := inRoot(dir, "commit", "-m", subject)
	return err
}

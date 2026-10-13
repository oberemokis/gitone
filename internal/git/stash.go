package git

import (
	"strings"

	"gitone/internal/i18n"
)

// Stash это запись stash: ссылка вида stash@{N}, хэш коммита stash и подпись
// git. Ссылка позиционная и сдвигается от любого git stash push, поэтому
// запись держит хэш: по нему находится именно тот stash, что видел пользователь.
type Stash struct {
	Ref     string
	Hash    string
	Subject string
}

// Stashes возвращает stash от новых к старым.
func Stashes(dir string) ([]Stash, error) {
	out, err := inRoot(dir, "stash", "list", "--format=%gd%x1f%H%x1f%gs")
	if err != nil {
		return nil, failed("stash list", err)
	}

	var stashes []Stash
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.SplitN(line, "\x1f", 3)
		if len(fields) < 3 || fields[0] == "" {
			continue
		}

		stashes = append(stashes, Stash{Ref: fields[0], Hash: fields[1], Subject: fields[2]})
	}

	return stashes, nil
}

// StashPush сохраняет отслеженные правки в stash. Неотслеженные файлы не
// трогаются: их в stash не берём, см. решения в задаче. Без правок git
// сообщает об успехе, поэтому проверяем сами: иначе пользователь думал бы, что
// правки сохранены. Подмодули в проверке не считаются: git stash push их
// игнорирует и на одном сдвинутом указателе отвечает «No local changes to save»
// с кодом 0.
func StashPush(dir string) error {
	return failed("stash push", stashPush(dir))
}

func stashPush(dir string) error {
	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	if _, err := run(call{dir: root}, "diff", "--quiet", "--ignore-submodules", "HEAD", "--"); err == nil {
		return i18n.New("git.nothingToStash", nil)
	}

	_, err = run(call{dir: root}, "stash", "push")
	return err
}

// StashApply применяет stash и оставляет его в списке.
func StashApply(dir string, ref string) error {
	_, err := inRoot(dir, "stash", "apply", ref)
	return failed("stash apply", err)
}

// StashDrop удаляет stash без применения. Вернуть его можно только через
// reflog, поэтому удаление спрашивается заранее.
func StashDrop(dir string, ref string) error {
	_, err := inRoot(dir, "stash", "drop", ref)
	return failed("stash drop", err)
}

package git

import (
	"errors"
	"strings"
)

// Stash это запись stash: ссылка вида stash@{N} и её подпись git.
type Stash struct {
	Ref     string
	Subject string
}

// Stashes возвращает stash от новых к старым.
func Stashes(dir string) ([]Stash, error) {
	out, err := inRoot(dir, "stash", "list", "--format=%gd%x1f%gs")
	if err != nil {
		return nil, err
	}

	var stashes []Stash
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		ref, subject, found := strings.Cut(line, "\x1f")
		if !found || ref == "" {
			continue
		}

		stashes = append(stashes, Stash{Ref: ref, Subject: subject})
	}

	return stashes, nil
}

// StashPush сохраняет отслеженные правки в stash. Неотслеженные файлы не
// трогаются: их в stash не берём, см. решения в задаче. Без правок git
// сообщает об успехе, поэтому проверяем сами: иначе пользователь думал бы, что
// правки сохранены.
func StashPush(dir string) error {
	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	if _, err := run(call{dir: root}, "diff", "--quiet", "HEAD", "--"); err == nil {
		return errors.New("нет правок, которые можно сохранить в stash")
	}

	_, err = run(call{dir: root}, "stash", "push")
	return err
}

// StashApply применяет stash и оставляет его в списке.
func StashApply(dir string, ref string) error {
	_, err := inRoot(dir, "stash", "apply", ref)
	return err
}

// StashDrop удаляет stash без применения. Вернуть его можно только через
// reflog, поэтому удаление спрашивается заранее.
func StashDrop(dir string, ref string) error {
	_, err := inRoot(dir, "stash", "drop", ref)
	return err
}

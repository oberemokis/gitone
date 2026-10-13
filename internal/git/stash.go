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

// StashApply применяет stash и оставляет его в списке. Если stash@{N} уже
// другой коммит, чем у строки списка, отказывает и ничего не применяет.
func StashApply(dir string, stash Stash) error {
	return failed("stash apply", stashAt(dir, stash, "apply"))
}

// StashDrop удаляет stash без применения. Вернуть его можно только через
// reflog, поэтому удаление спрашивается заранее. Если stash@{N} уже другой
// коммит, чем у строки списка, отказывает и ничего не удаляет.
func StashDrop(dir string, stash Stash) error {
	return failed("stash drop", stashAt(dir, stash, "drop"))
}

// stashAt выполняет git stash <действие> stash@{N}, только пока ссылка указывает
// на коммит из записи. refs/stash общий для всех worktree: git stash push из
// соседнего терминала сдвигает номера, и позиция из списка попала бы в чужой
// stash. Между проверкой и действием остаётся окно в миллисекунды, закрыть его
// git не даёт: drop принимает только ссылку.
func stashAt(dir string, stash Stash, action string) error {
	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	hash, err := run(call{dir: root}, "rev-parse", "--verify", "--quiet", stash.Ref)
	if err != nil || strings.TrimSpace(hash) != stash.Hash {
		return i18n.New("git.stashChanged", nil)
	}

	_, err = run(call{dir: root}, "stash", action, stash.Ref)
	return err
}

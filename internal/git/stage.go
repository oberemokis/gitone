package git

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gitone/internal/i18n"
)

// Stage добавляет пути в индекс. Удалённый файл тоже: add -A берёт и удаление.
func Stage(dir string, paths []string) error {
	return failed("add", stage(dir, paths))
}

// noPaths это отказ пустому списку путей до вызова git: `git add -A --` без
// путей ставит в индекс всё дерево.
func noPaths() error {
	return i18n.New("git.noPaths", nil)
}

func stage(dir string, paths []string) error {
	if len(paths) == 0 {
		return noPaths()
	}

	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	_, err = run(literal(root), append([]string{"add", "-A", "--"}, paths...)...)
	return err
}

// Unstage убирает пути из индекса. Правка в рабочем дереве остаётся на месте.
func Unstage(dir string, paths []string) error {
	return failed("restore", unstage(dir, paths))
}

func unstage(dir string, paths []string) error {
	if len(paths) == 0 {
		return noPaths()
	}

	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	command := []string{"restore", "--staged", "--"}
	if !hasCommits(root) {
		// Без HEAD восстанавливать не из чего: запись просто выходит из индекса.
		command = []string{"rm", "--cached", "--quiet", "--"}
	}

	_, err = run(literal(root), append(command, paths...)...)
	return err
}

// submoduleMode это режим записи подмодуля (gitlink) в дереве.
const submoduleMode = "160000"

// Discard отменяет правки путей. Файл, который есть в HEAD, возвращается к нему
// вместе с индексом. Файл, которого в HEAD нет, убирается из индекса и с диска.
// Подмодуль не отменяется: restore сдвинул бы его checkout и потерял коммиты.
// Ошибка git на файле из HEAD не трогает диск: решается по ls-tree, а не по
// сбою restore, чтобы сбой не стёр правку.
func Discard(dir string, paths []string) error {
	return failed("restore", discard(dir, paths))
}

func discard(dir string, paths []string) error {
	if len(paths) == 0 {
		return noPaths()
	}

	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	base, err := baseRevision(root)
	if err != nil {
		return err
	}

	// Отказ на подмодуле идёт до первой правки: иначе часть путей была бы уже
	// отменена.
	tracked := make([]bool, len(paths))
	for i, path := range paths {
		mode, err := treeMode(root, base, path)
		if err != nil {
			return err
		}
		if mode == submoduleMode {
			return i18n.New("git.submoduleDiscard", i18n.Args{"path": path})
		}
		tracked[i] = mode != ""
	}

	for i, path := range paths {
		if tracked[i] {
			if _, err := run(literal(root), "restore", "--source=HEAD", "--staged", "--worktree", "--", path); err != nil {
				return err
			}
			continue
		}

		if _, err := run(literal(root), "rm", "-f", "--cached", "--ignore-unmatch", "--quiet", "--", path); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(root, path)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}

	return nil
}

// treeMode возвращает режим записи пути в дереве base, пустую строку, если пути
// там нет. cat-file для этого не годится: у gitlink подмодуля в этом
// репозитории нет объекта.
func treeMode(root, base, path string) (string, error) {
	out, err := run(literal(root), "ls-tree", base, "--", path)
	mode, _, _ := strings.Cut(out, " ")
	return mode, err
}

// StagedPaths возвращает пути, у которых индекс отличается от HEAD. Неслитые
// пути (конфликт слияния) не в счёте: их ещё нужно пометить разрешёнными, то
// есть поставить, а не убрать из индекса.
func StagedPaths(dir string) ([]string, error) {
	out, err := inRoot(dir, "diff", "--cached", "--name-only", "-z", "--diff-filter=u")
	if err != nil {
		return nil, failed("diff", err)
	}

	var paths []string
	for _, path := range strings.Split(out, "\x00") {
		if path != "" {
			paths = append(paths, path)
		}
	}

	return paths, nil
}

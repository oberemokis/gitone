package git

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Stage добавляет пути в индекс. Удалённый файл тоже: add -A берёт и удаление.
func Stage(dir string, paths []string) error {
	return failed("add", stage(dir, paths))
}

func stage(dir string, paths []string) error {
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
	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	_, err = run(literal(root), append([]string{"restore", "--staged", "--"}, paths...)...)
	return err
}

// Discard отменяет правки путей. Файл, который есть в HEAD, возвращается к нему
// вместе с индексом. Файл, которого в HEAD нет, убирается из индекса и с диска.
// Ошибка git на файле из HEAD не трогает диск: решается по cat-file, а не по
// сбою restore, чтобы сбой не стёр правку.
func Discard(dir string, paths []string) error {
	return failed("restore", discard(dir, paths))
}

func discard(dir string, paths []string) error {
	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	for _, path := range paths {
		if _, err := run(literal(root), "cat-file", "-e", "HEAD:"+path); err == nil {
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

// StagedPaths возвращает пути, у которых индекс отличается от HEAD.
func StagedPaths(dir string) ([]string, error) {
	out, err := inRoot(dir, "diff", "--cached", "--name-only", "-z")
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

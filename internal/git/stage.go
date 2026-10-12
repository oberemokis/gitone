package git

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Пути в diff идут от корня репозитория, а gitone может быть запущен из
// подкаталога. Поэтому все команды правки идут из корня, иначе относительный
// путь указывал бы не на тот файл.
func topLevel(dir string) (string, error) {
	root, err := run(call{dir: dir}, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(root, "\n"), nil
}

// Stage добавляет пути в индекс. Удалённый файл тоже: add -A берёт и удаление.
func Stage(dir string, paths []string) error {
	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	_, err = run(call{dir: root}, append([]string{"add", "-A", "--"}, paths...)...)
	return err
}

// Unstage убирает пути из индекса. Правка в рабочем дереве остаётся на месте.
func Unstage(dir string, paths []string) error {
	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	_, err = run(call{dir: root}, append([]string{"restore", "--staged", "--"}, paths...)...)
	return err
}

// Discard отменяет правки путей. Файл, который есть в HEAD, возвращается к нему
// вместе с индексом. Файл, которого в HEAD нет, убирается из индекса и с диска.
// Ошибка git на файле из HEAD не трогает диск: решается по cat-file, а не по
// сбою restore, чтобы сбой не стёр правку.
func Discard(dir string, paths []string) error {
	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	for _, path := range paths {
		if _, err := run(call{dir: root}, "cat-file", "-e", "HEAD:"+path); err == nil {
			if _, err := run(call{dir: root}, "restore", "--source=HEAD", "--staged", "--worktree", "--", path); err != nil {
				return err
			}
			continue
		}

		if _, err := run(call{dir: root}, "rm", "-f", "--cached", "--ignore-unmatch", "--quiet", "--", path); err != nil {
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
	root, err := topLevel(dir)
	if err != nil {
		return nil, err
	}

	out, err := run(call{dir: root}, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, path := range strings.Split(out, "\x00") {
		if path != "" {
			paths = append(paths, path)
		}
	}

	return paths, nil
}

package git

import (
	"os"
	"path/filepath"
	"strings"
)

// Patch сохраняет патч коммита hash в корень репозитория файлом
// `0001-<тема>.patch`, как `git format-patch -1` (#224), и возвращает имя файла
// от корня и сам патч: его же кладут в буфер обмена.
func Patch(dir, hash string) (name, patch string, err error) {
	name, patch, err = savePatch(dir, hash)
	return name, patch, failed("format-patch", err)
}

func savePatch(dir, hash string) (string, string, error) {
	root, err := topLevel(dir)
	if err != nil {
		return "", "", err
	}

	written, err := run(call{dir: root}, "format-patch", "-1", "--output-directory", root, hash)
	if err != nil {
		return "", "", err
	}

	path := strings.TrimSpace(written)
	patch, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}

	name, err := filepath.Rel(root, path)
	return name, string(patch), err
}

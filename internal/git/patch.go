package git

import (
	"os"
	"path/filepath"
	"strings"

	"gitone/internal/i18n"
)

// Patch сохраняет патч коммита hash в корень репозитория файлом
// `0001-<тема>.patch`, как `git format-patch -1` (#224), и возвращает имя файла
// от корня и сам патч: его же кладут в буфер обмена. Merge-коммит
// отвергается: format-patch молча взял бы вместо него коммит под ним.
func (r Repo) Patch(hash string) (name, patch string, err error) {
	name, patch, err = r.savePatch(hash)
	return name, patch, failed("format-patch", err)
}

func (r Repo) savePatch(hash string) (string, string, error) {
	if isMerge(r.root, hash) {
		return "", "", i18n.New("git.patchMerge", nil)
	}

	written, err := r.inRoot("format-patch", "-1", "--output-directory", r.root, hash)
	if err != nil {
		return "", "", err
	}

	path := strings.TrimSpace(written)
	patch, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}

	name, err := filepath.Rel(r.root, path)
	return name, string(patch), err
}

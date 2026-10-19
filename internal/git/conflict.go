package git

import (
	"os"
	"path/filepath"
	"strings"

	"gitone/internal/i18n"
)

// ConflictedPaths возвращает пути с нерешённым конфликтом слияния (#242).
func (r Repo) ConflictedPaths() ([]string, error) {
	out, err := r.inRoot("diff", "--name-only", "--diff-filter=U", "-z")
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

// ResolveFile решает конфликт файла целиком: берёт нашу или их версию из
// индекса и ставит её, то есть помечает конфликт решённым. Бинарный файл
// решается только так.
func (r Repo) ResolveFile(path string, theirs bool) error {
	side := "--ours"
	if theirs {
		side = "--theirs"
	}
	if _, err := run(literal(r.root), "checkout", side, "--", path); err != nil {
		return failed("checkout", headline(err))
	}
	_, err := run(literal(r.root), "add", "--", path)
	return failed("add", headline(err))
}

// file это имя файла рабочего дерева по пути от корня. Нулевой Repo не
// репозиторий: имени в текущем каталоге процесса он не даёт.
func (r Repo) file(path string) (string, error) {
	if r.root == "" {
		return "", i18n.New("git.noRepository", nil)
	}
	return filepath.Join(r.root, filepath.FromSlash(path)), nil
}

// WorktreeFile читает файл рабочего дерева по пути от корня: решение конфликта
// по хунку правит его текст.
func (r Repo) WorktreeFile(path string) (string, error) {
	name, err := r.file(path)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(name)
	return string(content), err
}

// HeadFile читает файл из HEAD по пути от корня, пустой текст, если его там
// нет: маркеры конфликта, которые уже лежали в HEAD, решать не нужно.
func (r Repo) HeadFile(path string) string {
	content, _ := r.inRoot("cat-file", "blob", "HEAD:"+path)
	return content
}

// WriteWorktreeFile записывает текст в файл рабочего дерева, права файла
// остаются прежними.
func (r Repo) WriteWorktreeFile(path, text string) error {
	name, err := r.file(path)
	if err != nil {
		return err
	}

	info, err := os.Stat(name)
	if err != nil {
		return err
	}
	return os.WriteFile(name, []byte(text), info.Mode().Perm())
}

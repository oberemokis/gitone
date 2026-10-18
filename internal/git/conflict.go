package git

import (
	"os"
	"path/filepath"
	"strings"
)

// ConflictedPaths возвращает пути с нерешённым конфликтом слияния (#242).
func ConflictedPaths(dir string) ([]string, error) {
	out, err := inRoot(dir, "diff", "--name-only", "--diff-filter=U", "-z")
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
func ResolveFile(dir, path string, theirs bool) error {
	root, err := topLevel(dir)
	if err != nil {
		return failed("checkout", err)
	}

	side := "--ours"
	if theirs {
		side = "--theirs"
	}
	if _, err := run(literal(root), "checkout", side, "--", path); err != nil {
		return failed("checkout", headline(err))
	}
	_, err = run(literal(root), "add", "--", path)
	return failed("add", headline(err))
}

// WorktreeFile читает файл рабочего дерева по пути от корня: решение конфликта
// по хунку правит его текст.
func WorktreeFile(dir, path string) (string, error) {
	root, err := topLevel(dir)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	return string(content), err
}

// WriteWorktreeFile записывает текст в файл рабочего дерева, права файла
// остаются прежними.
func WriteWorktreeFile(dir, path, text string) error {
	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	name := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Stat(name)
	if err != nil {
		return err
	}
	return os.WriteFile(name, []byte(text), info.Mode().Perm())
}

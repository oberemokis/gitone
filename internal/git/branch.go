package git

import "strings"

// Branch это локальная ветка. Current значит, что на ней стоит HEAD.
type Branch struct {
	Name    string
	Current bool
}

// Branches возвращает локальные ветки по имени. Текущая помечена.
func Branches(dir string) ([]Branch, error) {
	out, err := inRoot(dir, "for-each-ref", "--format=%(refname:short)%09%(HEAD)", "refs/heads")
	if err != nil {
		return nil, failed("branch", err)
	}

	var branches []Branch
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		name, head, found := strings.Cut(line, "\t")
		if !found || name == "" {
			continue
		}

		branches = append(branches, Branch{Name: name, Current: head == "*"})
	}

	return branches, nil
}

// Switch переключает рабочее дерево на ветку. Правки, которым переключение
// мешает, git отвергает сам: тогда ветка остаётся прежней, а текст отказа
// возвращается ошибкой.
func Switch(dir string, name string) error {
	_, err := inRoot(dir, "switch", "--", name)
	return failed("switch", err)
}

package git

import "strings"

// Branch это локальная ветка. Current значит, что на ней стоит HEAD. Author и
// When это автор и время последнего коммита ветки: кто создал ветку, git не
// хранит.
type Branch struct {
	Name    string
	Current bool
	Author  string
	When    string
}

// Branches возвращает локальные ветки по имени. Текущая помечена. Время
// относительное, как в логе: «4 minutes ago».
func Branches(dir string) ([]Branch, error) {
	out, err := inRoot(dir, "for-each-ref", "--format=%(refname:short)%1f%(HEAD)%1f%(authorname)%1f%(committerdate:relative)", "refs/heads")
	if err != nil {
		return nil, failed("branch", err)
	}

	var branches []Branch
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Split(line, "\x1f")
		if len(fields) != 4 || fields[0] == "" {
			continue
		}

		branches = append(branches, Branch{Name: fields[0], Current: fields[1] == "*", Author: fields[2], When: fields[3]})
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

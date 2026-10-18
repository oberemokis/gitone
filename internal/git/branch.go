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
// относительное, как в логе: «4 minutes ago». Имя берётся срезом двух
// компонентов refs/heads, а не %(refname:short): рядом с тегом или удалённой
// веткой того же имени git отдаёт неоднозначное heads/v1. Автор стоит в формате
// последним и режется по первым трём разделителям: в имени автора разделитель
// допустим, и такая ветка не должна пропадать.
func Branches(dir string) ([]Branch, error) {
	out, err := inRoot(dir, "for-each-ref", "--format=%(refname:lstrip=2)%1f%(HEAD)%1f%(committerdate:relative)%1f%(authorname)", "refs/heads")
	if err != nil {
		return nil, failed("branch", err)
	}

	var branches []Branch
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.SplitN(line, "\x1f", 4)
		// пустой вывод даёт одну пустую строку, у настоящей ветки полей четыре
		if len(fields) != 4 {
			continue
		}

		branches = append(branches, Branch{Name: fields[0], Current: fields[1] == "*", When: fields[2], Author: fields[3]})
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

// CreateBranch создаёт ветку name на base и переключается на неё, как
// `git switch -c` (#227). Имя проверяет git: занятое или недопустимое он
// отвергает своим текстом, и ветка остаётся прежней.
func CreateBranch(dir, name, base string) error {
	_, err := inRoot(dir, "switch", "-c", name, base)
	return failed("switch", err)
}

// DeleteBranch удаляет ветку через `git branch -d`: неслитую git отвергает, а
// принудительного удаления нет, как и удаления на remote (#227).
func DeleteBranch(dir, name string) error {
	_, err := inRoot(dir, "branch", "-d", "--", name)
	return failed("branch", err)
}

// RenameBranch переименовывает ветку только локально, `git branch -m` (#228).
// Связь с upstream git переносит сам, занятое имя отвергает: принудительного
// переименования нет.
func RenameBranch(dir, from, to string) error {
	_, err := inRoot(dir, "branch", "-m", "--", from, to)
	return failed("branch", err)
}

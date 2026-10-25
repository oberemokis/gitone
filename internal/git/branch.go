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
func (r Repo) Branches() ([]Branch, error) {
	out, err := r.inRoot("for-each-ref", "--format=%(refname:lstrip=2)%1f%(HEAD)%1f%(committerdate:relative)%1f%(authorname)", "refs/heads")
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
func (r Repo) Switch(name string) error {
	_, err := r.inRoot("switch", "--", name)
	return failed("switch", err)
}

// CreateBranch создаёт ветку name на base и переключается на неё, как
// `git switch -c` (#227). Имя проверяет git: занятое или недопустимое он
// отвергает своим текстом, и ветка остаётся прежней.
func (r Repo) CreateBranch(name, base string) error {
	_, err := r.inRoot("switch", "-c", name, base)
	return failed("switch", err)
}

// CheckoutCommit переводит HEAD на коммит без ветки, `git switch --detach`
// (#223). Правки, которым переход мешает, git отвергает сам, и HEAD остаётся.
func (r Repo) CheckoutCommit(hash string) error {
	_, err := r.inRoot("switch", "--detach", hash)
	return failed("switch", headline(err))
}

// DeleteBranch удаляет ветку через `git branch -d`: неслитую git отвергает, а
// принудительного удаления нет, как и удаления на remote (#227).
func (r Repo) DeleteBranch(name string) error {
	_, err := r.inRoot("branch", "-d", "--", name)
	return failed("branch", err)
}

// RenameBranch переименовывает ветку только локально, `git branch -m` (#228).
// Связь с upstream git переносит сам, занятое имя отвергает: принудительного
// переименования нет.
func (r Repo) RenameBranch(from, to string) error {
	_, err := r.inRoot("branch", "-m", "--", from, to)
	return failed("branch", err)
}

// RemoteBranches возвращает удалённые ветки из локальных ссылок, `origin/main`,
// по имени (#230). Символическая `origin/HEAD` не ветка, её нет.
func (r Repo) RemoteBranches() ([]string, error) {
	out, err := r.inRoot("for-each-ref", "--format=%(refname:lstrip=2)%1f%(symref)", "refs/remotes")
	if err != nil {
		return nil, failed("branch", err)
	}

	var names []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		name, symref, _ := strings.Cut(line, "\x1f")
		if name != "" && symref == "" {
			names = append(names, name)
		}
	}
	return names, nil
}

// SetUpstream привязывает ветку к удалённой upstream (#230), заменяя прежнюю.
// Удалённой ветки нет в локальных ссылках, и git отказывает своим текстом.
// upstream называется полной ссылкой: локальную ветку или тег с именем
// origin/x git предпочёл бы удалённой (#387).
func (r Repo) SetUpstream(branch, upstream string) error {
	_, err := r.inRoot("branch", "--set-upstream-to=refs/remotes/"+upstream, branch)
	return failed("branch", err)
}

// Merge сливает ветку name в текущую (#229): перемоткой, если можно, иначе
// merge-коммитом с сообщением по умолчанию, редактор не открывается. Конфликт
// останавливает слияние, ошибка тогда git.stoppedOnConflict. Тег с тем же
// именем git предпочёл бы ветке, тогда ветка называется полной ссылкой, а
// сообщение задаётся явно: по полной ссылке git написал бы «refs/heads/» в нём.
func (r Repo) Merge(name string) error {
	args := []string{"merge", "--no-edit", name}
	if _, err := r.inRoot("rev-parse", "--verify", "-q", "refs/tags/"+name); err == nil {
		args = []string{"merge", "--no-edit", "-m", r.mergeMessage(name), "refs/heads/" + name}
	}
	return failed("merge", r.stopping("merge", args...))
}

// mergeMessage повторяет сообщение git по умолчанию: вне main и master он
// называет ветку, в которую сливает, а без ветки пишет HEAD (#387).
func (r Repo) mergeMessage(name string) string {
	message := "Merge branch '" + name + "'"
	current, err := r.inRoot("symbolic-ref", "--short", "-q", "HEAD")
	current = strings.TrimSpace(current)
	if err != nil {
		current = "HEAD"
	}
	if current != "main" && current != "master" {
		message += " into " + current
	}
	return message
}

// Rebase перестраивает текущую ветку поверх name (#229). Грязное дерево git
// отвергает, конфликт останавливает rebase на первом спорном коммите. Ветка
// называется полной ссылкой: одноимённый тег git предпочёл бы ей.
func (r Repo) Rebase(name string) error {
	return failed("rebase", r.stopping("rebase", "rebase", "refs/heads/"+name))
}

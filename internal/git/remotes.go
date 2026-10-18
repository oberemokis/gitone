package git

import "strings"

// Remote это remote репозитория (#233): имя и адреса fetch и push. Upstream
// значит, что на нём стоит upstream текущей ветки, и удаление remote его
// снимет.
type Remote struct {
	Name     string
	Fetch    string
	Push     string
	Upstream bool
}

// Remotes возвращает remote по имени из `git remote -v`: строка на адрес
// fetch и строка на адрес push, имя и адрес разделены табуляцией.
func Remotes(dir string) ([]Remote, error) {
	out, err := inRoot(dir, "remote", "-v")
	if err != nil {
		return nil, failed("remote", err)
	}

	var remotes []Remote
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		name, address, found := strings.Cut(line, "\t")
		if !found {
			continue
		}
		if len(remotes) == 0 || remotes[len(remotes)-1].Name != name {
			remotes = append(remotes, Remote{Name: name})
		}

		remote := &remotes[len(remotes)-1]
		if url, isFetch := strings.CutSuffix(address, " (fetch)"); isFetch {
			remote.Fetch = url
		} else if url, isPush := strings.CutSuffix(address, " (push)"); isPush {
			remote.Push = url
		}
	}

	upstream := upstreamRemote(dir)
	for index := range remotes {
		remotes[index].Upstream = remotes[index].Name == upstream
	}
	return remotes, nil
}

// upstreamRemote это имя remote, на котором стоит upstream текущей ветки, или
// пустая строка: при detached HEAD и без upstream.
func upstreamRemote(dir string) string {
	branch, err := currentBranch(dir)
	if err != nil {
		return ""
	}

	out, err := run(call{dir: dir}, "config", "--get", "branch."+branch+".remote")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// AddRemote добавляет remote name с адресом url для fetch и push. Занятое имя
// git отвергает.
func AddRemote(dir, name, url string) error {
	_, err := inRoot(dir, "remote", "add", "--", name, url)
	return failed("remote", headline(err))
}

// RemoveRemote удаляет remote вместе с его удалёнными ветками и upstream
// локальных веток, которые на него указывали.
func RemoveRemote(dir, name string) error {
	_, err := inRoot(dir, "remote", "remove", "--", name)
	return failed("remote", headline(err))
}

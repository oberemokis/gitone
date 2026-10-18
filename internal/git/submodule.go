package git

import (
	"context"
	"strings"
)

// Submodule это подмодуль (#244): путь, адрес из .gitmodules, коммит и
// состояние. Initialized false значит, что подмодуль не клонирован, Commit
// тогда коммит, записанный в суперпроекте. Moved значит, что в подмодуле
// стоит другой коммит, чем записан.
type Submodule struct {
	Path        string
	URL         string
	Commit      string
	Initialized bool
	Moved       bool
}

// Submodules возвращает подмодули из `git submodule status`: первый знак
// строки говорит о состоянии («-» не клонирован, «+» другой коммит), дальше
// хэш и путь. Адрес берётся из .gitmodules по имени подмодуля.
func Submodules(dir string) ([]Submodule, error) {
	out, err := inRoot(dir, "submodule", "status")
	if err != nil {
		return nil, failed("submodule", err)
	}
	urls := submoduleURLs(dir)

	var submodules []Submodule
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if len(line) < 2 {
			continue
		}
		fields := strings.Fields(line[1:])
		if len(fields) < 2 {
			continue
		}

		submodules = append(submodules, Submodule{
			Path:        fields[1],
			URL:         urls[fields[1]],
			Commit:      fields[0],
			Initialized: line[0] != '-',
			Moved:       line[0] == '+',
		})
	}
	return submodules, nil
}

// submoduleURLs это адреса подмодулей по пути из .gitmodules. Ключи там по
// имени подмодуля, путь и адрес у имени свои.
func submoduleURLs(dir string) map[string]string {
	out, err := inRoot(dir, "config", "--file", ".gitmodules", "--get-regexp", `^submodule\..*\.(path|url)$`)
	if err != nil {
		return nil
	}

	paths, urls := map[string]string{}, map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		key, value, _ := strings.Cut(line, " ")
		if name, found := strings.CutSuffix(key, ".path"); found {
			paths[name] = value
		} else if name, found := strings.CutSuffix(key, ".url"); found {
			urls[name] = value
		}
	}

	byPath := make(map[string]string, len(paths))
	for name, path := range paths {
		byPath[path] = urls[name]
	}
	return byPath
}

// UpdateSubmodule клонирует подмодуль path, если его нет, и ставит в нём
// коммит, записанный в суперпроекте: `git submodule update --init`. Сеть
// нужна для клона, отмена ctx убивает git.
func UpdateSubmodule(ctx context.Context, dir, path string) error {
	root, err := topLevel(dir)
	if err != nil {
		return failed("submodule", err)
	}
	return failed("submodule", headline(network(ctx, root, "submodule", "update", "--init", "--", path)))
}

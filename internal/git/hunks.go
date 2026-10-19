package git

import "strings"

// UnstagedDiff возвращает дифф файла path между индексом и рабочим деревом
// (#212): в нём то, что ещё можно поставить в индекс по хункам и строкам.
// flags это флаги вида, как у Diff.
func (r Repo) UnstagedDiff(path string, flags ...string) (string, error) {
	text, err := run(literal(r.root), diffCommand(flags, "--", path)...)
	return text, failed("diff", err)
}

// UnstagedPaths возвращает пути, у которых рабочее дерево отличается от
// индекса. Вместе со StagedPaths они дают частично поставленные файлы.
func (r Repo) UnstagedPaths() ([]string, error) {
	out, err := r.inRoot("diff", "--name-only", "-z")
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

// ApplyPatch применяет патч из выбранных строк (#212, #213): cached к
// индексу, без него к рабочему дереву, reverse в обратную сторону. Патч,
// который git не принял, ничего не меняет: git проверяет все хунки до записи.
func (r Repo) ApplyPatch(patch string, cached, reverse bool) error {
	args := []string{"apply", "--whitespace=nowarn"}
	if cached {
		args = append(args, "--cached")
	}
	if reverse {
		args = append(args, "--reverse")
	}

	_, err := run(call{dir: r.root, stdin: patch}, append(args, "-")...)
	return failed("apply", headline(err))
}

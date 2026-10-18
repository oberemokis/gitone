package git

import "strings"

// UnstagedDiff возвращает дифф файла path между индексом и рабочим деревом
// (#212): в нём то, что ещё можно поставить в индекс по хункам и строкам.
// flags это флаги вида, как у Diff.
func UnstagedDiff(dir, path string, flags ...string) (string, error) {
	root, err := topLevel(dir)
	if err != nil {
		return "", failed("diff", err)
	}

	text, err := run(literal(root), diffCommand(flags, "--", path)...)
	return text, failed("diff", err)
}

// UnstagedPaths возвращает пути, у которых рабочее дерево отличается от
// индекса. Вместе со StagedPaths они дают частично поставленные файлы.
func UnstagedPaths(dir string) ([]string, error) {
	out, err := inRoot(dir, "diff", "--name-only", "-z")
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
func ApplyPatch(dir, patch string, cached, reverse bool) error {
	root, err := topLevel(dir)
	if err != nil {
		return failed("apply", err)
	}

	args := []string{"apply", "--whitespace=nowarn"}
	if cached {
		args = append(args, "--cached")
	}
	if reverse {
		args = append(args, "--reverse")
	}

	_, err = run(call{dir: root, stdin: patch}, append(args, "-")...)
	return failed("apply", headline(err))
}

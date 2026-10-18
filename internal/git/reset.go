package git

import "strings"

// ResetMode это режим reset (#220): что станет с правками между HEAD и целью.
type ResetMode string

const (
	// ResetSoft оставляет правки в индексе
	ResetSoft ResetMode = "soft"
	// ResetMixed оставляет правки в рабочем дереве, индекс пуст
	ResetMixed ResetMode = "mixed"
	// ResetHard стирает правки: дерево и индекс совпадают с целью
	ResetHard ResetMode = "hard"
)

// Reset переводит текущую ветку на target в режиме mode. published говорит,
// что среди снятых коммитов есть коммиты upstream ветки: ветка теперь
// разошлась с remote, и обычный push её не отправит. Без upstream published
// всегда false.
func Reset(dir, target string, mode ResetMode) (published bool, err error) {
	published, err = reset(dir, target, mode)
	return published, failed("reset", err)
}

func reset(dir, target string, mode ResetMode) (bool, error) {
	root, err := topLevel(dir)
	if err != nil {
		return false, err
	}

	published := dropsPublished(root, target)
	if _, err := run(call{dir: root}, "reset", "--"+string(mode), target, "--"); err != nil {
		return false, headline(err)
	}
	return published, nil
}

// dropsPublished говорит, что переход HEAD на target снимет коммит, который
// уже есть в upstream. Общие предки HEAD и upstream это ровно опубликованная
// часть ветки: если хоть один из них не предок target, reset его снимет.
func dropsPublished(root, target string) bool {
	bases, err := run(call{dir: root}, "merge-base", "--all", "HEAD", "@{upstream}")
	if err != nil {
		return false
	}

	for _, base := range strings.Fields(bases) {
		if _, err := run(call{dir: root}, "merge-base", "--is-ancestor", base, target); err != nil {
			return true
		}
	}
	return false
}

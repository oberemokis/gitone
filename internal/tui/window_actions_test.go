package tui

import (
	"slices"
	"testing"

	"gitone/internal/keymap"
)

// Подсказка окна-списка вместе с навигацией и группа окна в keymap называют
// одни и те же действия: что окно разбирает, то keymap проверяет на конфликт
// клавиш, и наоборот (#353).
func TestWindowActionsMatchKeymap(t *testing.T) {
	navigation := []keymap.Action{keymap.ListUp, keymap.ListDown, keymap.ListClose}
	hints := map[string][]keymap.Action{
		"log":        logHint,
		"branches":   branchHint,
		"worktrees":  worktreeHint,
		"stash":      stashHint,
		"tags":       tagHint,
		"remotes":    remoteHint,
		"reflog":     reflogHint,
		"submodules": submoduleHint,
		"recent":     recentHint,
	}

	groups := keymap.Groups()
	for _, group := range groups {
		slices.Sort(group)
	}
	for name, hint := range hints {
		want := slices.Sorted(slices.Values(slices.Concat(hint, navigation)))
		if !slices.ContainsFunc(groups, func(group []keymap.Action) bool { return slices.Equal(group, want) }) {
			t.Errorf("окно %s: действия %q не совпадают ни с одной группой keymap", name, want)
		}
	}
}

package tui

import (
	"testing"

	"gitone/internal/keymap"
)

// Внутренний тест: таблица globalActions не экспортируется, а проверить все
// действия через экран нечем. Глобальное действие узнаётся по keymap: его
// клавиша на главном экране ведёт к нему самому, клавиша действия окна нет.
func TestGlobalActionsComplete(t *testing.T) {
	keys := keymap.Default()
	global := func(action keymap.Action) bool {
		bound := keys.KeysFor(action)
		return len(bound) > 0 && keys.ActionFor(bound[0]) == action
	}

	for _, binding := range keys.Bindings() {
		if _, found := globalActions[binding.Action]; global(binding.Action) && !found {
			t.Errorf("у глобального действия %s нет записи в globalActions", binding.Action)
		}
	}
	for action := range globalActions {
		if !global(action) {
			t.Errorf("в globalActions действие %s, которое не глобальное или без клавиш", action)
		}
	}
}

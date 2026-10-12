package tui

import (
	"slices"
	"strings"
)

type action string

const (
	actionQuit      action = "app.quit"
	actionFiles     action = "files.pick"
	actionNextFile  action = "files.next"
	actionPrevFile  action = "files.previous"
	actionHelp      action = "app.help"
	actionCloseHelp action = "help.close"
	actionMode      action = "diff.toggleMode"
	actionReload    action = "diff.reload"
	actionDown      action = "diff.scrollDown"
	actionUp        action = "diff.scrollUp"
	actionHalfDown  action = "diff.halfPageDown"
	actionHalfUp    action = "diff.halfPageUp"
	actionTop       action = "diff.top"
	actionBottom    action = "diff.bottom"
)

// Заголовок группы в справке. Порядок групп задаёт первое появление
// категории в bindings(), подряд клавиши одной категории стоять не обязаны.
type category string

const (
	categoryGeneral category = "General"
	categoryFiles   category = "Files"
	categoryDiff    category = "Diff"
)

type binding struct {
	action   action
	category category
	// как их возвращает KeyPressMsg.String()
	keys []string
	// подпись в подвале, пустая значит в подвал не выводить
	label string
	// строка в справке, пустая значит в справку не выводить
	description string
}

// Подвал, справка и Update читают один список, поэтому не расходятся.
// ponytail: список зашит, переедет в keymap вместе с keybinds.json
func bindings() []binding {
	return []binding{
		{action: actionQuit, category: categoryGeneral, keys: []string{"q", "ctrl+c"}, label: "quit", description: "Quit"},
		{action: actionFiles, category: categoryFiles, keys: []string{"ctrl+p"}, label: "files", description: "File list"},
		{action: actionHelp, category: categoryGeneral, keys: []string{"?"}, label: "help", description: "Toggle this help"},
		{action: actionNextFile, category: categoryFiles, keys: []string{"]"}, description: "Next file"},
		{action: actionPrevFile, category: categoryFiles, keys: []string{"["}, description: "Previous file"},
		{action: actionDown, category: categoryDiff, keys: []string{"j", "down"}, description: "Scroll down"},
		{action: actionUp, category: categoryDiff, keys: []string{"k", "up"}, description: "Scroll up"},
		{action: actionHalfDown, category: categoryDiff, keys: []string{"ctrl+d"}, description: "Half page down"},
		{action: actionHalfUp, category: categoryDiff, keys: []string{"ctrl+u"}, description: "Half page up"},
		{action: actionTop, category: categoryDiff, keys: []string{"g"}, description: "Top"},
		{action: actionBottom, category: categoryDiff, keys: []string{"G"}, description: "Bottom"},
		{action: actionMode, category: categoryDiff, keys: []string{"m"}, description: "Split or unified"},
		{action: actionReload, category: categoryDiff, keys: []string{"r"}, description: "Reload diff"},
		// в справке стоит в заголовке, как esc в окнах opencode
		{action: actionCloseHelp, keys: []string{"esc"}},
	}
}

func keysFor(target action) []string {
	for _, binding := range bindings() {
		if binding.action == target {
			return binding.keys
		}
	}

	return nil
}

func actionFor(key string) action {
	for _, binding := range bindings() {
		if slices.Contains(binding.keys, key) {
			return binding.action
		}
	}

	return ""
}

// ctrl+p показываем как «ctrl p», так читается в подвале critique
func displayKey(key string) string {
	return strings.ReplaceAll(key, "+", " ")
}

package tui

import (
	"slices"
	"strings"
)

type action string

const (
	actionQuit      action = "app.quit"
	actionFiles     action = "files.pick"
	actionHelp      action = "app.help"
	actionCloseHelp action = "help.close"
)

// Заголовок группы в справке. Порядок групп задаёт первое появление
// категории в bindings(), подряд клавиши одной категории стоять не обязаны.
type category string

const (
	categoryGeneral category = "General"
	categoryFiles   category = "Files"
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

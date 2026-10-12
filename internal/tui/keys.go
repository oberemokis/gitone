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

type binding struct {
	action action
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
		{action: actionQuit, keys: []string{"q", "ctrl+c"}, label: "quit", description: "Quit"},
		{action: actionFiles, keys: []string{"ctrl+p"}, label: "files", description: "File list"},
		{action: actionHelp, keys: []string{"?"}, label: "help", description: "Toggle this help"},
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

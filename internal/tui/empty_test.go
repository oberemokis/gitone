package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
	"gitone/internal/tui"
)

// В репозитории без коммитов пустой список говорит о том, что в нём нет
// записей, и только набранный запрос превращает это в «ничего не нашлось»:
// открытое поле с пустым запросом фильтром не считается (#153).
func TestEmptyListsDoNotClaimFilter(t *testing.T) {
	for _, tc := range []struct {
		name  string
		press []rune
		want  string
		not   string
	}{
		{name: "log", press: []rune{'l'}, want: "No commits", not: "No matching"},
		{name: "log search", press: []rune{'l', '/'}, want: "No commits", not: "No matching"},
		{name: "branches", press: []rune{'B'}, want: "No branches", not: "No matches"},
		{name: "branches search", press: []rune{'B', '/'}, want: "No branches", not: "No matches"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := resize(open(t, options(newRepo(t), layout.Focus())), 100, 30)

			model = openAsync(model, key(tc.press[0]))
			model = typeText(model, string(tc.press[1:]))

			if screen := screenText(model); !strings.Contains(screen, tc.want) || strings.Contains(screen, tc.not) {
				t.Fatalf("на экране нет %q или есть %q:\n%s", tc.want, tc.not, screen)
			}
		})
	}
}

// Без изменений ctrl p всё равно открывает поиск: клавиша не должна молча
// не работать.
func TestPickerOpensWithoutFiles(t *testing.T) {
	model := resize(tui.New(options("", layout.Focus()), nil), 80, 20)

	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	if !strings.Contains(screenText(model), "No files") {
		t.Fatalf("ctrl p без файлов не открыл поиск:\n%s", screenText(model))
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if strings.Contains(screenText(model), "No files") {
		t.Fatalf("esc не закрыл поиск:\n%s", screenText(model))
	}
}

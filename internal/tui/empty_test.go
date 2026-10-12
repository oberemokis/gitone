package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
	"gitone/internal/tui"
)

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

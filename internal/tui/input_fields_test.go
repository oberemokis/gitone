package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Описание из эмодзи с вариантом представления переносится по ширине
// терминала: строка поля не шире строки из латиницы (#312, случай 1).
func TestDescriptionWrapsWideGraphemes(t *testing.T) {
	typed := func(text string) []string {
		_, model := threeFileRepo(t)
		model = resize(model, 40, 30)
		model, _ = model.Update(key('c'))
		model, _ = model.Update(tab)
		model, _ = paste(model, text)
		return fieldRows(t, model)
	}

	limit := ansi.StringWidth(typed(strings.Repeat("x", 100))[0])
	for _, row := range typed(strings.Repeat("⚠️", 40)) {
		if width := ansi.StringWidth(row); width > limit {
			t.Fatalf("строка поля шириной %d, шире строки латиницы %d: %q", width, limit, row)
		}
	}
}

// Курсор и стирание не рвут кластер: ← после «⚠️» встаёт перед ним целиком,
// backspace стирает его целиком (случай 1).
func TestDescriptionCursorKeepsGraphemes(t *testing.T) {
	_, model := threeFileRepo(t)
	model, _ = model.Update(key('c'))
	model, _ = model.Update(tab)
	model, _ = paste(model, "a⚠️b")

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if rows := fieldRows(t, model); rows[0] != "ab" {
		t.Fatalf("поле %q, ожидалось ab: backspace стёр кластер не целиком", rows[0])
	}
}

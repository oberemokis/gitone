package tui_test

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

// mutedFg это приглушённый цвет темы в тесте: #030303 в ANSI 38;2;3;3;3.
const mutedFg = "38;2;3;3;3"

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// styleBefore это последний код стиля перед первым вхождением text в строке
// сырого вывода, в которой есть marker.
func styleBefore(t *testing.T, raw, marker, text string) string {
	t.Helper()

	line := rawLine(t, raw, marker)
	at := strings.Index(line, text)
	if at < 0 {
		t.Fatalf("в строке нет %q: %q", text, line)
	}
	codes := sgr.FindAllString(line[:at], -1)
	if len(codes) == 0 {
		return ""
	}
	return codes[len(codes)-1]
}

// mutedTheme открывает три изменённых файла с приглушённым цветом #030303.
func mutedTheme(t *testing.T) tea.Model {
	t.Helper()

	repo, _ := threeFileRepo(t)
	custom := theme.Dark()
	custom.TextMuted = lipgloss.Color("#030303")
	opts := options(repo, layout.Focus())
	opts.Theme = custom
	return resize(open(t, opts), 100, 24)
}

// Клавиша в подсказке идёт цветом текста, подпись приглушённая.
func TestKeysStandOutInHints(t *testing.T) {
	tests := []struct {
		name       string
		open       func(tea.Model) tea.Model
		marker     string
		key, label string
	}{
		{name: "окно лога", open: func(m tea.Model) tea.Model { return openAsync(m, key('l')) }, marker: "commit diff", key: "enter", label: "commit diff"},
		{name: "закрытие в окне лога", open: func(m tea.Model) tea.Model { return openAsync(m, key('l')) }, marker: "commit diff", key: "esc", label: "close"},
		{name: "окно коммита", open: func(m tea.Model) tea.Model { m, _ = m.Update(key('c')); return m }, marker: "cancel", key: "enter", label: "commit"},
		{name: "окно веток", open: func(m tea.Model) tea.Model { return openAsync(m, key('B')) }, marker: "switch", key: "enter", label: "switch"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := tc.open(mutedTheme(t)).View().Content

			if code := styleBefore(t, raw, tc.marker, tc.key); strings.Contains(code, mutedFg) {
				t.Errorf("клавиша %q приглушена: %q", tc.key, code)
			}
			if code := styleBefore(t, raw, tc.marker, " "+tc.label); !strings.Contains(code, mutedFg) {
				t.Errorf("подпись %q не приглушена: %q", tc.label, code)
			}
		})
	}
}

// esc в заголовке поиска и справки и клавиши в столбце справки не приглушены.
func TestKeysStandOutInTitles(t *testing.T) {
	ctrlP := tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}

	tests := []struct {
		name, marker, key string
		press             tea.KeyPressMsg
	}{
		{name: "поиск файлов", press: ctrlP, marker: "Files", key: "esc"},
		{name: "заголовок справки", press: key('?'), marker: "Hotkeys", key: "esc"},
		{name: "клавиша в справке", press: key('?'), marker: "Wrap long lines", key: "w"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model, _ := mutedTheme(t).Update(tc.press)
			line := rawLine(t, model.View().Content, tc.marker)
			// клавиша стоит справа: ищется последнее вхождение
			at := strings.LastIndex(line, tc.key)
			codes := sgr.FindAllString(line[:at], -1)
			if len(codes) > 0 && strings.Contains(codes[len(codes)-1], mutedFg) {
				t.Errorf("%q приглушена: %q", tc.key, line)
			}
		})
	}
}

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/config"
	"gitone/internal/keymap"
	"gitone/internal/layout"
	"gitone/internal/tui"
	"gitone/internal/tui/theme"
)

// Изменённое слово в паре красится фоном слов темы. Проверяется сырой вывод:
// screenOf фон срезает. #010203 в ANSI это 48;2;1;2;3.
func TestChangedWordUsesWordBackground(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "alpha beta gamma\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "alpha BETA gamma\n")

	custom := theme.Dark()
	custom.DiffRemovedWordBg = lipgloss.Color("#010203")

	files, err := tui.Load(repo, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	opts := tui.Options{Config: config.Default(), Keys: keymap.Default(), Theme: custom, Layout: layout.Focus()}
	model, _ := tui.New(opts, files).Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	raw := model.View().Content
	if !strings.Contains(raw, "48;2;1;2;3") {
		t.Fatalf("фон слов не попал в вывод:\n%q", raw)
	}
}

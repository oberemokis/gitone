package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/config"
	"gitone/internal/keymap"
	"gitone/internal/tui"
	"gitone/internal/tui/theme"
)

// Цвета темы доходят до вывода: заголовок диффа красит добавленные строки
// цветом DiffAdded. Проверяется сырой вывод, потому что screenOf цвета срезает.
func TestThemeColorReachesOutput(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\ntwo\n")

	custom := theme.Dark()
	custom.DiffAdded = lipgloss.Color("#123456")

	files, err := tui.Load(repo, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	model, _ := tui.New(tui.Options{Config: config.Default(), Keys: keymap.Default(), Theme: custom}, files).
		Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	// #123456 в ANSI это 38;2;18;52;86
	if raw := model.View().Content; !strings.Contains(raw, "38;2;18;52;86") {
		t.Fatalf("цвет темы не попал в вывод:\n%q", raw)
	}
}

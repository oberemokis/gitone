package tui_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"gitone/internal/layout"
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

	opts := options(repo, layout.Focus())
	opts.Theme = custom
	model := resize(open(t, opts), 100, 20)

	// #123456 в ANSI это 38;2;18;52;86
	if raw := model.View().Content; !strings.Contains(raw, "38;2;18;52;86") {
		t.Fatalf("цвет темы не попал в вывод:\n%q", raw)
	}
}

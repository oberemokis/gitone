package tui_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"gitone/internal/layout"
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

	opts := options(repo, layout.Focus())
	opts.Theme = custom
	raw := resize(open(t, opts), 100, 20).View().Content
	if !strings.Contains(raw, "48;2;1;2;3") {
		t.Fatalf("фон слов не попал в вывод:\n%q", raw)
	}
}

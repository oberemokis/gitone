package tui_test

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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

// Подсветка слов не режет графемный кластер: куски строки с границей внутри
// ZWJ-последовательности дают сумму ширин, не равную ширине строки, и строка
// съезжает от края панели.
func TestChangedEmojiClusterKeepsRowWidth(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "x 👨‍👩‍👧 y\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "x 👨‍👩‍👦 y\n")

	rows := render(open(t, options(repo, layout.Focus())), 100, 20)

	got := []int{ansi.StringWidth(rows[bodyRow]), ansi.StringWidth(rows[bodyRow+1])}
	if want := []int{100, 100}; !slices.Equal(got, want) {
		t.Errorf("ширины строк диффа %v, want %v:\n%q\n%q", got, want, rows[bodyRow], rows[bodyRow+1])
	}
}

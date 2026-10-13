package tui_test

import (
	"slices"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

// wordTheme даёт каждому фону отличимый цвет: фон строки добавления #060606
// (48;2;6;6;6), удаления #080808, фон слова в добавленной строке #0a0a0a,
// в удалённой #0c0c0c, фон номера добавления #0e0e0e, удаления #101010.
func wordTheme() theme.Theme {
	palette := theme.Dark()
	palette.DiffAddedBg = lipgloss.Color("#060606")
	palette.DiffRemovedBg = lipgloss.Color("#080808")
	palette.DiffAddedWordBg = lipgloss.Color("#0a0a0a")
	palette.DiffRemovedWordBg = lipgloss.Color("#0c0c0c")
	palette.DiffAddedNumberBg = lipgloss.Color("#0e0e0e")
	palette.DiffRemovedNumberBg = lipgloss.Color("#101010")
	return palette
}

// Изменённое слово красится фоном слов своей стороны, а слова вокруг него
// остаются на фоне строки: в строке слева «beta», в строке справа «BETA». Фон
// читается по клеткам нужной строки, потому что фон слов удалённой и
// добавленной стороны в обычном выводе встречается рядом.
func TestChangedWordBackgroundBelongsToItsSide(t *testing.T) {
	tests := []struct {
		name, mode, marker string
		width              int
		want               []string
	}{
		{name: "удалённая строка в unified", mode: "unified", marker: "beta", width: 100, want: []string{"16;16;16", "8;8;8", "12;12;12", "8;8;8"}},
		{name: "добавленная строка в unified", mode: "unified", marker: "BETA", width: 100, want: []string{"14;14;14", "6;6;6", "10;10;10", "6;6;6"}},
		{
			name: "обе половины split", mode: "split", marker: "beta", width: 100,
			want: []string{"16;16;16", "8;8;8", "12;12;12", "8;8;8", "14;14;14", "6;6;6", "10;10;10", "6;6;6"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			write(t, repo, "a.txt", "alpha beta gamma\n")
			commitAll(t, repo)
			write(t, repo, "a.txt", "alpha BETA gamma\n")

			opts := options(repo, layout.Focus())
			opts.Theme = wordTheme()
			opts.Config.Diff.Mode = tc.mode
			raw := resize(open(t, opts), tc.width, 20).View().Content

			cells := cellBackgrounds(rawLine(t, raw, tc.marker))
			if got := slices.Compact(cells); !slices.Equal(got, tc.want) {
				t.Errorf("фоны клеток строки с %q: %q, want %q", tc.marker, got, tc.want)
			}
		})
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

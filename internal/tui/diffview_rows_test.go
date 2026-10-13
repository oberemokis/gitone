package tui_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/config"
	"gitone/internal/layout"
)

// openView открывает дифф каталога в виде mode на экране шириной width и
// высотой 30. change правит настройки до открытия.
func openView(t *testing.T, repo, mode string, width int, change func(*config.Config)) tea.Model {
	t.Helper()

	opts := options(repo, layout.Focus())
	opts.Config.Diff.Mode = mode
	if change != nil {
		change(&opts.Config)
	}
	return resize(open(t, opts), width, 30)
}

// halves склеивает строку split экрана шириной 120: левая половина ровно в 60
// колонок, за ней правая.
func halves(left, right string) string {
	return fmt.Sprintf("%-60s%s", left, right)
}

// twoHunkRepo это файл на 30 строк с правками в 5 и 25: контекст в три строки
// даёт два хунка, 2-8 и 22-28.
func twoHunkRepo(t *testing.T) string {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", numberedLines(30))
	commitAll(t, repo)

	edited := strings.Replace(strings.Replace(numberedLines(30), "l5\n", "L5\n", 1), "l25\n", "L25\n", 1)
	write(t, repo, "a.txt", edited)
	return repo
}

// Между хунками одна пустая строка, а не пометка «No newline»: разрыв не имеет
// отношения к концу файла. В обоих видах.
func TestGapBetweenHunksIsBlankRow(t *testing.T) {
	tests := []struct {
		name  string
		mode  string
		width int
		want  []string
	}{
		{
			name: "unified", mode: "unified", width: 100,
			want: []string{
				"  2   l2", "  3   l3", "  4   l4", "  5 - l5", "  5 + L5", "  6   l6", "  7   l7", "  8   l8",
				"",
				" 22   l22",
			},
		},
		{
			name: "split", mode: "split", width: 120,
			want: []string{
				halves("  2   l2", "  2   l2"), halves("  3   l3", "  3   l3"), halves("  4   l4", "  4   l4"),
				halves("  5 - l5", "  5 + L5"),
				halves("  6   l6", "  6   l6"), halves("  7   l7", "  7   l7"), halves("  8   l8", "  8   l8"),
				"",
				halves(" 22   l22", " 22   l22"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model := openView(t, twoHunkRepo(t), tc.mode, tc.width, nil)

			if got := bodyRows(model, len(tc.want)); !slices.Equal(got, tc.want) {
				t.Errorf("строки диффа\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// В split добавленная строка без пары стоит справа, левая половина пустая.
// Ширина 121 нечётная: левая половина 60 колонок, правая 61, и код правой
// кончается на 56 цифре, а не на 55.
func TestSplitAddedLineLeavesLeftHalfEmpty(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "keep\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "keep\n"+digits+"\n")

	model := openView(t, repo, "split", 121, nil)

	want := []string{
		halves(" 1   keep", " 1   keep"),
		strings.Repeat(" ", 60) + " 2 + " + digits[:56],
		strings.Repeat(" ", 60) + "     " + digits[56:112],
	}
	if got := bodyRows(model, len(want)); !slices.Equal(got, want) {
		t.Errorf("строки диффа\n%q\nwant\n%q", got, want)
	}
}

// Удалённая строка без пары стоит слева, правая половина пустая и занимает
// остаток ширины до последней колонки.
func TestSplitRemovedLineFillsRightHalfToEdge(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "keep\ngone\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "keep\n")

	row := screenOf(openView(t, repo, "split", 121, nil))[bodyRow+1]

	if want := " 2 - gone" + strings.Repeat(" ", 121-9); row != want {
		t.Errorf("строка %q (%d колонок), want %q", row, ansi.StringWidth(row), want)
	}
}

// Пометка «No newline» стоит там, где у строки начинается текст: под отступ,
// номер и знак. Без номеров это три колонки, с номерами пять.
func TestNoNewlineMarkerStartsUnderCode(t *testing.T) {
	const marker = "No newline at end of file"
	hideNumbers := func(cfg *config.Config) { cfg.UI.LineNumbers = false }

	tests := []struct {
		name   string
		mode   string
		change func(*config.Config)
		want   []string
	}{
		{
			name: "unified с номерами", mode: "unified",
			want: []string{" 1   a", " 2 - b", "     " + marker, " 2 + B"},
		},
		{
			name: "unified без номеров", mode: "unified", change: hideNumbers,
			want: []string{"   a", " - b", "   " + marker, " + B"},
		},
		{
			name: "split с номерами", mode: "split",
			want: []string{fmt.Sprintf("%-50s%s", " 1   a", " 1   a"), fmt.Sprintf("%-50s%s", " 2 - b", " 2 + B"), "     " + marker},
		},
		{
			name: "split без номеров", mode: "split", change: hideNumbers,
			want: []string{fmt.Sprintf("%-50s%s", "   a", "   a"), fmt.Sprintf("%-50s%s", " - b", " + B"), "   " + marker},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			write(t, repo, "a.txt", "a\nb")
			commitAll(t, repo)
			write(t, repo, "a.txt", "a\nB\n")

			model := openView(t, repo, tc.mode, 100, tc.change)

			if got := bodyRows(model, len(tc.want)); !slices.Equal(got, tc.want) {
				t.Errorf("строки диффа\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// Колонка номеров по самому большому номеру обеих сторон: удаление хвоста
// файла через 99 и 100 строк и добавление его в split и unified. Строка 97
// стоит в тех же колонках, что и 100.
func TestNumberColumnFitsBothSides(t *testing.T) {
	tests := []struct {
		name               string
		mode               string
		oldCount, newCount int
		want               []string
	}{
		{
			name: "unified, удалена строка 100", mode: "unified", oldCount: 100, newCount: 99,
			want: []string{"  97   l97", "  98   l98", "  99   l99", " 100 - l100"},
		},
		{
			name: "unified, добавлена строка 100", mode: "unified", oldCount: 99, newCount: 100,
			want: []string{"  97   l97", "  98   l98", "  99   l99", " 100 + l100"},
		},
		{
			name: "split, удалена строка 100", mode: "split", oldCount: 100, newCount: 99,
			want: []string{halves("  97   l97", "  97   l97"), halves("  98   l98", "  98   l98"), halves("  99   l99", "  99   l99"), " 100 - l100"},
		},
		{
			name: "split, добавлена строка 100", mode: "split", oldCount: 99, newCount: 100,
			want: []string{halves("  97   l97", "  97   l97"), halves("  98   l98", "  98   l98"), halves("  99   l99", "  99   l99"), strings.Repeat(" ", 60) + " 100 + l100"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			write(t, repo, "a.txt", numberedLines(tc.oldCount))
			commitAll(t, repo)
			write(t, repo, "a.txt", numberedLines(tc.newCount))

			model := openView(t, repo, tc.mode, 120, nil)

			if got := bodyRows(model, len(tc.want)); !slices.Equal(got, tc.want) {
				t.Errorf("строки диффа\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// Перенос строки в split идёт по длиннейшей из сторон: у стороны, где текст
// кончился раньше, остаются пустые строки.
func TestSplitWrapFollowsLongerSide(t *testing.T) {
	long := strings.Repeat("c", 30)

	tests := []struct {
		name     string
		old, new string
		want     []string
	}{
		{
			name: "длиннее правая", old: "x", new: long,
			want: []string{fmt.Sprintf("%-25s%s", " 1 - x", " 1 + "+long[:20]), strings.Repeat(" ", 30) + long[:10]},
		},
		{
			name: "длиннее левая", old: long, new: "x",
			want: []string{" 1 - " + long[:20] + " 1 + x", "     " + long[:10]},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			write(t, repo, "a.txt", tc.old+"\n")
			commitAll(t, repo)
			write(t, repo, "a.txt", tc.new+"\n")

			// половина 25 колонок: номер и знак 5, на код 20
			model := openView(t, repo, "split", 50, nil)

			if got := bodyRows(model, len(tc.want)); !slices.Equal(got, tc.want) {
				t.Errorf("строки диффа\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// Строка ровно в две ширины колонки занимает две строки экрана, третьей, пустой,
// нет: следующая строка файла идёт сразу за ней.
func TestWrapLineOfExactlyTwoWidths(t *testing.T) {
	model := narrowDiff(t, nil, strings.Repeat("a", 40), "bbb")

	want := []string{" 1 - x", " 1 + " + strings.Repeat("a", 20), "     " + strings.Repeat("a", 20), " 2 + bbb"}
	if got := bodyRows(model, len(want)); !slices.Equal(got, want) {
		t.Errorf("строки диффа\n%q\nwant\n%q", got, want)
	}
}

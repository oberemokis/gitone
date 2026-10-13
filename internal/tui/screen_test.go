package tui_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

func TestFirstChangedFileOpensWithPathAndCounts(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\ntwo\n")
	write(t, repo, "b.txt", "x\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\nTWO\nthree\n")
	write(t, repo, "b.txt", "y\n")

	screen := render(open(t, options(repo, layout.Focus())), 100, 20)

	if header := headerText(screen); header != "a.txt +2 -1 (1/2)" {
		t.Fatalf("заголовок %q, ожидался %q", header, "a.txt +2 -1 (1/2)")
	}
	if body := strings.Join(screen[1:], "\n"); !strings.Contains(body, "TWO") || strings.Contains(body, "y") {
		t.Fatalf("на экране не первый файл:\n%s", body)
	}
}

func TestModeFollowsWidthUntilToggled(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\ntwo\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\nTWO\n")
	model := open(t, options(repo, layout.Focus()))
	toggle := tea.KeyPressMsg{Code: 'm', Text: "m"}

	steps := []struct {
		name      string
		msg       tea.Msg
		wantSplit bool
	}{
		{name: "ширина 120", msg: tea.WindowSizeMsg{Width: 120, Height: 20}, wantSplit: true},
		{name: "ширина 119", msg: tea.WindowSizeMsg{Width: 119, Height: 20}, wantSplit: false},
		{name: "m на 119", msg: toggle, wantSplit: true},
		{name: "после m ширина 100", msg: tea.WindowSizeMsg{Width: 100, Height: 20}, wantSplit: true},
		{name: "второй m", msg: toggle, wantSplit: false},
		{name: "после второго m ширина 150", msg: tea.WindowSizeMsg{Width: 150, Height: 20}, wantSplit: false},
	}

	for _, step := range steps {
		model, _ = model.Update(step.msg)

		if got := isSplit(screenOf(model), "two", "TWO"); got != step.wantSplit {
			t.Fatalf("%s: split %v, ожидалось %v\n%s", step.name, got, step.wantSplit, strings.Join(screenOf(model), "\n"))
		}
	}
}

// Без переноса длинная строка режется по краю колонки: в каждой половине split
// остаётся 55 колонок кода после номера и знака, и экран не шире 120. Таб
// становится ровно четырьмя пробелами.
func TestLongLinesAreCutAtColumnEdgeAndTabsBecomeSpaces(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "short\n"+strings.Repeat("x", 300)+"\n\ttabbed\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "SHORT\n"+strings.Repeat("y", 300)+"\n\tTABBED\n")

	model := openView(t, repo, "split", 120, noWrap)

	want := []string{
		halves(" 1 - short", " 1 + SHORT"),
		halves(" 2 - "+strings.Repeat("x", 55), " 2 + "+strings.Repeat("y", 55)),
		halves(" 3 -     tabbed", " 3 +     TABBED"),
	}
	if got := bodyRows(model, len(want)); !slices.Equal(got, want) {
		t.Errorf("строки диффа\n%q\nwant\n%q", got, want)
	}
	for index, line := range screenOf(model)[2 : bodyRow+len(want)] {
		if width := ansi.StringWidth(line); width != 120 {
			t.Errorf("строка экрана %d шириной %d, want 120: %q", index+2, width, line)
		}
	}
}

func TestScrollStopsAtBothEnds(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "seed.txt", "seed\n")
	commitAll(t, repo)
	var content strings.Builder
	for number := 1; number <= 60; number++ {
		fmt.Fprintf(&content, "l%d\n", number)
	}
	write(t, repo, "long.txt", content.String())

	// высота 20: заголовок и подвал, оба с пустыми строками над и под ними,
	// оставляют телу 14 строк, полэкрана это 7
	model := open(t, options(repo, layout.Focus()))
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	steps := []struct {
		name    string
		msg     tea.Msg
		wantTop int
	}{
		{name: "j", msg: tea.KeyPressMsg{Code: 'j', Text: "j"}, wantTop: 2},
		{name: "стрелка вниз", msg: tea.KeyPressMsg{Code: tea.KeyDown}, wantTop: 3},
		{name: "k", msg: tea.KeyPressMsg{Code: 'k', Text: "k"}, wantTop: 2},
		{name: "стрелка вверх", msg: tea.KeyPressMsg{Code: tea.KeyUp}, wantTop: 1},
		{name: "k в начале", msg: tea.KeyPressMsg{Code: 'k', Text: "k"}, wantTop: 1},
		{name: "ctrl d", msg: tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}, wantTop: 8},
		{name: "колесо вниз", msg: tea.MouseWheelMsg{Button: tea.MouseWheelDown}, wantTop: 11},
		{name: "колесо вверх", msg: tea.MouseWheelMsg{Button: tea.MouseWheelUp}, wantTop: 8},
		{name: "ctrl u", msg: tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}, wantTop: 1},
		{name: "G", msg: tea.KeyPressMsg{Code: 'g', Text: "G", Mod: tea.ModShift}, wantTop: 47},
		{name: "j в конце", msg: tea.KeyPressMsg{Code: 'j', Text: "j"}, wantTop: 47},
		{name: "колесо в конце", msg: tea.MouseWheelMsg{Button: tea.MouseWheelDown}, wantTop: 47},
		{name: "g", msg: tea.KeyPressMsg{Code: 'g', Text: "g"}, wantTop: 1},
	}

	for _, step := range steps {
		model, _ = model.Update(step.msg)

		if got := topNumber(t, screenOf(model)); got != step.wantTop {
			t.Fatalf("%s: первая строка тела %d, ожидалась %d", step.name, got, step.wantTop)
		}
	}
}

func TestModeToggleKeepsTopLine(t *testing.T) {
	repo := newRepo(t)
	var before, after strings.Builder
	for number := 1; number <= 35; number++ {
		fmt.Fprintf(&before, "r%d\n", number)
		fmt.Fprintf(&after, "a%d\n", number)
	}
	write(t, repo, "a.txt", before.String())
	commitAll(t, repo)
	write(t, repo, "a.txt", after.String())

	// unified: 35 удалённых строк, затем 35 добавленных. Пять раз на
	// полэкрана (по 7 строк) ставят наверх первую добавленную, её номер 1.
	model := open(t, options(repo, layout.Focus()))
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	for range 5 {
		model, _ = model.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	}
	if top := strings.Fields(screenOf(model)[bodyRow]); len(top) < 3 || top[2] != "a1" {
		t.Fatalf("наверху не первая добавленная строка: %v", top)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})

	if got := topNumber(t, screenOf(model)); got != 1 {
		t.Fatalf("после m наверху строка %d, ожидалась та же строка 1", got)
	}
}

func TestSpecialFilesShowNotices(t *testing.T) {
	cases := []struct {
		name   string
		before map[string]string
		change func(t *testing.T, repo string)
		args   []string
		want   string
	}{
		{
			name:   "бинарный файл",
			before: map[string]string{"image.bin": "\x00\x01"},
			change: func(t *testing.T, repo string) { write(t, repo, "image.bin", "\x00\x02") },
			want:   "Binary or large file",
		},
		{
			name:   "смена прав",
			before: map[string]string{"script.sh": "echo\n"},
			change: func(t *testing.T, repo string) {
				if err := os.Chmod(filepath.Join(repo, "script.sh"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: "mode 644 → 755",
		},
		{
			name:   "пустой новый файл",
			before: map[string]string{"seed.txt": "seed\n"},
			change: func(t *testing.T, repo string) { write(t, repo, "empty.txt", "") },
			want:   "No content changes",
		},
		{
			name:   "нераспознанный вывод git показывается как есть",
			before: map[string]string{"a.txt": "a\n"},
			change: func(t *testing.T, repo string) { write(t, repo, "a.txt", "b\n") },
			args:   []string{"--stat"},
			want:   "1 file changed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			for name, content := range tc.before {
				write(t, repo, name, content)
			}
			commitAll(t, repo)
			tc.change(t, repo)

			opts := options(repo, layout.Focus())
			opts.Args = tc.args
			screen := render(open(t, opts), 100, 20)

			if body := strings.Join(screen, "\n"); !strings.Contains(body, tc.want) {
				t.Fatalf("на экране нет %q:\n%s", tc.want, body)
			}
		})
	}
}

// Пометка «No newline» стоит на своей стороне и серая: в unified под строкой, у
// которой нет перевода, в split в той половине, где его нет, а другая половина
// пуста. У старой версии пометка идёт после удалённой строки, у новой после
// добавленной.
func TestNoNewlineMarkerSitsOnItsSideAndIsMuted(t *testing.T) {
	const marker = "No newline at end of file"
	// половина split на экране 100 колонок
	pair := func(left, right string) string { return fmt.Sprintf("%-50s%s", left, right) }

	tests := []struct {
		name, mode, old, new string
		want                 []string
	}{
		{
			name: "unified, перевода нет у старой версии", mode: "unified", old: "a\nb", new: "a\nB\n",
			want: []string{" 1   a", " 2 - b", "     " + marker, " 2 + B"},
		},
		{
			name: "unified, перевода нет у новой версии", mode: "unified", old: "a\nb\n", new: "a\nB",
			want: []string{" 1   a", " 2 - b", " 2 + B", "     " + marker},
		},
		{
			name: "split, перевода нет у старой версии", mode: "split", old: "a\nb", new: "a\nB\n",
			want: []string{pair(" 1   a", " 1   a"), pair(" 2 - b", " 2 + B"), "     " + marker},
		},
		{
			name: "split, перевода нет у новой версии", mode: "split", old: "a\nb\n", new: "a\nB",
			want: []string{pair(" 1   a", " 1   a"), pair(" 2 - b", " 2 + B"), pair("", "     "+marker)},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			write(t, repo, "a.txt", tc.old)
			commitAll(t, repo)
			write(t, repo, "a.txt", tc.new)

			custom := theme.Dark()
			custom.TextMuted = lipgloss.Color("#030303")
			opts := options(repo, layout.Focus())
			opts.Theme = custom
			opts.Config.Diff.Mode = tc.mode
			model := resize(open(t, opts), 100, 20)

			if got := bodyRows(model, len(tc.want)); !slices.Equal(got, tc.want) {
				t.Errorf("строки диффа\n%q\nwant\n%q", got, tc.want)
			}
			if got := styleBefore(t, model.View().Content, marker, marker); !strings.Contains(got, mutedFg) {
				t.Errorf("перед пометкой %q, want приглушённый цвет %s", got, mutedFg)
			}
		})
	}
}

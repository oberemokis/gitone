package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
)

// keycap ansi.Truncate и ansi.Wrap в x/ansi v0.11.9 считают одной клеткой, а
// ansi.StringWidth двумя: строка, обрезанная по одной мере и дополненная по
// другой, роняла gitone отрицательным strings.Repeat (#377).
const keycap = "1️⃣"

// fitsScreen падает, если строка экрана шире width.
func fitsScreen(t *testing.T, model tea.Model, width int) {
	t.Helper()

	for row, line := range screenOf(model) {
		if got := ansi.StringWidth(line); got > width {
			t.Fatalf("строка %d шириной %d на экране %d:\n%s", row, got, width, line)
		}
	}
}

// Вопрос discard с keycap в имени файла держит подвал в ширину экрана
// (#377, случай 1).
func TestDiscardQuestionWithKeycapFitsFooter(t *testing.T) {
	repo := newRepo(t)
	name := "notes-" + keycap + "-x.txt"
	write(t, repo, name, "old\n")
	commitAll(t, repo)
	write(t, repo, name, "new\n")

	model, _ := open(t, options(repo, layout.Focus())).Update(key('x'))
	for _, width := range []int{46, 80, 100, 138} {
		model = resize(model, width, 20)
		fitsScreen(t, model, width)

		screen := screenOf(model)
		footer := screen[footerRow(screen)]
		if got := ansi.StringWidth(footer); got != width || !strings.Contains(footer, "Discard") {
			t.Fatalf("ширина %d: подвал %q шириной %d, ожидался вопрос во всю ширину", width, footer, got)
		}
	}
}

// Ветка с keycap в подвале главного экрана на узком экране (случай 2).
func TestBranchWithKeycapOnNarrowScreen(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "old\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "release-1️⃣2️⃣3️⃣")
	write(t, repo, "a.txt", "new\n")

	model := open(t, options(repo, layout.Focus()))
	for width := 31; width <= 40; width++ {
		model = resize(model, width, 20)
		fitsScreen(t, model, width)
	}
}

// Тема коммита с keycap в таблице лога (случай 3).
func TestLogSubjectWithKeycap(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "1\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "Step "+keycap+" of the migration: move the tables")

	model := openAsync(resize(open(t, options(repo, layout.Focus())), 80, 20), key('l'))
	for _, width := range []int{80, 100, 120} {
		model = resize(model, width, 20)
		fitsScreen(t, model, width)
		if screen := screenText(model); !strings.Contains(screen, "Step") {
			t.Fatalf("ширина %d: в логе нет темы коммита:\n%s", width, screen)
		}
	}
}

// Заметка с keycap и с CJK в тосте (случай 4). Строка, которую ansi.Wrap
// считает влезшей, а она шире тоста, переносит хвост, а не теряет его: все
// двадцать keycap на месте.
func TestToastWithWideCells(t *testing.T) {
	for _, test := range []struct {
		text    string
		widths  []int
		keycaps int
	}{
		{"Step " + keycap + " failed in a.txt", []int{80, 200}, 1},
		{strings.Repeat(keycap, 20) + " end", []int{40}, 20},
		{"提交失败了", []int{13}, 0},
	} {
		repo := stagedRepo(t)
		failingHook(t, repo, "echo '"+test.text+"' >&2")
		model := commitX(resize(open(t, options(repo, layout.Default())), 100, 20))

		for _, width := range test.widths {
			model = resize(model, width, 20)
			fitsScreen(t, model, width)
			if got := strings.Count(noticeOf(model), keycap); got != test.keycaps {
				t.Fatalf("ширина %d: в тосте %q keycap %d, ожидалось %d", width, noticeOf(model), got, test.keycaps)
			}
		}
	}
}

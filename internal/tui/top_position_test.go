package tui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// gappedFile открывает файл на 200 строк с правками в 10-14, 60, 90, 120, 150
// и 180: между хунками разрывы. Правка меняет l на L и дописывает tail, экран
// 100 на 20 это unified.
func gappedFile(t *testing.T, tail string) tea.Model {
	t.Helper()

	edited := map[int]bool{10: true, 11: true, 12: true, 13: true, 14: true, 60: true, 90: true, 120: true, 150: true, 180: true}
	var before, after strings.Builder
	for number := 1; number <= 200; number++ {
		fmt.Fprintf(&before, "l%d\n", number)
		if edited[number] {
			fmt.Fprintf(&after, "L%d%s\n", number, tail)
		} else {
			fmt.Fprintf(&after, "l%d\n", number)
		}
	}

	repo := newRepo(t)
	write(t, repo, "a.txt", before.String())
	commitAll(t, repo)
	write(t, repo, "a.txt", after.String())

	return resize(open(t, options(repo, layout.Focus())), 100, 20)
}

// scrollDown нажимает j count раз.
func scrollDown(model tea.Model, count int) tea.Model {
	for range count {
		model, _ = model.Update(key('j'))
	}
	return model
}

// gapOnTop проверяет, что наверху экрана разрыв между хунками, а под ним
// строка 57: контекст хунка с правкой в 60.
func gapOnTop(t *testing.T, model tea.Model) {
	t.Helper()

	screen := screenOf(model)
	if top := strings.TrimSpace(screen[bodyRow]); top != "" {
		t.Fatalf("наверху %q, want разрыв между хунками:\n%s", top, strings.Join(screen, "\n"))
	}
	if got := strings.Fields(screen[bodyRow+1])[0]; got != "57" {
		t.Fatalf("под разрывом строка %s, want 57:\n%s", got, strings.Join(screen, "\n"))
	}
}

// Наверху разрыв, под ним строка 57. Смена вида не должна унести строки 57-60
// вместе с правкой за верх экрана: у разрыва нет номера, и верх искали бы по
// прежнему индексу, который в другом виде указывает на другое место.
func TestToggleModeKeepsTopAtGap(t *testing.T) {
	model := scrollDown(gappedFile(t, ""), 16)
	gapOnTop(t, model)

	model, _ = model.Update(key('m'))

	if got := topNumber(t, screenOf(model)); got != 57 {
		t.Errorf("после m наверху строка %d, want 57", got)
	}
}

// Перенос меняет число строк экрана так же, как смена вида: длинные правки
// занимают по три строки.
func TestToggleWrapKeepsTopAtGap(t *testing.T) {
	// шесть строк контекста, пять удалённых и пять добавленных по три строки
	model := scrollDown(gappedFile(t, strings.Repeat("x", 190)), 26)
	gapOnTop(t, model)

	model, _ = model.Update(key('w'))

	if got := topNumber(t, screenOf(model)); got != 57 {
		t.Errorf("после w наверху строка %d, want 57", got)
	}
}

// У пометки «No newline» номера тоже нет: верх берётся по строке под ней, а не
// по индексу.
func TestToggleModeKeepsTopAtNoNewlineMarker(t *testing.T) {
	var before, after strings.Builder
	for number := 1; number <= 200; number++ {
		fmt.Fprintf(&before, "l%d\n", number)
		if number > 170 {
			fmt.Fprintf(&after, "L%d\n", number)
		} else {
			fmt.Fprintf(&after, "l%d\n", number)
		}
	}
	repo := newRepo(t)
	write(t, repo, "a.txt", strings.TrimSuffix(before.String(), "\n"))
	commitAll(t, repo)
	write(t, repo, "a.txt", strings.TrimSuffix(after.String(), "\n"))
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	// три строки контекста и тридцать удалённых, затем пометка под последней
	model = scrollDown(model, 33)
	if top := screenOf(model)[bodyRow]; !strings.Contains(top, "No newline at end of file") {
		t.Fatalf("наверху %q, want пометку:\n%s", top, screenText(model))
	}

	model, _ = model.Update(key('m'))

	if got := topNumber(t, screenOf(model)); got != 171 {
		t.Errorf("после m наверху строка %d, want 171", got)
	}
}

// Ширина 120 переключает auto на split, строк становится меньше.
func TestResizeToSplitKeepsTopAtGap(t *testing.T) {
	model := scrollDown(gappedFile(t, ""), 16)
	gapOnTop(t, model)

	model = resize(model, 120, 20)

	if got := topNumber(t, screenOf(model)); got != 57 {
		t.Errorf("после смены ширины наверху строка %d, want 57", got)
	}
}

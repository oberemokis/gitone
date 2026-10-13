package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
)

// dragMouse нажимает левую кнопку в fromX, ведёт мышь до toX по строке y и
// отпускает.
func dragMouse(model tea.Model, fromX, toX, y int) tea.Model {
	model, _ = model.Update(tea.MouseClickMsg{X: fromX, Y: y, Button: tea.MouseLeft})
	model, _ = model.Update(tea.MouseMotionMsg{X: toX, Y: y, Button: tea.MouseLeft})
	model, _ = model.Update(tea.MouseReleaseMsg{X: toX, Y: y, Button: tea.MouseLeft})
	return model
}

// textColumn это колонка экрана, с которой написан text.
func textColumn(t *testing.T, model tea.Model, text string) int {
	t.Helper()
	return column(rowWith(t, screenOf(model), text), text)
}

// В default на экране 100 колонок панель занимает 20: стык это колонки 19
// и 20. Стык вправо на 10 сдвигает дифф на 10 (#121).
func TestDragFilesEdgeWidensPanel(t *testing.T) {
	model := toggleFiles(openLayout(t, layout.Default(), 100))
	before := textColumn(t, model, "new a")

	for _, grab := range []int{19, 20} {
		dragged := dragMouse(model, grab, grab+10, bodyRow+5)
		if after := textColumn(t, dragged, "new a"); after != before+10 {
			t.Errorf("стык из колонки %d: дифф с колонки %d, want %d", grab, after, before+10)
		}
	}
}

// Панель не уже 12 колонок, дифф не уже 20.
func TestDragFilesEdgeStopsAtLimits(t *testing.T) {
	model := toggleFiles(openLayout(t, layout.Default(), 100))
	before := textColumn(t, model, "new a")

	if after := textColumn(t, dragMouse(model, 20, 0, bodyRow+5), "new a"); after != before-8 {
		t.Errorf("панель уже 12 колонок: дифф с колонки %d, want %d", after, before-8)
	}
	if after := textColumn(t, dragMouse(model, 20, 99, bodyRow+5), "new a"); after != before+60 {
		t.Errorf("дифф уже 20 колонок: дифф с колонки %d, want %d", after, before+60)
	}
}

// f закрывает и открывает панель с той же шириной.
func TestFilesWidthSurvivesToggle(t *testing.T) {
	model := dragMouse(toggleFiles(openLayout(t, layout.Default(), 100)), 20, 30, bodyRow+5)
	dragged := textColumn(t, model, "new a")

	if after := textColumn(t, toggleFiles(toggleFiles(model)), "new a"); after != dragged {
		t.Errorf("после f дважды дифф с колонки %d, want %d", after, dragged)
	}
}

// screenWidth это ширина самой широкой строки экрана.
func screenWidth(model tea.Model) int {
	widest := 0
	for _, line := range screenOf(model) {
		widest = max(widest, ansi.StringWidth(line))
	}
	return widest
}

// Панель, вытянутая до 80 колонок из 100, после сужения окна до 70 уступает
// диффу его 20 колонок, строки остаются в экране (#151).
func TestFilesWidthClampedWhenWindowShrinks(t *testing.T) {
	model := dragMouse(toggleFiles(openLayout(t, layout.Default(), 100)), 20, 80, bodyRow+5)

	narrow := resize(model, 70, 20)

	// текст диффа стоит на 5 колонок правее его левого края
	if got := textColumn(t, narrow, "new a"); got != 55 {
		t.Errorf("после сужения до 70 дифф с колонки %d, want 55", got)
	}
	if got := screenWidth(narrow); got != 70 {
		t.Errorf("после сужения до 70 строки шириной %d, want 70", got)
	}
}

// Экран уже суммы пределов 12 и 20: мышь не применяется, панель как в
// раскладке, 20% от 30 колонок.
func TestFilesWidthIgnoredOnTinyWindow(t *testing.T) {
	model := dragMouse(toggleFiles(openLayout(t, layout.Default(), 100)), 20, 80, bodyRow+5)

	tiny := resize(model, 30, 20)

	if got := textColumn(t, tiny, "new a"); got != 11 {
		t.Errorf("на экране 30 дифф с колонки %d, want 11", got)
	}
	if got := screenWidth(tiny); got != 30 {
		t.Errorf("на экране 30 строки шириной %d, want 30", got)
	}
}

// Сужение окна не стирает ширину, выбранную мышью: после возврата размера
// панель снова 80 колонок.
func TestFilesWidthReturnsWithWindow(t *testing.T) {
	model := dragMouse(toggleFiles(openLayout(t, layout.Default(), 100)), 20, 80, bodyRow+5)

	back := resize(resize(model, 30, 20), 100, 20)

	if got := textColumn(t, back, "new a"); got != 85 {
		t.Errorf("после возврата до 100 дифф с колонки %d, want 85", got)
	}
}

// После сужения стык стоит на краю урезанной панели и тянется мышью: панель
// 50 колонок из 70, стык это колонки 49 и 50.
func TestDragFilesEdgeAfterShrink(t *testing.T) {
	model := resize(dragMouse(toggleFiles(openLayout(t, layout.Default(), 100)), 20, 80, bodyRow+5), 70, 20)

	for _, grab := range []int{49, 50} {
		dragged := dragMouse(model, grab, grab-10, bodyRow+5)
		if after := textColumn(t, dragged, "new a"); after != 45 {
			t.Errorf("стык из колонки %d: дифф с колонки %d, want 45", grab, after)
		}
	}
}

// Колонки рядом со стыком (18 и 21 при стыке 19 и 20) панель не тянут: в 18
// кончается строка b.txt, клик открывает файл, в 21 начинается дифф.
func TestClickBesideFilesEdgeDoesNotDrag(t *testing.T) {
	model := toggleFiles(openLayout(t, layout.Default(), 100))

	opened := dragMouse(model, 18, 28, bodyRow+2)
	if file := openedFile(screenOf(opened)); file != "b.txt" {
		t.Errorf("клик в колонку 18 открыл %q, want b.txt", file)
	}
	if after := textColumn(t, opened, "new b"); after != 25 {
		t.Errorf("после клика в колонку 18 дифф с колонки %d, want 25", after)
	}

	if after := textColumn(t, dragMouse(model, 21, 31, bodyRow+2), "new a"); after != 25 {
		t.Errorf("после клика в колонку 21 дифф с колонки %d, want 25", after)
	}
}

// Панель справа в своей раскладке: стык влево расширяет её.
func TestDragRightFilesEdge(t *testing.T) {
	right := layout.Node{Direction: "row", Children: []layout.Node{
		{Pane: "diff"},
		{Pane: "files", Size: layout.Size{Percent: 30}},
	}}
	model := toggleFiles(openLayout(t, right, 100))
	before := textColumn(t, model, "b.txt")

	if after := textColumn(t, dragMouse(model, 70, 60, bodyRow+5), "b.txt"); after != before-10 {
		t.Errorf("панель справа: b.txt с колонки %d, want %d", after, before-10)
	}
}

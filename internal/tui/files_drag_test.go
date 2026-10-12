package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

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

package tui_test

import (
	"testing"

	"gitone/internal/layout"
)

// На экране 140 колонок дифф в split, половины по 70: стык это колонки 69 и
// 70. Стык вправо на 10 сдвигает правую половину на 10 (#122).
func TestDragSplitWidensLeftHalf(t *testing.T) {
	model := openLayout(t, layout.Focus(), 140)
	before := textColumn(t, model, "new a")

	for _, grab := range []int{69, 70} {
		dragged := dragMouse(model, grab, grab+10, bodyRow+3)
		if after := textColumn(t, dragged, "new a"); after != before+10 {
			t.Errorf("стык из колонки %d: правая половина с колонки %d, want %d", grab, after, before+10)
		}
	}
}

// Колонки рядом со стыком (68 и 71 при стыке 69 и 70) половины не тянут.
func TestClickBesideSplitEdgeDoesNotDrag(t *testing.T) {
	model := openLayout(t, layout.Focus(), 140)
	before := textColumn(t, model, "new a")

	for _, grab := range []int{68, 71} {
		dragged := dragMouse(model, grab, grab+10, bodyRow+3)
		if after := textColumn(t, dragged, "new a"); after != before {
			t.Errorf("клик в колонку %d: правая половина с колонки %d, want %d", grab, after, before)
		}
	}
}

// Половина не уже 20 колонок.
func TestDragSplitStopsAtLimits(t *testing.T) {
	model := openLayout(t, layout.Focus(), 140)
	before := textColumn(t, model, "new a")

	if after := textColumn(t, dragMouse(model, 70, 0, bodyRow+3), "new a"); after != before-50 {
		t.Errorf("левая уже 20 колонок: правая с колонки %d, want %d", after, before-50)
	}
	if after := textColumn(t, dragMouse(model, 70, 139, bodyRow+3), "new a"); after != before+50 {
		t.Errorf("правая уже 20 колонок: правая с колонки %d, want %d", after, before+50)
	}
}

// Переход к другому файлу долю не сбрасывает.
func TestSplitShareSurvivesFileSwitch(t *testing.T) {
	model := dragMouse(openLayout(t, layout.Focus(), 140), 70, 80, bodyRow+3)
	dragged := textColumn(t, model, "new a")

	model, _ = model.Update(key(']'))
	if after := textColumn(t, model, "new b"); after != dragged {
		t.Errorf("после ] правая половина с колонки %d, want %d", after, dragged)
	}
}

// Смена ширины терминала сохраняет долю: левая половина 80 из 140 на
// экране 120 это 69 колонок, на 9 шире половины.
func TestSplitShareSurvivesResize(t *testing.T) {
	half := textColumn(t, resize(openLayout(t, layout.Focus(), 140), 120, 20), "new a")

	model := dragMouse(openLayout(t, layout.Focus(), 140), 70, 80, bodyRow+3)
	if after := textColumn(t, resize(model, 120, 20), "new a"); after != half+9 {
		t.Errorf("после смены ширины правая половина с колонки %d, want %d", after, half+9)
	}
}

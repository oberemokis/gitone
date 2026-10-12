package tui

import (
	"image"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
	"gitone/internal/tui/zone"
)

// filesEdgeTarget это место клика на стыке панели файлов и диффа (#121).
const filesEdgeTarget = "edge:files"

// Пределы ширины панели файлов при перетаскивании: список ещё читается, а
// диффу остаётся место.
const (
	minFilesWidth = 12
	minDiffWidth  = 20
)

// drag это перетаскивание стыка: где нажали, какая была ширина и в какую
// сторону она растёт с движением мыши вправо. Нулевое значение значит, что
// ничего не тянут.
type drag struct {
	active bool
	x      int
	width  int
	// 1 у панели слева от диффа, -1 у панели справа
	sign int
}

// filesEdge это стык панели файлов и диффа: последняя колонка одного и
// первая другого, рамки нет. Стыка нет у закрытой панели и у панели над или
// под диффом: тянется только панель во всю высоту тела.
func (m Model) filesEdge() (edge image.Rectangle, panel image.Rectangle, found bool) {
	area, found := paneArea(m.placement(), "files", image.Pt(0, m.titleHeight()))
	if !found || area.Dy() != m.bodyHeight() || area.Dx() >= m.width {
		return image.Rectangle{}, image.Rectangle{}, false
	}

	x := area.Max.X - 1
	if area.Min.X > 0 {
		x = area.Min.X - 1
	}
	return image.Rect(x, area.Min.Y, x+2, area.Max.Y), area, true
}

// startDrag начинает тянуть стык панели файлов из колонки x.
func (m Model) startDrag(x int) Model {
	_, panel, found := m.filesEdge()
	if !found {
		return m
	}

	sign := 1
	if panel.Min.X > 0 {
		sign = -1
	}
	m.drag = drag{active: true, x: x, width: panel.Dx(), sign: sign}
	return m
}

// dragTo ведёт стык за мышью: ширина панели меняется на столько колонок,
// на сколько сдвинулась мышь, в пределах minFilesWidth и minDiffWidth.
// Дифф получает новую ширину сразу.
func (m Model) dragTo(x int) Model {
	if !m.drag.active || m.width-minDiffWidth < minFilesWidth {
		return m
	}

	width := m.drag.width + m.drag.sign*(x-m.drag.x)
	m.filesWidth = min(max(width, minFilesWidth), m.width-minDiffWidth)
	return m.resize()
}

// mouseDrag разбирает движение и отпускание кнопки, пока тянут стык.
func (m Model) mouseDrag(msg tea.Msg) Model {
	switch msg := msg.(type) {
	case tea.MouseMotionMsg:
		return m.dragTo(msg.X)
	case tea.MouseReleaseMsg:
		m.drag = drag{}
	}
	return m
}

// paneArea это прямоугольник панели на экране: дерево размещения кладёт
// детей подряд по своей оси, начиная с origin.
func paneArea(placed layout.Placed, pane string, origin image.Point) (image.Rectangle, bool) {
	if placed.Pane == pane && len(placed.Children) == 0 {
		return image.Rectangle{Min: origin, Max: origin.Add(image.Pt(placed.Width, placed.Height))}, true
	}

	for _, child := range placed.Children {
		if area, found := paneArea(child, pane, origin); found {
			return area, true
		}
		if placed.Direction == "column" {
			origin.Y += child.Height
		} else {
			origin.X += child.Width
		}
	}

	return image.Rectangle{}, false
}

// edgeZones это места стыков, их кладут поверх строк панелей.
func (m Model) edgeZones() []zone.Zone {
	if edge, _, found := m.filesEdge(); found {
		return []zone.Zone{{Target: filesEdgeTarget, Area: edge}}
	}
	return nil
}

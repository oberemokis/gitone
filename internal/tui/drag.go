package tui

import (
	"image"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
	"gitone/internal/tui/zone"
)

// Места клика на стыках: панели файлов и диффа (#121), половин split (#122).
const (
	filesEdgeTarget = "edge:files"
	splitEdgeTarget = "edge:split"
)

// Пределы ширины панели файлов при перетаскивании и при раскладке (arrangement):
// список ещё читается, а диффу остаётся место.
const (
	minFilesWidth = 12
	minDiffWidth  = 20
)

// drag это перетаскивание стыка: где нажали, какая была ширина и в какую
// сторону она растёт с движением мыши вправо. Нулевое значение значит, что
// ничего не тянут.
type drag struct {
	active bool
	// split: тянут стык половин диффа, width это левая половина; иначе край
	// панели файлов
	split bool
	x     int
	width int
	// 1 у панели слева от диффа и у стыка половин, -1 у панели справа
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

// splitEdge это стык половин split: последняя колонка левой и первая правой,
// разделителя нет. В unified и без файлов стыка нет.
func (m Model) splitEdge() (edge image.Rectangle, left int, found bool) {
	area, found := paneArea(m.placement(), "diff", image.Pt(0, m.titleHeight()))
	left, split := m.view.SplitLeft()
	if !found || !split || len(m.files) == 0 {
		return image.Rectangle{}, 0, false
	}

	x := area.Min.X + left - 1
	return image.Rect(x, area.Min.Y, x+2, area.Max.Y), left, true
}

// startDrag начинает тянуть стык target из колонки x.
func (m Model) startDrag(target string, x int) Model {
	switch target {
	case filesEdgeTarget:
		if _, panel, found := m.filesEdge(); found {
			sign := 1
			if panel.Min.X > 0 {
				sign = -1
			}
			m.drag = drag{active: true, x: x, width: panel.Dx(), sign: sign}
		}
	case splitEdgeTarget:
		if _, left, found := m.splitEdge(); found {
			m.drag = drag{active: true, split: true, x: x, width: left, sign: 1}
		}
	}
	return m
}

// dragTo ведёт стык за мышью: ширина меняется на столько колонок, на сколько
// сдвинулась мышь. Панель файлов держится в пределах minFilesWidth и
// minDiffWidth, и дифф получает новую ширину сразу; пределы половин держит
// сам дифф.
func (m Model) dragTo(x int) Model {
	if !m.drag.active {
		return m
	}

	width := m.drag.width + m.drag.sign*(x-m.drag.x)
	if m.drag.split {
		m.view = m.view.WithSplit(width)
		return m
	}
	if m.width-minDiffWidth < minFilesWidth {
		return m
	}
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
// детей подряд по своей оси, начиная с origin. Панель это лист дерева с её
// именем, и это единственное место, где так её ищут: резайз берёт отсюда
// размер, стыки положение.
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
	var zones []zone.Zone
	if edge, _, found := m.filesEdge(); found {
		zones = append(zones, zone.Zone{Target: filesEdgeTarget, Area: edge})
	}
	if edge, _, found := m.splitEdge(); found {
		zones = append(zones, zone.Zone{Target: splitEdgeTarget, Area: edge})
	}
	return zones
}

package diffview

import "gitone/internal/diff"

// selection это выделение изменённых строк для индекса (#212). Курсор и якорь
// это изменённые строки по номерам, а не по индексу на экране: так выделение
// переживает смену вида, ширины и перенос. at это место курсора на новой
// стороне (newPositions): по нему курсор находит место, когда его строка
// пропала. hunk значит, что выделен весь хунк под курсором, иначе строки от
// якоря до курсора. conflict значит выбор конфликтов (conflict.go): выделен
// конфликт под курсором.
type selection struct {
	on       bool
	hunk     bool
	conflict bool
	cursor   diff.LineRef
	at       int
	anchor   diff.LineRef
}

// Select показывает file и ставит курсор на первую изменённую строку экрана,
// выделен хунк под ней. Верх экрана держится по новой стороне: она у диффа от
// HEAD и диффа вне индекса одна, рабочее дерево, а старая у них разная.
func (m Model) Select(file diff.File) Model {
	top := m.newTop()
	m = m.withFile(file).rebuild().seekNew(top)

	m.selection = selection{on: true, hunk: true}
	index := m.changeFrom(m.scroll, 1)
	if index < 0 {
		index = m.changeFrom(len(m.lines)-1, -1)
	}
	if index >= 0 {
		m = m.setCursor(index)
	}
	m.selection.anchor = m.selection.cursor
	return m.follow()
}

// Selecting говорит, что идёт выделение.
func (m Model) Selecting() bool {
	return m.selection.on
}

// ToggleLines переключает выделение хунка и строк: строки начинаются с
// курсора.
func (m Model) ToggleLines() Model {
	m.selection.hunk = !m.selection.hunk
	m.selection.anchor = m.selection.cursor
	return m
}

// Deselect снимает выделение и показывает file, держа верх экрана по новой
// стороне, как Select.
func (m Model) Deselect(file diff.File) Model {
	top := m.newTop()
	m.selection = selection{}
	return m.withFile(file).rebuild().seekNew(top)
}

// MoveCursor переводит курсор на delta изменённых строк вниз или вверх, а при
// выделении хунка на delta хунков, при выборе конфликтов на delta
// конфликтов. Экран идёт за курсором.
func (m Model) MoveCursor(delta int) Model {
	if m.selection.conflict {
		return m.moveConflict(delta)
	}

	index := m.cursorIndex()
	if index < 0 {
		return m
	}

	step := 1
	if delta < 0 {
		step = -1
	}
	for range max(delta, -delta) {
		from := index + step
		if m.selection.hunk {
			start, end := m.hunkBounds(index)
			from = end + 1
			if step < 0 {
				from = start - 1
			}
		}

		next := m.changeFrom(from, step)
		if next < 0 {
			break
		}
		if m.selection.hunk {
			start, _ := m.hunkBounds(next)
			next = m.changeFrom(start, 1)
		}
		index = next
	}

	return m.setCursor(index).follow()
}

// setCursor ставит курсор на первую изменённую строку строки экрана index.
func (m Model) setCursor(index int) Model {
	m.selection.cursor = refs(m.lines[index])[0]
	m.selection.at = m.newPositions()[index]
	return m
}

// Selected возвращает изменённые строки выделения.
func (m Model) Selected() []diff.LineRef {
	from, to := m.bounds()

	var selected []diff.LineRef
	for index := from; index <= to; index++ {
		if m.lines[index].part == 0 {
			selected = append(selected, refs(m.lines[index])...)
		}
	}
	return selected
}

// bounds это строки экрана выделения от from до to включительно, to меньше
// from без выделения. Перенесённая строка входит всеми кусками.
func (m Model) bounds() (from, to int) {
	cursor := m.cursorIndex()
	if !m.selection.on || cursor < 0 {
		return 0, -1
	}
	if m.selection.conflict {
		return m.conflictBounds()
	}
	if m.selection.hunk {
		return m.hunkBounds(cursor)
	}

	anchor := m.rowIndex(m.selection.anchor)
	if anchor < 0 {
		anchor = cursor
	}
	from, to = min(anchor, cursor), max(anchor, cursor)
	for to+1 < len(m.lines) && m.lines[to+1].part > 0 {
		to++
	}
	return from, to
}

// hunkBounds это строки экрана хунка, в котором стоит строка index: от
// разрыва до разрыва.
func (m Model) hunkBounds(index int) (from, to int) {
	from, to = index, index
	for from > 0 && !m.lines[from-1].gap() {
		from--
	}
	for to+1 < len(m.lines) && !m.lines[to+1].gap() {
		to++
	}
	return from, to
}

// follow прокручивает экран так, чтобы курсор был виден.
func (m Model) follow() Model {
	index := m.cursorIndex()
	switch {
	case index < 0:
	case index < m.scroll:
		m.scroll = index
	case index >= m.scroll+m.rows():
		m.scroll = index - m.rows() + 1
	}
	return m.clamp()
}

// cursorIndex это строка экрана курсора. Курсор мог пропасть после
// перечитывания: строку поставили в индекс. Тогда он встаёт на ближайшую
// изменённую строку ниже по новой стороне, а без неё выше: номер на старой
// стороне после правки индекса уже другой, а удалённые и добавленные строки
// пронумерованы по разным сторонам.
func (m Model) cursorIndex() int {
	if m.selection.conflict {
		block, found := m.conflictCursor()
		if !found {
			return -1
		}
		return block.from
	}
	if index := m.rowIndex(m.selection.cursor); index >= 0 {
		return index
	}
	positions := m.newPositions()
	for index, line := range m.lines {
		if line.part == 0 && len(refs(line)) > 0 && positions[index] >= m.selection.at {
			return index
		}
	}
	return m.changeFrom(len(m.lines)-1, -1)
}

// newPositions это место каждой строки экрана на новой стороне: её новый
// номер, у удалённой строки номер ближайшей строки выше плюс один, у строки
// без номеров 0.
func (m Model) newPositions() []int {
	positions := make([]int, len(m.lines))
	last := 0
	for index, line := range m.lines {
		oldNumber, newNumber := line.numbers()
		switch {
		case newNumber > 0:
			positions[index], last = newNumber, newNumber
		case oldNumber > 0:
			positions[index] = last + 1
		}
	}
	return positions
}

// newTop это место верха экрана на новой стороне, 0 под служебной строкой.
func (m Model) newTop() int {
	if m.scroll >= len(m.lines) || m.lines[m.scroll].notice != nil {
		return 0
	}
	for _, position := range m.newPositions()[m.scroll:] {
		if position > 0 {
			return position
		}
	}
	return 0
}

// seekNew ставит наверх первую строку не выше места target на новой стороне.
func (m Model) seekNew(target int) Model {
	if target == 0 {
		return m.clamp()
	}
	for index, position := range m.newPositions() {
		if position >= target {
			m.scroll = index
			break
		}
	}
	return m.clamp()
}

// rowIndex ищет первую строку экрана, в которой есть ref.
func (m Model) rowIndex(ref diff.LineRef) int {
	for index, line := range m.lines {
		if line.part != 0 {
			continue
		}
		for _, candidate := range refs(line) {
			if candidate == ref {
				return index
			}
		}
	}
	return -1
}

// changeFrom ищет от строки from в сторону step первую строку экрана с
// изменением, -1 без неё. Продолжения перенесённых строк пропускаются.
func (m Model) changeFrom(from, step int) int {
	for index := from; index >= 0 && index < len(m.lines); index += step {
		if m.lines[index].part == 0 && len(refs(m.lines[index])) > 0 {
			return index
		}
	}
	return -1
}

func (l line) gap() bool {
	return (l.split != nil && l.split.Kind == diff.RowGap) || (l.unified != nil && l.unified.Kind == diff.RowGap)
}

// refs это изменённые строки файла в строке экрана: у split их бывает две,
// удалённая слева и добавленная справа.
func refs(l line) []diff.LineRef {
	var found []diff.LineRef
	switch {
	case l.unified != nil && l.unified.Kind == diff.RowLine:
		switch l.unified.Line {
		case diff.LineRemoved:
			found = append(found, diff.LineRef{Kind: diff.LineRemoved, Number: l.unified.OldNumber, Text: l.unified.Text})
		case diff.LineAdded:
			found = append(found, diff.LineRef{Kind: diff.LineAdded, Number: l.unified.NewNumber, Text: l.unified.Text})
		}
	case l.split != nil && l.split.Kind == diff.RowLine:
		if l.split.Left.Kind == diff.LineRemoved {
			found = append(found, diff.LineRef{Kind: diff.LineRemoved, Number: l.split.Left.Number, Text: l.split.Left.Text})
		}
		if l.split.Right.Kind == diff.LineAdded {
			found = append(found, diff.LineRef{Kind: diff.LineAdded, Number: l.split.Right.Number, Text: l.split.Right.Text})
		}
	}
	return found
}

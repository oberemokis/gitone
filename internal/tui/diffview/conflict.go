package diffview

import "gitone/internal/diff"

// Выбор конфликтов (#242) это выделение, где курсор ходит по конфликтам
// файла. Что считать конфликтом, решает diff.Conflicts, то же правило, что у
// решения конфликта. Маркеры настоящего конфликта в диффе от HEAD всегда
// добавленные строки, поэтому правило получает только их: маркеры, которые
// уже лежали в HEAD, конфликтом не считаются. Курсор держится за номер строки
// начала.

// SelectConflicts включает выбор конфликтов на открытом файле, курсор на
// первом конфликте экрана или ниже. found false, если конфликтов в диффе нет.
func (m Model) SelectConflicts() (next Model, found bool) {
	next = m.onPage(func(m Model) Model {
		blocks := m.conflicts()
		if len(blocks) == 0 {
			return m
		}

		chosen := blocks[len(blocks)-1]
		for _, block := range blocks {
			if block.from >= m.scroll {
				chosen = block
				break
			}
		}
		m.selection = selection{on: true, conflict: true, cursor: diff.LineRef{Kind: diff.LineAdded, Number: chosen.Start}}
		found = true
		return m.follow()
	})
	return next, found
}

// SelectingConflicts говорит, что идёт выбор конфликтов.
func (m Model) SelectingConflicts() bool {
	m = m.focus()
	return m.selection.on && m.selection.conflict
}

// ConflictStart это номер строки рабочего дерева, с которой начинается
// конфликт под курсором.
func (m Model) ConflictStart() (line int, found bool) {
	m = m.focus()
	block, found := m.conflictCursor()
	if !m.SelectingConflicts() || !found {
		return 0, false
	}
	return block.Start, true
}

// conflictCursor это конфликт под курсором. После решения конфликта строки
// сдвигаются, и курсор встаёт на первый конфликт с тем же номером строки или
// ниже, а без него на последний.
func (m Model) conflictCursor() (screenConflict, bool) {
	blocks := m.conflicts()
	if len(blocks) == 0 {
		return screenConflict{}, false
	}
	for _, block := range blocks {
		if block.Start >= m.selection.cursor.Number {
			return block, true
		}
	}
	return blocks[len(blocks)-1], true
}

// moveConflict переводит курсор на delta конфликтов.
func (m Model) moveConflict(delta int) Model {
	blocks := m.conflicts()
	current, found := m.conflictCursor()
	if !found {
		return m
	}
	for position, block := range blocks {
		if block.Start == current.Start {
			next := blocks[max(min(position+delta, len(blocks)-1), 0)]
			m.selection.cursor = diff.LineRef{Kind: diff.LineAdded, Number: next.Start}
			break
		}
	}
	return m.follow()
}

// conflictBounds это строки экрана конфликта под курсором, от начала до
// закрывающего маркера с его перенесёнными частями.
func (m Model) conflictBounds() (from, to int) {
	block, found := m.conflictCursor()
	if !found {
		return 0, -1
	}
	to = block.to
	for to+1 < len(m.lines) && m.lines[to+1].part > 0 {
		to++
	}
	return block.from, to
}

// screenConflict это конфликт и строки экрана его маркеров начала и конца.
type screenConflict struct {
	diff.Conflict
	from, to int
}

// conflicts это конфликты диффа по его добавленным строкам. Продолжение
// перенесённой строки строкой не считается.
func (m Model) conflicts() []screenConflict {
	var added []diff.NumberedLine
	rows := map[int]int{}
	for index, l := range m.lines {
		if line, ok := addedLine(l); ok {
			added = append(added, line)
			rows[line.Number] = index
		}
	}

	var result []screenConflict
	for _, block := range diff.Conflicts(added) {
		result = append(result, screenConflict{Conflict: block, from: rows[block.Start], to: rows[block.End]})
	}
	return result
}

// addedLine это текст и номер в рабочем дереве добавленной строки экрана.
func addedLine(l line) (diff.NumberedLine, bool) {
	switch {
	case l.part != 0:
		return diff.NumberedLine{}, false
	case l.unified != nil && l.unified.Kind == diff.RowLine && l.unified.Line == diff.LineAdded:
		return diff.NumberedLine{Number: l.unified.NewNumber, Text: l.unified.Text}, true
	case l.split != nil && l.split.Kind == diff.RowLine && l.split.Right.Kind == diff.LineAdded:
		return diff.NumberedLine{Number: l.split.Right.Number, Text: l.split.Right.Text}, true
	}
	return diff.NumberedLine{}, false
}

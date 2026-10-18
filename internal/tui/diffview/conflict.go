package diffview

import (
	"strings"

	"gitone/internal/diff"
)

// Выбор конфликтов (#242) это выделение, где курсор ходит по конфликтам
// файла: блокам от строки «<<<<<<<» до строки «>>>>>>>». Маркеры в диффе от
// HEAD всегда добавленные строки, курсор держится за номер строки начала.

// SelectConflicts включает выбор конфликтов на открытом файле, курсор на
// первом конфликте экрана или ниже. found false, если маркеров в диффе нет.
func (m Model) SelectConflicts() (next Model, found bool) {
	starts := m.conflictStarts()
	if len(starts) == 0 {
		return m, false
	}

	index := starts[len(starts)-1]
	for _, start := range starts {
		if start >= m.scroll {
			index = start
			break
		}
	}
	ref, _ := conflictMark(m.lines[index])
	m.selection = selection{on: true, conflict: true, cursor: ref}
	return m.follow(), true
}

// SelectingConflicts говорит, что идёт выбор конфликтов.
func (m Model) SelectingConflicts() bool {
	return m.selection.on && m.selection.conflict
}

// ConflictStart это номер строки рабочего дерева, с которой начинается
// конфликт под курсором.
func (m Model) ConflictStart() (line int, found bool) {
	index := m.conflictCursor()
	if !m.SelectingConflicts() || index < 0 {
		return 0, false
	}
	ref, _ := conflictMark(m.lines[index])
	return ref.Number, true
}

// conflictCursor это строка экрана начала конфликта под курсором. После
// решения конфликта строки сдвигаются, и курсор встаёт на первый конфликт с
// тем же номером строки или ниже, а без него на последний.
func (m Model) conflictCursor() int {
	starts := m.conflictStarts()
	if len(starts) == 0 {
		return -1
	}
	for _, index := range starts {
		if ref, _ := conflictMark(m.lines[index]); ref.Number >= m.selection.cursor.Number {
			return index
		}
	}
	return starts[len(starts)-1]
}

// moveConflict переводит курсор на delta конфликтов.
func (m Model) moveConflict(delta int) Model {
	starts := m.conflictStarts()
	current := m.conflictCursor()
	for position, index := range starts {
		if index == current {
			next := starts[max(min(position+delta, len(starts)-1), 0)]
			m.selection.cursor, _ = conflictMark(m.lines[next])
			break
		}
	}
	return m.follow()
}

// conflictBounds это строки экрана конфликта под курсором, от начала до
// закрывающего маркера.
func (m Model) conflictBounds() (from, to int) {
	from = m.conflictCursor()
	if from < 0 {
		return 0, -1
	}
	for to = from + 1; to < len(m.lines); to++ {
		if _, mark := conflictMark(m.lines[to]); mark == theirsEnd {
			break
		}
	}
	for to+1 < len(m.lines) && m.lines[to+1].part > 0 {
		to++
	}
	return from, min(to, len(m.lines)-1)
}

// conflictStarts это строки экрана, с которых начинаются конфликты.
func (m Model) conflictStarts() []int {
	var starts []int
	for index, line := range m.lines {
		if _, mark := conflictMark(line); mark == oursStart {
			starts = append(starts, index)
		}
	}
	return starts
}

type marker int

const (
	noMarker marker = iota
	oursStart
	theirsEnd
)

// conflictMark говорит, что добавленная строка экрана это маркер начала или
// конца конфликта, и возвращает её. Продолжение перенесённой строки маркером
// не считается.
func conflictMark(l line) (diff.LineRef, marker) {
	if l.part != 0 {
		return diff.LineRef{}, noMarker
	}

	var text string
	var number int
	switch {
	case l.unified != nil && l.unified.Kind == diff.RowLine && l.unified.Line == diff.LineAdded:
		text, number = l.unified.Text, l.unified.NewNumber
	case l.split != nil && l.split.Kind == diff.RowLine && l.split.Right.Kind == diff.LineAdded:
		text, number = l.split.Right.Text, l.split.Right.Number
	default:
		return diff.LineRef{}, noMarker
	}

	ref := diff.LineRef{Kind: diff.LineAdded, Number: number}
	switch {
	case markerLine(text, "<<<<<<<"):
		return ref, oursStart
	case markerLine(text, ">>>>>>>"):
		return ref, theirsEnd
	}
	return diff.LineRef{}, noMarker
}

func markerLine(text, marker string) bool {
	rest, found := strings.CutPrefix(text, marker)
	return found && (rest == "" || rest[0] == ' ')
}

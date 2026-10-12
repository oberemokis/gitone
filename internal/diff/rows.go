package diff

type RowKind string

const (
	RowLine RowKind = "line"
	// Разрыв между хунками. Строка `@@` не показывается: скачок номеров
	// строк виден и так.
	RowGap RowKind = "gap"
	// Пометка «нет перевода строки» после строки, у которой его нет.
	RowNoNewline RowKind = "no-newline"
)

// Cell это строка файла с одной стороны split. Нулевое значение значит, что с
// этой стороны строки нет.
type Cell struct {
	Kind   LineKind
	Number int
	Text   string
	// изменённые слова пары, пусто без пары или когда подсветки нет
	Changed []Span
}

type SplitRow struct {
	Kind  RowKind
	Left  Cell
	Right Cell
}

// UnifiedRow это строка одной колонки. Номер 0 значит, что с этой стороны
// строки нет: у добавленной нет старого номера, у удалённой нового.
type UnifiedRow struct {
	Kind      RowKind
	Line      LineKind
	OldNumber int
	NewNumber int
	Text      string
	// изменённые слова пары, пусто без пары или когда подсветки нет
	Changed []Span
}

func Unified(file File) []UnifiedRow {
	var rows []UnifiedRow

	for index, hunk := range file.Hunks {
		if index > 0 {
			rows = append(rows, UnifiedRow{Kind: RowGap})
		}

		marks := wordMarks(hunk.Lines)
		oldNumber, newNumber := hunk.OldStart, hunk.NewStart
		for lineIndex, line := range hunk.Lines {
			row := UnifiedRow{Kind: RowLine, Line: line.Kind, Text: line.Text, Changed: marks[lineIndex]}
			switch line.Kind {
			case LineContext:
				row.OldNumber, row.NewNumber = oldNumber, newNumber
				oldNumber++
				newNumber++
			case LineRemoved:
				row.OldNumber = oldNumber
				oldNumber++
			case LineAdded:
				row.NewNumber = newNumber
				newNumber++
			}

			rows = append(rows, row)
			if line.NoNewline {
				rows = append(rows, UnifiedRow{Kind: RowNoNewline, Line: line.Kind})
			}
		}
	}

	return rows
}

func Split(file File) []SplitRow {
	var rows []SplitRow

	for index, hunk := range file.Hunks {
		if index > 0 {
			rows = append(rows, SplitRow{Kind: RowGap})
		}

		rows = append(rows, splitHunk(hunk)...)
	}

	return rows
}

// splitHunk ставит удалённые строки в пары с добавленными, которые идут за
// ними, по порядку: так правка строки стоит на одной высоте с обеих сторон.
// Лишние строки получают пустую половину напротив.
func splitHunk(hunk Hunk) []SplitRow {
	var rows []SplitRow
	var removedRun, addedRun []numbered
	oldNumber, newNumber := hunk.OldStart, hunk.NewStart

	flush := func() {
		for index := range max(len(removedRun), len(addedRun)) {
			var left, right numbered
			if index < len(removedRun) {
				left = removedRun[index]
			}
			if index < len(addedRun) {
				right = addedRun[index]
			}

			rows = append(rows, pair(left, right)...)
		}

		removedRun, addedRun = nil, nil
	}

	for _, line := range hunk.Lines {
		switch line.Kind {
		case LineContext:
			flush()
			rows = append(rows, pair(numbered{line, oldNumber}, numbered{line, newNumber})...)
			oldNumber++
			newNumber++
		case LineRemoved:
			removedRun = append(removedRun, numbered{line, oldNumber})
			oldNumber++
		case LineAdded:
			addedRun = append(addedRun, numbered{line, newNumber})
			newNumber++
		}
	}

	flush()
	return rows
}

// numbered это строка хунка с её номером на своей стороне. Нулевое значение
// значит, что с этой стороны строки нет.
type numbered struct {
	line   Line
	number int
}

func (n numbered) cell() Cell {
	if n.line.Kind == "" {
		return Cell{}
	}

	return Cell{Kind: n.line.Kind, Number: n.number, Text: n.line.Text}
}

// pair собирает строку split и, если у строки с какой-то стороны нет
// перевода в конце, строку-пометку под ней на той же стороне.
func pair(left, right numbered) []SplitRow {
	leftCell, rightCell := left.cell(), right.cell()
	if left.line.Kind == LineRemoved && right.line.Kind == LineAdded {
		leftCell.Changed, rightCell.Changed = wordSpans(left.line.Text, right.line.Text)
	}

	rows := []SplitRow{{Kind: RowLine, Left: leftCell, Right: rightCell}}
	if !left.line.NoNewline && !right.line.NoNewline {
		return rows
	}

	marker := SplitRow{Kind: RowNoNewline}
	if left.line.NoNewline {
		marker.Left = Cell{Kind: left.line.Kind}
	}
	if right.line.NoNewline {
		marker.Right = Cell{Kind: right.line.Kind}
	}

	return append(rows, marker)
}

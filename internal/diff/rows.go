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
	// Строка кончалась `\r\n`, а пара отличается от неё только этим: экран
	// рисует значок ␍. В обычном CRLF-файле значков нет.
	CR bool
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
	// как Cell.CR
	CR bool
}

func Unified(file File) []UnifiedRow {
	var rows []UnifiedRow

	for index, hunk := range file.Hunks {
		if index > 0 {
			rows = append(rows, UnifiedRow{Kind: RowGap})
		}

		marks := pairMarks(hunk.Lines)
		oldNumber, newNumber := hunk.OldStart, hunk.NewStart
		for lineIndex, line := range hunk.Lines {
			row := UnifiedRow{Kind: RowLine, Line: line.Kind, Text: line.Text, Changed: marks[lineIndex].changed, CR: marks[lineIndex].cr}
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

// noLine стоит в слоте на месте пустой половины.
const noLine = -1

// slot это одна строка split в индексах строк хунка: слева и справа. У строки
// контекста обе стороны одного индекса.
type slot struct {
	left  int
	right int
}

// replaced говорит, что в слоте удалённая строка стоит напротив добавленной.
func (s slot) replaced() bool {
	return s.left != noLine && s.right != noLine && s.left != s.right
}

// slots раскладывает хунк по строкам split. Удалённые строки встают в пары с
// добавленными по порядку, пока не встретится контекст: так правка строки
// стоит на одной высоте с обеих сторон. Лишние строки получают пустую половину
// напротив. Правило пар записано только здесь: по слотам split строит строки,
// а unified ставит подсветку слов (pairMarks), и два вида не расходятся.
func slots(lines []Line) []slot {
	var result []slot
	var removedRun, addedRun []int

	flush := func() {
		for index := range max(len(removedRun), len(addedRun)) {
			result = append(result, slot{runAt(removedRun, index), runAt(addedRun, index)})
		}

		removedRun, addedRun = nil, nil
	}

	for index, line := range lines {
		switch line.Kind {
		case LineContext:
			flush()
			result = append(result, slot{index, index})
		case LineRemoved:
			removedRun = append(removedRun, index)
		case LineAdded:
			addedRun = append(addedRun, index)
		}
	}

	flush()
	return result
}

// runAt это индекс строки блока на позиции index, а у короткого блока noLine.
func runAt(run []int, index int) int {
	if index < len(run) {
		return run[index]
	}

	return noLine
}

// splitHunk строит строки split по слотам. Номер у каждой стороны растёт с её
// строками: слева контекст и удалённые, справа контекст и добавленные.
func splitHunk(hunk Hunk) []SplitRow {
	var rows []SplitRow
	oldNumber, newNumber := hunk.OldStart, hunk.NewStart

	for _, row := range slots(hunk.Lines) {
		var left, right numbered
		if row.left != noLine {
			left = numbered{hunk.Lines[row.left], oldNumber}
			oldNumber++
		}
		if row.right != noLine {
			right = numbered{hunk.Lines[row.right], newNumber}
			newNumber++
		}

		rows = append(rows, pair(left, right)...)
	}

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
		leftMark, rightMark := markPair(left.line, right.line)
		leftCell.Changed, leftCell.CR = leftMark.changed, leftMark.cr
		rightCell.Changed, rightCell.CR = rightMark.changed, rightMark.cr
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

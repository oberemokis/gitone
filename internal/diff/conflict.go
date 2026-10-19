package diff

import "strings"

// ConflictSide это сторона, которую берёт решение конфликта (#242): наша
// (до =======), их (после) или обе подряд, наша первой.
type ConflictSide int

const (
	SideOurs ConflictSide = iota
	SideTheirs
	SideBoth
)

// Знаки маркеров конфликта git. Маркер это строка из одного знака, повторённого
// не меньше семи раз (conflict-marker-size меняет длину), дальше конец строки,
// пробел с меткой стороны или `\r` у файла с CRLF: «<<<<<<< HEAD».
const (
	oursMarker   = '<'
	baseMarker   = '|'
	middleMarker = '='
	theirsMarker = '>'

	minMarkerSize = 7
)

// NumberedLine это строка текста и её номер (с 1).
type NumberedLine struct {
	Number int
	Text   string
}

// Conflict это закрытый блок конфликта: номера строк (с 1) маркера начала,
// конца нашей стороны (основа diff3 или =======), ======= и маркера конца.
type Conflict struct {
	Start, OursEnd, Middle, End int
}

// Conflicts находит закрытые блоки конфликтов среди строк, идущих по порядку
// номеров. Блок это маркер начала, необязательная основа, ======= и маркер
// конца той же длины; незакрытый блок и блок без ======= конфликтом не
// считаются, новое начало до конца обрывает прежний блок. Строки между
// маркерами можно опускать: блок строится по одним маркерам. Это единственное
// правило «что такое конфликт» для решения и для экрана.
func Conflicts(lines []NumberedLine) []Conflict {
	var conflicts []Conflict
	for index := 0; index < len(lines); index++ {
		if block, end, ok := conflictAt(lines, index); ok {
			conflicts = append(conflicts, block)
			index = end
		}
	}
	return conflicts
}

// ConflictCount это число конфликтов в тексте файла.
func ConflictCount(text string) int {
	return len(Conflicts(textLines(text)))
}

// ConflictLabels возвращает метки сторон конфликта, который начинается
// строкой start (с 1): «HEAD» и «feature». found false, если с этой строки
// конфликт не начинается или он не закрыт.
func ConflictLabels(text string, start int) (ours, theirs string, found bool) {
	lines := textLines(text)
	block, _, ok := conflictAt(lines, start-1)
	if !ok {
		return "", "", false
	}
	return label(lines[block.Start-1].Text), label(lines[block.End-1].Text), true
}

// ResolveConflict заменяет конфликт, который начинается строкой start (с 1),
// на выбранную сторону. Основа из стиля diff3 (между ||||||| и =======)
// выпадает при любой стороне. found false, если с этой строки конфликт не
// начинается: текст тогда возвращается как есть.
func ResolveConflict(text string, start int, side ConflictSide) (resolved string, found bool) {
	lines := strings.Split(text, "\n")
	block, _, ok := conflictAt(textLines(text), start-1)
	if !ok {
		return text, false
	}

	ours := lines[block.Start : block.OursEnd-1]
	theirs := lines[block.Middle : block.End-1]
	var chosen []string
	switch side {
	case SideOurs:
		chosen = ours
	case SideTheirs:
		chosen = theirs
	default:
		chosen = append(append([]string{}, ours...), theirs...)
	}

	result := append(append(append([]string{}, lines[:block.Start-1]...), chosen...), lines[block.End:]...)
	return strings.Join(result, "\n"), true
}

// textLines это строки текста с номерами.
func textLines(text string) []NumberedLine {
	lines := strings.Split(text, "\n")
	result := make([]NumberedLine, len(lines))
	for index, line := range lines {
		result[index] = NumberedLine{Number: index + 1, Text: line}
	}
	return result
}

// conflictAt разбирает блок, который начинается строкой lines[start]. end
// это индекс маркера конца в lines.
func conflictAt(lines []NumberedLine, start int) (block Conflict, end int, ok bool) {
	if start < 0 || start >= len(lines) {
		return Conflict{}, 0, false
	}
	size := markerSize(lines[start].Text, oursMarker)
	if size == 0 {
		return Conflict{}, 0, false
	}

	block = Conflict{Start: lines[start].Number}
	for index := start + 1; index < len(lines); index++ {
		switch line := lines[index]; {
		case markerSize(line.Text, oursMarker) > 0:
			return Conflict{}, 0, false
		case markerSize(line.Text, baseMarker) == size && block.Middle == 0 && block.OursEnd == 0:
			block.OursEnd = line.Number
		case markerSize(line.Text, middleMarker) == size && block.Middle == 0:
			block.Middle = line.Number
			if block.OursEnd == 0 {
				block.OursEnd = line.Number
			}
		case markerSize(line.Text, theirsMarker) == size && block.Middle > 0:
			block.End = line.Number
			return block, index, true
		}
	}
	return Conflict{}, 0, false
}

// markerSize это длина маркера из знака char в начале строки, или 0, если
// строка не маркер.
func markerSize(line string, char byte) int {
	size := 0
	for size < len(line) && line[size] == char {
		size++
	}
	if size < minMarkerSize {
		return 0
	}
	if rest := line[size:]; rest == "" || rest[0] == ' ' || rest == "\r" {
		return size
	}
	return 0
}

// label это метка стороны после маркера.
func label(line string) string {
	return strings.TrimSpace(strings.TrimLeft(line, "<>"))
}

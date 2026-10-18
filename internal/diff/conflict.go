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

// Маркеры конфликта git: строка начинается с семи знаков. Метка стороны идёт
// после пробела: «<<<<<<< HEAD», «>>>>>>> feature».
const (
	oursMarker   = "<<<<<<<"
	baseMarker   = "|||||||"
	middleMarker = "======="
	theirsMarker = ">>>>>>>"
)

// HasConflicts говорит, что в тексте файла остался маркер начала конфликта.
func HasConflicts(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if isMarker(line, oursMarker) {
			return true
		}
	}
	return false
}

// ConflictLabels возвращает метки сторон конфликта, который начинается
// строкой start (с 1): «HEAD» и «feature». found false, если с этой строки
// конфликт не начинается или он не закрыт.
func ConflictLabels(text string, start int) (ours, theirs string, found bool) {
	lines := strings.Split(text, "\n")
	block, ok := conflictAt(lines, start-1)
	if !ok {
		return "", "", false
	}
	return label(lines[block.start]), label(lines[block.end]), true
}

// ResolveConflict заменяет конфликт, который начинается строкой start (с 1),
// на выбранную сторону. Основа из стиля diff3 (между ||||||| и =======)
// выпадает при любой стороне. found false, если с этой строки конфликт не
// начинается: текст тогда возвращается как есть.
func ResolveConflict(text string, start int, side ConflictSide) (resolved string, found bool) {
	lines := strings.Split(text, "\n")
	block, ok := conflictAt(lines, start-1)
	if !ok {
		return text, false
	}

	ours := lines[block.start+1 : block.oursEnd]
	theirs := lines[block.middle+1 : block.end]
	var chosen []string
	switch side {
	case SideOurs:
		chosen = ours
	case SideTheirs:
		chosen = theirs
	default:
		chosen = append(append([]string{}, ours...), theirs...)
	}

	result := append(append(append([]string{}, lines[:block.start]...), chosen...), lines[block.end+1:]...)
	return strings.Join(result, "\n"), true
}

// conflictBlock это строки маркеров одного конфликта: начало, конец нашей
// стороны (основа diff3 или =======), ======= и конец.
type conflictBlock struct {
	start, oursEnd, middle, end int
}

func conflictAt(lines []string, start int) (conflictBlock, bool) {
	if start < 0 || start >= len(lines) || !isMarker(lines[start], oursMarker) {
		return conflictBlock{}, false
	}

	block := conflictBlock{start: start, oursEnd: -1, middle: -1}
	for index := start + 1; index < len(lines); index++ {
		switch line := lines[index]; {
		case isMarker(line, baseMarker) && block.middle < 0 && block.oursEnd < 0:
			block.oursEnd = index
		case isMarker(line, middleMarker) && block.middle < 0:
			block.middle = index
			if block.oursEnd < 0 {
				block.oursEnd = index
			}
		case isMarker(line, theirsMarker) && block.middle >= 0:
			block.end = index
			return block, true
		case isMarker(line, oursMarker):
			return conflictBlock{}, false
		}
	}
	return conflictBlock{}, false
}

// isMarker говорит, что строка это маркер: семь знаков, дальше конец строки,
// пробел или `\r` у файла с CRLF.
func isMarker(line, marker string) bool {
	rest, found := strings.CutPrefix(line, marker)
	return found && (rest == "" || rest[0] == ' ' || rest == "\r")
}

// label это метка стороны после маркера.
func label(line string) string {
	return strings.TrimSpace(line[len(oursMarker):])
}

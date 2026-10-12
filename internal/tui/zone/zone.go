// Package zone помечает места клика в рисунке экрана (#119). Кликабельный
// кусок обёрнут невидимыми метками: OSC-последовательностями с номером,
// которого не знают терминалы. Lip Gloss и x/ansi считают их ширину нулевой и
// сохраняют при обрезке, отступах и склейке, поэтому метки доходят до готового
// рисунка на тех же местах, что и текст. Scan находит их и снимает.
package zone

import (
	"image"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Метка начала несёт цель, метка конца пустая. Цель не содержит ESC и BEL.
const (
	markOpen  = "\x1b]4747;"
	markClose = "\a"
)

// Mark оборачивает text метками места target.
func Mark(target, text string) string {
	return markOpen + target + markClose + text + markOpen + markClose
}

// rowPrefix начинает цель строки списка.
const rowPrefix = "row:"

// Row это цель строки списка номер index. Строки ставят и панели, и окна:
// чья это строка, решает корень, он знает, что сейчас открыто.
func Row(index int) string {
	return rowPrefix + strconv.Itoa(index)
}

// RowIndex это номер строки из цели Row. found false у любой другой цели.
func RowIndex(target string) (index int, found bool) {
	number, cut := strings.CutPrefix(target, rowPrefix)
	if !cut {
		return 0, false
	}

	index, err := strconv.Atoi(number)
	return index, err == nil
}

// Zone это место клика: цель и прямоугольник в клетках рисунка.
type Zone struct {
	Target string
	Area   image.Rectangle
}

// Scan снимает метки с рисунка и возвращает рисунок без них и места клика.
// Место, не закрытое до конца строки, кончается на её краю и продолжается с
// начала следующей.
func Scan(content string) (string, []Zone) {
	lines := strings.Split(content, "\n")

	var zones []Zone
	target, start := "", 0
	for y, line := range lines {
		var clean strings.Builder
		column := 0
		for {
			at := strings.Index(line, markOpen)
			end := -1
			if at >= 0 {
				end = strings.Index(line[at+len(markOpen):], markClose)
			}
			if end < 0 {
				clean.WriteString(line)
				column += ansi.StringWidth(line)
				break
			}

			clean.WriteString(line[:at])
			column += ansi.StringWidth(line[:at])
			found := line[at+len(markOpen) : at+len(markOpen)+end]
			line = line[at+len(markOpen)+end+len(markClose):]

			if target != "" && column > start {
				zones = append(zones, Zone{Target: target, Area: image.Rect(start, y, column, y+1)})
			}
			target, start = found, column
		}

		if target != "" && column > start {
			zones = append(zones, Zone{Target: target, Area: image.Rect(start, y, column, y+1)})
		}
		start = 0
		lines[y] = clean.String()
	}

	return strings.Join(lines, "\n"), zones
}

// At это цель места под точкой x, y или пустая строка. Места проверяются по
// порядку, первое выигрывает: так окно закрывает собой то, что под ним.
func At(zones []Zone, x, y int) string {
	point := image.Pt(x, y)
	for _, zone := range zones {
		if point.In(zone.Area) {
			return zone.Target
		}
	}

	return ""
}

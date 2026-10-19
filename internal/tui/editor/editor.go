// Package editor это многострочное поле ввода: текст, курсор, перенос строк и
// прокрутка. Это поле, поэтому оно получает нажатия напрямую, а не действия:
// стрелки, home, end, delete, backspace и enter зашиты, остальное набирается.
// Рисует поле корень, как окно picker: здесь только состояние и ввод.
package editor

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/rivo/uniseg"
)

// Model это текст и курсор. Курсор это номер руны, перед которой он стоит;
// нулевое значение это пустое поле.
type Model struct {
	text   []rune
	cursor int
	// want это колонка, с которой курсор начал движение вверх или вниз: по
	// короткой строке он проходит и возвращается в неё. wanted false значит,
	// что колонку надо взять с курсора.
	want   int
	wanted bool
	// top это первая показанная строка поля
	top           int
	width, height int
}

// row это строка поля на экране, руны text[start:end]. last значит, что строка
// кончает строку текста (за ней перевод строки или конец текста): только там
// курсор стоит после последней руны.
type row struct {
	start, end int
	last       bool
}

// Resize задаёт размер поля в клетках. Курсор остаётся на экране.
func (m Model) Resize(width, height int) Model {
	m.width, m.height = width, max(height, 1)
	return m.follow()
}

// Text это набранный текст.
func (m Model) Text() string {
	return string(m.text)
}

// HandleKey разбирает нажатие: набранный текст вставляется в позицию курсора,
// именованные клавиши правят и двигают его. Поле идёт за курсором.
func (m Model) HandleKey(key tea.KeyPressMsg) Model {
	return m.handle(key).follow()
}

func (m Model) handle(key tea.KeyPressMsg) Model {
	if key.Text != "" {
		return m.insert(key.Text)
	}

	switch key.String() {
	case "enter":
		return m.insert("\n")
	case "up":
		return m.moveRows(-1)
	case "down":
		return m.moveRows(1)
	case "left":
		m.cursor = m.previous(m.cursor)
	case "right":
		m.cursor = m.next(m.cursor)
	case "home":
		m.cursor = m.lineStart()
	case "end":
		m.cursor = m.lineEnd()
	case "backspace":
		previous := m.previous(m.cursor)
		m.text = slices.Concat(m.text[:previous], m.text[m.cursor:])
		m.cursor = previous
	case "delete":
		m.text = slices.Concat(m.text[:m.cursor], m.text[m.next(m.cursor):])
	default:
		return m
	}

	m.wanted = false
	return m
}

// Type вставляет text в позицию курсора, как набранные клавиши: так приходит
// вставка из буфера. Конец строки любого вида становится переводом строки,
// таб четырьмя пробелами, остальные управляющие символы отбрасываются: на
// экран они ушли бы кодами терминала. Поле идёт за курсором.
func (m Model) Type(text string) Model {
	return m.insert(text).follow()
}

func (m Model) insert(text string) Model {
	text = strings.NewReplacer("\r\n", "\n", "\r", "\n", "\t", "    ").Replace(text)
	inserted := slices.DeleteFunc([]rune(text), func(char rune) bool {
		return char != '\n' && unicode.IsControl(char)
	})

	m.text = slices.Concat(m.text[:m.cursor], inserted, m.text[m.cursor:])
	m.cursor += len(inserted)
	m.wanted = false
	return m
}

// Scroll сдвигает видимую часть на delta строк, не дальше краёв текста. Курсор
// остаётся на экране: если сдвиг вынес его за край, он встаёт на ближайшую
// видимую строку.
func (m Model) Scroll(delta int) Model {
	rows := m.rows()
	m.top = max(min(m.top+delta, len(rows)-m.height), 0)

	line := cursorRow(rows, m.cursor)
	target := max(min(line, m.top+m.height-1), m.top)
	if target == line {
		return m
	}

	return m.moveRows(target - line)
}

// Rows это видимые строки поля, ровно height штук: короткий текст добит
// пустыми.
func (m Model) Rows() []string {
	rows := m.rows()
	shown := make([]string, m.height)
	for index := range shown {
		if line := m.top + index; line < len(rows) {
			shown[index] = string(m.text[rows[line].start:rows[line].end])
		}
	}

	return shown
}

// Cursor это номер строки из Rows и номер руны в ней, перед которой стоит
// курсор.
func (m Model) Cursor() (line, offset int) {
	rows := m.rows()
	line = cursorRow(rows, m.cursor)

	return line - m.top, m.cursor - rows[line].start
}

// follow двигает видимую часть так, чтобы курсор остался на экране, и не
// оставляет под концом текста пустых строк, пока он помещается.
func (m Model) follow() Model {
	rows := m.rows()
	line := cursorRow(rows, m.cursor)

	m.top = max(min(m.top, line), line-m.height+1)
	m.top = max(min(m.top, len(rows)-m.height), 0)
	return m
}

// rows раскладывает текст по строкам поля: каждая строка текста идёт одной
// строкой поля или несколькими, если не помещается в ширину.
func (m Model) rows() []row {
	var rows []row
	start := 0
	for {
		end := len(m.text)
		if newline := slices.Index(m.text[start:], '\n'); newline >= 0 {
			end = start + newline
		}

		rows = append(rows, m.wrap(start, end)...)
		if end == len(m.text) {
			return rows
		}
		start = end + 1
	}
}

// wrap делит строку текста text[start:end] на строки поля.
func (m Model) wrap(start, end int) []row {
	var rows []row
	for {
		next := m.fit(start, end)
		if next >= end {
			return append(rows, row{start: start, end: end, last: true})
		}

		rows = append(rows, row{start: start, end: next})
		start = next
	}
}

// fit это конец строки поля, которая начинается в start: графемы, пока они
// помещаются в ширину, на краю перенос по последнему пробелу, а слово длиннее
// строки рвётся по ширине. Последняя клетка остаётся курсору в конце строки,
// каждая строка берёт хотя бы одну графему.
func (m Model) fit(start, end int) int {
	limit := max(m.width-1, 1)
	cells, space := 0, -1
	for _, char := range m.graphemes(start, end) {
		if cells+char.width > limit && char.start > start {
			if space >= 0 {
				return space + 1
			}
			return char.start
		}

		cells += char.width
		if m.text[char.start] == ' ' {
			space = char.start
		}
	}

	return end
}

// grapheme это графемный кластер text[start:end] и его ширина в клетках.
// Терминал меряет кластер целиком: «⚠️» это две руны и две клетки, а по
// рунам вышла одна.
type grapheme struct {
	start, end, width int
}

// graphemes делит text[from:to] на кластеры. from должен быть границей
// кластера: начало строки или место, куда встал курсор.
func (m Model) graphemes(from, to int) []grapheme {
	var chars []grapheme
	state, rest := -1, string(m.text[from:to])
	for rest != "" {
		var cluster string
		var width int
		cluster, rest, width, state = uniseg.FirstGraphemeClusterInString(rest, state)
		end := from + utf8.RuneCountInString(cluster)
		chars = append(chars, grapheme{start: from, end: end, width: width})
		from = end
	}
	return chars
}

// previous это начало кластера перед pos, next конец кластера после него:
// курсор, стирание и delete не рвут кластер.
func (m Model) previous(pos int) int {
	start := 0
	for _, char := range m.graphemes(0, len(m.text)) {
		if char.start >= pos {
			break
		}
		start = char.start
	}
	return start
}

func (m Model) next(pos int) int {
	for _, char := range m.graphemes(0, len(m.text)) {
		if char.end > pos {
			return char.end
		}
	}
	return len(m.text)
}

// cursorRow это строка, в которой стоит курсор pos. Строки идут встык, и
// курсор на стыке принадлежит нижней: ищется последняя, что начинается не
// правее.
func cursorRow(rows []row, pos int) int {
	for index := len(rows) - 1; index > 0; index-- {
		if rows[index].start <= pos {
			return index
		}
	}

	return 0
}

// moveRows сдвигает курсор на delta строк поля, не выходя за текст.
func (m Model) moveRows(delta int) Model {
	rows := m.rows()
	current := cursorRow(rows, m.cursor)
	target := current + delta
	if target < 0 || target >= len(rows) {
		return m
	}

	if !m.wanted {
		m.want, m.wanted = m.cells(rows[current].start, m.cursor), true
	}

	m.cursor = m.at(rows[target], m.want)
	return m
}

// at это позиция курсора в строке line на колонке col: перед графемой, которая
// начинается не правее. Курсор на стыке принадлежит нижней строке, поэтому в
// строке, которая не кончает строку текста, он остаётся перед последней
// графемой.
func (m Model) at(line row, col int) int {
	pos, last, cells := line.start, line.start, 0
	for _, char := range m.graphemes(line.start, line.end) {
		if cells+char.width > col {
			break
		}
		cells += char.width
		pos, last = char.end, char.start
	}

	if !line.last && pos == line.end {
		pos = last
	}
	return pos
}

// cells это ширина text[from:to] в клетках.
func (m Model) cells(from, to int) int {
	var width int
	for _, char := range m.graphemes(from, to) {
		width += char.width
	}

	return width
}

// lineStart это начало строки текста, в которой стоит курсор: после
// предыдущего перевода строки.
func (m Model) lineStart() int {
	for pos := m.cursor; pos > 0; pos-- {
		if m.text[pos-1] == '\n' {
			return pos
		}
	}

	return 0
}

// lineEnd это конец строки текста, в которой стоит курсор: перед переводом
// строки.
func (m Model) lineEnd() int {
	if end := slices.Index(m.text[m.cursor:], '\n'); end >= 0 {
		return m.cursor + end
	}

	return len(m.text)
}

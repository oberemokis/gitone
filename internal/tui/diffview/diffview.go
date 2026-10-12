// Package diffview рисует один файл диффа в виде split или unified и
// прокручивает его. Клавиш и других файлов он не знает: корень вызывает его
// методы по действиям и сам решает, какой файл открыт.
package diffview

import (
	"strconv"
	"strings"

	"gitone/internal/diff"
	"gitone/internal/tui/theme"
)

type Mode string

const (
	ModeAuto    Mode = "auto"
	ModeSplit   Mode = "split"
	ModeUnified Mode = "unified"
)

// С этой ширины auto выбирает split: на меньшей две колонки слишком узки для
// кода. Так же делает critique.
const autoSplitMinWidth = 120

type Model struct {
	file   diff.File
	colors theme.Theme
	mode   Mode
	width  int
	height int
	scroll int
	// Строки пересчитываются при смене файла, режима или размера, а не в
	// View: Bubble Tea вызывает View на каждое сообщение.
	lines       []line
	numberWidth int
}

func New(colors theme.Theme) Model {
	return Model{colors: colors, mode: ModeAuto}
}

func (m Model) SetFile(file diff.File) Model {
	m.file = file
	m.scroll = 0
	return m.rebuild()
}

// Reload ставит на место файла его новую версию. Прокрутка остаётся той же,
// обрезанной по новой длине, а SetFile её сбросил бы.
func (m Model) Reload(file diff.File) Model {
	m.file = file
	return m.rebuild()
}

// SetSize может сменить вид в режиме auto, поэтому держит на месте верхнюю
// строку файла, как ToggleMode.
func (m Model) SetSize(width, height int) Model {
	top := m.topPosition()
	m.width, m.height = width, height
	return m.rebuild().seek(top)
}

// ToggleMode переключает от того вида, что сейчас на экране, а не от auto, и
// держит выбор до конца сеанса: смена ширины после этого вид не меняет.
func (m Model) ToggleMode() Model {
	top := m.topPosition()
	if m.effectiveMode() == ModeSplit {
		m.mode = ModeUnified
	} else {
		m.mode = ModeSplit
	}

	return m.rebuild().seek(top)
}

// position это номер строки файла наверху экрана: новый, а если его нет
// (удалённая строка), старый. В split строк меньше, чем в unified, и одна и та
// же позиция прокрутки показывает в них разные места, а номер строки нет.
type position struct {
	number int
	old    bool
}

func (m Model) topPosition() position {
	if m.scroll >= len(m.lines) {
		return position{}
	}

	oldNumber, newNumber := m.lines[m.scroll].numbers()
	if newNumber > 0 {
		return position{number: newNumber}
	}

	return position{number: oldNumber, old: true}
}

// seek ставит наверх первую строку с номером не меньше искомого: номера на
// каждой стороне растут, так что это то же место или ближайшее после него.
func (m Model) seek(target position) Model {
	if target.number == 0 {
		return m.clamp()
	}

	for index, line := range m.lines {
		oldNumber, number := line.numbers()
		if target.old {
			number = oldNumber
		}
		if number >= target.number {
			m.scroll = index
			break
		}
	}

	return m.clamp()
}

func (m Model) ScrollBy(delta int) Model {
	m.scroll += delta
	return m.clamp()
}

func (m Model) Top() Model {
	m.scroll = 0
	return m
}

func (m Model) Bottom() Model {
	m.scroll = len(m.lines)
	return m.clamp()
}

// clamp держит прокрутку в пределах: последняя строка файла не уходит выше
// нижнего края экрана, первая не уходит ниже верхнего.
func (m Model) clamp() Model {
	m.scroll = max(min(m.scroll, len(m.lines)-m.height), 0)
	return m
}

func (m Model) View() string {
	end := min(m.scroll+m.height, len(m.lines))

	rendered := make([]string, 0, end-m.scroll)
	for _, line := range m.lines[m.scroll:end] {
		rendered = append(rendered, m.render(line))
	}

	return strings.Join(rendered, "\n")
}

func (m Model) effectiveMode() Mode {
	if m.mode != ModeAuto {
		return m.mode
	}
	if m.width >= autoSplitMinWidth {
		return ModeSplit
	}

	return ModeUnified
}

func (m Model) rebuild() Model {
	m.lines = notices(m.file)
	largest := 0

	if m.effectiveMode() == ModeSplit {
		for _, row := range diff.Split(m.file) {
			m.lines = append(m.lines, line{split: &row})
			largest = max(largest, row.Left.Number, row.Right.Number)
		}
	} else {
		for _, row := range diff.Unified(m.file) {
			m.lines = append(m.lines, line{unified: &row})
			largest = max(largest, row.OldNumber, row.NewNumber)
		}
	}

	if len(m.lines) == 0 {
		m.lines = []line{{notice: &notice{text: "No content changes"}}}
	}

	m.numberWidth = len(strconv.Itoa(largest))
	return m.clamp()
}

// notices это служебные строки над строками файла. Нераспознанный блок git
// показывается целиком как есть: лучше сырой текст, чем пропавший файл.
func notices(file diff.File) []line {
	var lines []line

	if file.Status == diff.StatusRaw {
		for _, text := range strings.Split(file.Raw, "\n") {
			lines = append(lines, line{notice: &notice{text: text, plain: true}})
		}
		return lines
	}

	if file.Binary {
		lines = append(lines, line{notice: &notice{text: "Binary or large file"}})
	}
	if file.OldMode != "" && file.NewMode != "" && file.OldMode != file.NewMode {
		text := "mode " + shortMode(file.OldMode) + " → " + shortMode(file.NewMode)
		lines = append(lines, line{notice: &notice{text: text}})
	}

	return lines
}

// shortMode показывает права обычного файла тремя цифрами: «100644» как
// «644». Симлинк и подмодуль остаются полным кодом, он у них другой.
func shortMode(mode string) string {
	return strings.TrimPrefix(mode, "100")
}

// line это строка экрана до отрисовки. Заполнено ровно одно поле.
type line struct {
	split   *diff.SplitRow
	unified *diff.UnifiedRow
	notice  *notice
}

// notice это служебная строка вместо строки файла. plain значит текст как
// есть, без серого цвета служебных надписей.
type notice struct {
	text  string
	plain bool
}

func (l line) numbers() (int, int) {
	switch {
	case l.split != nil:
		return l.split.Left.Number, l.split.Right.Number
	case l.unified != nil:
		return l.unified.OldNumber, l.unified.NewNumber
	}

	return 0, 0
}

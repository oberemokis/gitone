// Package diffview рисует один файл диффа в виде split или unified и
// прокручивает его. Клавиш и других файлов он не знает: корень вызывает его
// методы по действиям и сам решает, какой файл открыт.
package diffview

import (
	"cmp"
	"strconv"
	"strings"

	"gitone/internal/diff"
	"gitone/internal/i18n"
	"gitone/internal/syntax"
	"gitone/internal/tui/theme"
)

type Mode string

const (
	ModeAuto    Mode = "auto"
	ModeSplit   Mode = "split"
	ModeUnified Mode = "unified"
)

type Model struct {
	file diff.File
	// подсветка синтаксиса старой и новой стороны по номеру строки, считается
	// при открытии файла (highlight.go)
	oldCode, newCode code
	colors           theme.Theme
	mode             Mode
	// с этой ширины auto выбирает split, по умолчанию 120 (diff.autoSplitMinWidth)
	autoSplitMinWidth int
	width             int
	height            int
	scroll            int
	// Строки пересчитываются при смене файла, режима или размера, а не в
	// View: Bubble Tea вызывает View на каждое сообщение.
	lines       []line
	numberWidth int
	// колонка номеров скрыта: флаг меняет только рисунок, строки те же
	hideNumbers bool
	// служебные строки на языке интерфейса
	text i18n.Dictionary
	// wrap переносит длинные строки, без него код сдвигается вбок на shift
	// колонок; longest это ширина самого длинного кода файла (wrap.go)
	wrap    bool
	shift   int
	longest int
	// share доля левой половины split, 0 значит пополам (split.go)
	share float64
	// versionKey ключ версий, по которым сейчас раскрашен файл, пусто при
	// подсветке по хункам; versions подсветка по версиям файла целиком
	// (versions.go)
	versionKey string
	versions   map[string]sides
}

func New(colors theme.Theme, mode Mode, autoSplitMinWidth int) Model {
	return Model{colors: colors, mode: mode, autoSplitMinWidth: autoSplitMinWidth}
}

// WithText задаёт язык служебных строк. Без него они английские.
func (m Model) WithText(text i18n.Dictionary) Model {
	m.text = text
	return m
}

// WithoutNumbers прячет колонку номеров (ui.lineNumbers в config.json).
func (m Model) WithoutNumbers(hide bool) Model {
	m.hideNumbers = hide
	return m
}

func (m Model) SetFile(file diff.File) Model {
	m.scroll, m.shift = 0, 0
	return m.withFile(file).rebuild()
}

// Reload ставит на место файла его новую версию. Прокрутка остаётся той же,
// обрезанной по новой длине, а SetFile её сбросил бы.
func (m Model) Reload(file diff.File) Model {
	return m.withFile(file).rebuild()
}

// withFile ставит файл и размечает его код. Язык выбирается по пути: у
// удалённого файла путь старый, у переименованного новый.
func (m Model) withFile(file diff.File) Model {
	m.file = file
	m.oldCode, m.newCode = highlight(syntax.Detect(cmp.Or(file.NewPath, file.OldPath)), file)
	m.versionKey = ""
	return m
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
	m.scroll = max(min(m.scroll, len(m.lines)-m.rows()), 0)
	return m
}

// rows это строки экрана под код: полоса прокрутки забирает нижнюю.
func (m Model) rows() int {
	if m.hasBar() {
		return m.height - 1
	}
	return m.height
}

func (m Model) View() string {
	end := min(m.scroll+m.rows(), len(m.lines))

	rendered := make([]string, 0, m.height)
	for _, line := range m.lines[m.scroll:end] {
		rendered = append(rendered, m.render(line))
	}

	if m.hasBar() {
		// полоса стоит внизу панели и под коротким файлом
		for len(rendered) < m.rows() {
			rendered = append(rendered, "")
		}
		rendered = append(rendered, m.scrollbar())
	}

	return strings.Join(rendered, "\n")
}

func (m Model) effectiveMode() Mode {
	if m.mode != ModeAuto {
		return m.mode
	}
	if m.width >= m.autoSplitMinWidth {
		return ModeSplit
	}

	return ModeUnified
}

// rebuild пересчитывает строки экрана. Ширина номеров нужна раньше строк:
// по ней считается ширина кода, а по ней перенос.
func (m Model) rebuild() Model {
	var rows []line
	largest := 0

	if m.effectiveMode() == ModeSplit {
		for _, row := range diff.Split(m.file) {
			rows = append(rows, line{split: &row})
			largest = max(largest, row.Left.Number, row.Right.Number)
		}
	} else {
		for _, row := range diff.Unified(m.file) {
			rows = append(rows, line{unified: &row})
			largest = max(largest, row.OldNumber, row.NewNumber)
		}
	}
	m.numberWidth = len(strconv.Itoa(largest))

	m.longest = 0
	m.lines = nil
	for _, row := range append(m.notices(), rows...) {
		m.longest = max(m.longest, m.extent(row))
		m.lines = append(m.lines, m.parts(row)...)
	}

	if len(m.lines) == 0 {
		m.lines = []line{{notice: &notice{text: m.text.Text("diff.noContent", nil)}}}
	}

	m.shift = min(m.shift, m.maxShift())
	return m.clamp()
}

// notices это служебные строки над строками файла. Нераспознанный блок git
// показывается целиком как есть: лучше сырой текст, чем пропавший файл.
func (m Model) notices() []line {
	var lines []line

	file := m.file
	if file.Status == diff.StatusRaw {
		for _, text := range strings.Split(file.Raw, "\n") {
			lines = append(lines, line{notice: &notice{text: text, plain: true}})
		}
		return lines
	}

	if file.Binary {
		lines = append(lines, line{notice: &notice{text: m.text.Text("diff.binary", nil)}})
	}
	if file.OldMode != "" && file.NewMode != "" && file.OldMode != file.NewMode {
		text := m.text.Text("diff.mode", i18n.Args{"old": shortMode(file.OldMode), "new": shortMode(file.NewMode)})
		lines = append(lines, line{notice: &notice{text: text}})
	}

	return lines
}

// shortMode показывает права обычного файла тремя цифрами: «100644» как
// «644». Симлинк и подмодуль остаются полным кодом, он у них другой.
func shortMode(mode string) string {
	return strings.TrimPrefix(mode, "100")
}

// line это строка экрана до отрисовки. Заполнено ровно одно из split, unified
// и notice. chunk это кусок строки файла при переносе (wrap.go).
type line struct {
	split   *diff.SplitRow
	unified *diff.UnifiedRow
	notice  *notice
	chunk
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

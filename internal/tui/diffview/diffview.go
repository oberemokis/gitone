// Package diffview рисует файл диффа в виде split или unified и прокручивает
// его, а в ленте (stream.go) все файлы подряд. Клавиш он не знает: корень
// вызывает его методы по действиям и сам решает, какой файл открыт, в ленте
// его выбирает прокрутка.
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

// Mode это вид диффа. Набор режимов проверяет и config при чтении файла
// (decode, ошибка config.badMode): импортировать этот пакет он не может, и
// новый режим правится в обоих местах и в тексте ошибки словаря.
type Mode string

const (
	ModeAuto    Mode = "auto"
	ModeSplit   Mode = "split"
	ModeUnified Mode = "unified"
)

type Model struct {
	settings
	file diff.File
	// подсветка синтаксиса старой и новой стороны по номеру строки, считается
	// при открытии файла (highlight.go), а у страницы ленты, когда она видна;
	// colored значит, что подсветка посчитана
	oldCode, newCode code
	colored          bool
	scroll           int
	// Строки пересчитываются при смене файла, режима или размера, а не в
	// View: Bubble Tea вызывает View на каждое сообщение.
	lines       []line
	numberWidth int
	// longest это ширина самого длинного кода файла (wrap.go)
	longest int
	// versionKey ключ версий, по которым сейчас раскрашен файл, пусто при
	// подсветке по хункам (versions.go)
	versionKey string
	// выделение строк для индекса (select.go)
	selection selection
	// лента всех файлов (stream.go); inStream значит, что модель это страница
	// ленты: её прокрутку держит лента
	stream   stream
	inStream bool
}

// settings это вид и кеш версий: у ленты они общие для всех её страниц.
type settings struct {
	colors theme.Theme
	mode   Mode
	// с этой ширины auto выбирает split, по умолчанию 120 (diff.autoSplitMinWidth)
	autoSplitMinWidth int
	width             int
	height            int
	// колонка номеров скрыта: флаг меняет только рисунок, строки те же
	hideNumbers bool
	// служебные строки на языке интерфейса
	text i18n.Dictionary
	// wrap переносит длинные строки, без него код сдвигается вбок на shift
	// колонок (wrap.go)
	wrap  bool
	shift int
	// share доля левой половины split, 0 значит пополам (split.go)
	share float64
	// versions подсветка по версиям файла целиком (versions.go)
	versions map[string]sides
}

func New(colors theme.Theme, mode Mode, autoSplitMinWidth int) Model {
	return Model{settings: settings{colors: colors, mode: mode, autoSplitMinWidth: autoSplitMinWidth}}
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

// SetFile открывает другой файл: прокрутка и выделение сбрасываются.
func (m Model) SetFile(file diff.File) Model {
	m.scroll, m.shift = 0, 0
	m.selection = selection{}
	return m.withFile(file).rebuild()
}

// Reload ставит на место файла его новую версию. Прокрутка и выделение
// остаются, прокрутка обрезана по новой длине, а SetFile их сбросил бы. При
// выделении экран идёт за курсором: после space он мог уйти за экран. В ленте
// так обновляется файл выделения, остальные файлы ставит ReloadAll.
func (m Model) Reload(file diff.File) Model {
	return m.onPage(func(m Model) Model {
		m = m.withFile(file).rebuild()
		if m.selection.on {
			return m.follow()
		}
		return m
	})
}

// withFile ставит файл и размечает его код.
func (m Model) withFile(file diff.File) Model {
	return m.plain(file).colorize()
}

// plain ставит файл без подсветки: у страницы ленты её считает Prepare, когда
// страница видна.
func (m Model) plain(file diff.File) Model {
	m.file = file
	m.oldCode, m.newCode, m.colored = nil, nil, false
	m.versionKey = ""
	return m
}

// colorize размечает код файла. Язык выбирается по пути: у удалённого файла
// путь старый, у переименованного новый.
func (m Model) colorize() Model {
	m.oldCode, m.newCode = highlight(syntax.Detect(cmp.Or(m.file.NewPath, m.file.OldPath)), m.file)
	m.colored = true
	return m
}

// SetSize может сменить вид в режиме auto, поэтому держит на месте верхнюю
// строку файла, как ToggleMode.
func (m Model) SetSize(width, height int) Model {
	return m.keepTop(func(m Model) Model {
		m.width, m.height = width, height
		return m
	})
}

// keepTop меняет вид и пересчитывает строки, держа наверху ту же строку
// файла: у одного файла по номеру строки, у ленты по файлу и номеру строки в
// нём.
func (m Model) keepTop(change func(Model) Model) Model {
	if m.stream.on {
		return change(m).relayout(m.place())
	}

	top := m.topPosition()
	return change(m).rebuild().seek(top)
}

// Mode это вид из настройки или после m: auto остаётся auto, а не вид на
// экране.
func (m Model) Mode() Mode {
	return m.mode
}

// ToggleMode переключает от того вида, что сейчас на экране, а не от auto, и
// держит выбор до конца сеанса: смена ширины после этого вид не меняет.
func (m Model) ToggleMode() Model {
	return m.keepTop(func(m Model) Model {
		if m.effectiveMode() == ModeSplit {
			m.mode = ModeUnified
		} else {
			m.mode = ModeSplit
		}
		return m
	})
}

// position это номер строки файла наверху экрана: новый, а если его нет
// (удалённая строка), старый. В split строк меньше, чем в unified, и одна и та
// же позиция прокрутки показывает в них разные места, а номер строки нет.
type position struct {
	number int
	old    bool
}

// topPosition берёт первую строку с номером, начиная с верхней: у разрыва между
// хунками и пометки «No newline» номеров нет, и верх по такой строке в другом
// виде указал бы на другое место. Служебная строка (надпись над файлом) верх не
// определяет, он остаётся по индексу.
func (m Model) topPosition() position {
	if m.scroll >= len(m.lines) || m.lines[m.scroll].notice != nil {
		return position{}
	}

	for _, line := range m.lines[m.scroll:] {
		oldNumber, newNumber := line.numbers()
		switch {
		case newNumber > 0:
			return position{number: newNumber}
		case oldNumber > 0:
			return position{number: oldNumber, old: true}
		}
	}

	return position{}
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
	return m.clamp()
}

func (m Model) Bottom() Model {
	m.scroll = len(m.lines)
	if m.stream.on {
		m.scroll = m.stream.total
	}
	return m.clamp()
}

// clamp держит прокрутку в пределах: последняя строка файла не уходит выше
// нижнего края экрана, первая не уходит ниже верхнего. Пределы ленты и её
// страниц в stream.go.
func (m Model) clamp() Model {
	switch {
	case m.inStream:
		// у страницы верх держит лента: её последняя строка может стоять у
		// верха экрана, под ней следующий файл
		m.scroll = max(min(m.scroll, len(m.lines)-1), 0)
	case m.stream.on:
		m = m.clampStream()
	default:
		m.scroll = max(min(m.scroll, len(m.lines)-m.rows()), 0)
	}
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
	var rendered []string
	if m.stream.on {
		rendered = m.streamRows()
	} else {
		rendered = m.lineRows(m.scroll, min(m.scroll+m.rows(), len(m.lines)))
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

// lineRows рисует строки экрана с from до to, не включая to.
func (m Model) lineRows(from, to int) []string {
	first, last := m.bounds()

	rendered := make([]string, 0, m.height)
	for index := from; index < to; index++ {
		line := m.lines[index]
		// конфликт отмечен целиком: наша сторона в нём бывает контекстом
		marked := index >= first && index <= last && (m.selection.conflict || len(refs(line)) > 0)
		rendered = append(rendered, m.render(line, marked))
	}
	return rendered
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

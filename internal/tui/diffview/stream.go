package diffview

import (
	"fmt"
	"iter"
	"reflect"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/diff"
)

// Лента (#373) показывает все файлы диффа подряд. Каждый файл это страница:
// модель одного файла со своими строками, подсветкой и выделением, а лента
// держит вид, прокрутку и текущий файл. Над строками страницы три строки
// заголовка: пустая, «путь +N -M» и пустая. Прокрутка ленты это номер строки
// во всей ленте; файл и строку в нём (place) лента считает только вокруг
// пересборки, поэтому страница выше экрана, сменившая высоту, верх не
// сдвигает.

// headerRows это строки заголовка файла в ленте.
const headerRows = 3

// stream это лента. files файлы диффа, titles их подписи: их собирает корень,
// у него словарь и очистка путей от управляющих символов. pages страницы
// файлов, starts строка ленты, с которой начинается каждая, total высота
// ленты, current файл, на котором она стоит. widest самый дальний сдвиг вбок
// по всем страницам, bar страница, которая его даёт: по ней рисуется полоса
// прокрутки.
type stream struct {
	on      bool
	files   []diff.File
	titles  []string
	pages   []Model
	starts  []int
	total   int
	current int
	widest  int
	bar     int
}

// place это место верха ленты: страница и строка экрана в ней, номер строки
// файла на этой строке (как topPosition у одного файла), а если header, то
// строка заголовка.
type place struct {
	page     int
	header   bool
	row      int
	position position
}

// Streaming говорит, что на экране лента всех файлов.
func (m Model) Streaming() bool {
	return m.stream.on
}

// Current это номер файла, на котором стоит лента: при выделении файл
// выделения, иначе тот, чья строка наверху.
func (m Model) Current() int {
	return m.stream.current
}

// ShowAll включает ленту файлов files с подписями titles. Наверху остаётся та
// же строка открытого файла current, а начало файла становится его
// заголовком.
func (m Model) ShowAll(files []diff.File, titles []string, current int) Model {
	at := place{page: current, header: true}
	if m.scroll > 0 {
		at = place{page: current, row: m.scroll, position: m.topPosition()}
	}

	m.stream = stream{on: true, current: current}
	return m.withFiles(files, titles, nil).seekPlace(at)
}

// ShowOne выключает ленту: остаётся текущий файл, наверху та же его строка, а
// вместо его заголовка начало файла.
func (m Model) ShowOne() Model {
	if len(m.stream.pages) == 0 {
		m.stream = stream{}
		return m.SetFile(diff.File{})
	}

	at := m.place()
	one := m.stream.pages[m.stream.current]
	one.settings, one.inStream = m.settings, false
	if !one.colored {
		one = one.colorize()
	}
	one = one.rebuild()
	one.scroll = 0
	if !at.header {
		one.scroll = at.row
	}
	return one.seek(at.position)
}

// ReloadAll ставит новый список файлов. Страница, чей файл не изменился,
// переходит как была, с подсветкой: опрос раз в две секунды иначе
// перекрашивал бы видимые файлы. Страница выделения тоже переходит как была:
// на ней дифф вне индекса, его обновит корень. Наверх встаёт файл keep: если
// это тот же файл (same), то та же его строка, иначе его заголовок. keep -1
// значит, что файлов нет.
func (m Model) ReloadAll(files []diff.File, titles []string, keep int, same bool) Model {
	at := m.place()
	selecting := m.Selecting()

	previous := make(map[string]int, len(m.stream.files))
	for index, file := range m.stream.files {
		previous[fileKey(file)] = index
	}
	kept := make(map[int]Model)
	for index, file := range files {
		old, found := previous[fileKey(file)]
		switch {
		case !found:
		case selecting && same && index == keep && old == m.stream.current:
			kept[index] = m.stream.pages[old]
		case !m.stream.pages[old].selection.on && reflect.DeepEqual(m.stream.files[old], file):
			kept[index] = m.stream.pages[old]
		}
	}

	_, unchanged := kept[keep]
	m.stream.current = max(keep, 0)
	m = m.withFiles(files, titles, kept)
	switch {
	case keep < 0 || !same:
		at = place{page: keep, header: true}
	case unchanged:
		// строки страницы те же, верх встаёт ровно туда же
		at.page, at.position = keep, position{}
	default:
		at.page = keep
	}
	return m.seekPlace(at)
}

// fileKey отличает файлы диффа друг от друга: у файла, ставшего симлинком,
// две записи с одним путём, удалённая и добавленная.
func fileKey(file diff.File) string {
	return file.OldPath + "\x00" + file.NewPath + "\x00" + string(file.Status)
}

// withFiles ставит файлы ленты и собирает их страницы. Страницы из kept, по
// номеру нового файла, переходят как есть. До первого размера строки не
// собираются: их соберёт SetSize, а сборка на нулевой ширине пропала бы зря.
func (m Model) withFiles(files []diff.File, titles []string, kept map[int]Model) Model {
	m.stream.files, m.stream.titles = files, titles
	m.stream.pages = make([]Model, len(files))
	for index, file := range files {
		if page, found := kept[index]; found {
			m.stream.pages[index] = page
			continue
		}
		m.stream.pages[index] = Model{inStream: true}.plain(file)
		if m.width > 0 {
			m.stream.pages[index] = m.pageFor(index).rebuild()
		}
	}

	m.stream.current = min(m.stream.current, max(len(files)-1, 0))
	return m.measure()
}

// Open ставит наверх заголовок файла index. Выделение снимается, как при
// смене файла без ленты: оно живёт в одном файле.
func (m Model) Open(index int) Model {
	if index < 0 || index >= len(m.stream.pages) {
		return m
	}
	if m.Selecting() {
		m = m.Deselect(m.stream.files[m.stream.current])
	}

	m.stream.current = index
	m.scroll = m.stream.starts[index]
	return m.clamp()
}

// AtFileStart говорит, что наверху заголовок текущего файла ленты: [ ведёт
// тогда к предыдущему файлу, а из середины файла сначала к его заголовку.
// Без ленты всегда true.
func (m Model) AtFileStart() bool {
	return !m.stream.on || len(m.stream.pages) == 0 || m.scroll == m.stream.starts[m.stream.current]
}

// Prepare раскрашивает видимые страницы ленты: подсветка синтаксиса дорогая,
// и лента считает её только для того, что на экране. Корень зовёт Prepare в
// конце каждого Update, и View получает уже раскрашенное.
// ponytail: подсветка идёт в Update, и большой файл, въехавший на экран,
// останавливает прокрутку на 100–300 мс; фоновая команда, как у версий, если
// это заметят (#373).
func (m Model) Prepare() Model {
	var pages []Model
	for index := range m.shown() {
		if m.stream.pages[index].colored {
			continue
		}
		if pages == nil {
			pages = slices.Clone(m.stream.pages)
		}
		pages[index] = pages[index].colorize()
	}

	if pages != nil {
		m.stream.pages = pages
	}
	return m
}

// shown это номера страниц, строки которых видны на экране.
func (m Model) shown() iter.Seq[int] {
	return func(yield func(int) bool) {
		if !m.stream.on {
			return
		}
		end := min(m.scroll+m.rows(), m.stream.total)
		for row := m.scroll; row < end; {
			index := m.pageAt(row)
			if !yield(index) {
				return
			}
			row = m.pageEnd(index)
		}
	}
}

// splitShown говорит, что на экране есть файл в split: стык половин тянется
// только у него.
func (m Model) splitShown() bool {
	for index := range m.shown() {
		if m.pageMode(m.stream.files[index]) == ModeSplit {
			return true
		}
	}
	return false
}

// pageAt это страница, в которой стоит строка ленты row.
func (m Model) pageAt(row int) int {
	index, found := slices.BinarySearch(m.stream.starts, row)
	if !found {
		index--
	}
	return max(index, 0)
}

// pageEnd это строка ленты сразу за страницей index.
func (m Model) pageEnd(index int) int {
	return m.stream.starts[index] + headerRows + len(m.stream.pages[index].lines)
}

// pageFor это страница index с видом ленты: хранимая страница несёт вид на
// момент сборки, а сдвиг и размер у ленты меняются и без пересборки.
func (m Model) pageFor(index int) Model {
	page := m.stream.pages[index]
	page.settings = m.settings
	page.mode = m.pageMode(m.stream.files[index])
	return page
}

// pageMode это вид страницы файла: в split файл с изменениями на одной
// стороне идёт unified во всю ширину, вторая половина была бы пустой. Сторону
// решает файл диффа, а не показанный при выделении: иначе v перекладывал бы
// страницу из вида в вид.
func (m Model) pageMode(file diff.File) Mode {
	mode := m.effectiveMode()
	if mode == ModeSplit && (file.Added == 0 || file.Removed == 0) {
		return ModeUnified
	}
	return mode
}

// focus это модель, в которой живут выделение и версии: страница текущего
// файла ленты, а без ленты сама модель.
func (m Model) focus() Model {
	if m.stream.on && len(m.stream.pages) > 0 {
		return m.pageFor(m.stream.current)
	}
	return m
}

// onPage меняет модель, в которой живут выделение и версии. В ленте это
// страница текущего файла: верх ленты отдаётся ей как её прокрутка и высота,
// а потом забирается обратно, если страница его сдвинула (Select, Deselect и
// follow держат место сами). Пока наверху заголовок, страница видна ниже него
// и на экране короче.
func (m Model) onPage(change func(Model) Model) Model {
	if !m.stream.on {
		return change(m)
	}
	if len(m.stream.pages) == 0 {
		return m
	}

	index := m.stream.current
	local := m.scroll - m.stream.starts[index]
	header := max(headerRows-local, 0)
	page := m.pageFor(index)
	page.scroll = max(local-headerRows, 0)
	page.height = max(m.rows()-header, 1)
	before := page.scroll
	page = change(page)

	m.stream.pages = slices.Clone(m.stream.pages)
	m.stream.pages[index] = page
	m = m.measure()
	if header == 0 || page.scroll != before {
		m.scroll = m.stream.starts[index] + headerRows + page.scroll
	}
	return m.clamp()
}

// measure считает, с какой строки ленты начинается каждая страница, и самый
// дальний сдвиг вбок: сдвиг у ленты общий.
func (m Model) measure() Model {
	m.stream.starts = make([]int, len(m.stream.pages))
	m.stream.total, m.stream.widest, m.stream.bar = 0, 0, 0
	for index, page := range m.stream.pages {
		m.stream.starts[index] = m.stream.total
		m.stream.total += headerRows + len(page.lines)
		if shift := m.pageFor(index).maxShift(); shift > m.stream.widest {
			m.stream.widest, m.stream.bar = shift, index
		}
	}

	m.shift = min(m.shift, m.stream.widest)
	return m
}

// relayout пересобирает строки всех страниц после смены вида и ставит наверх
// место at.
// ponytail: пересобирает всю ленту на каждое событие, и на диффе в десятки
// тысяч строк ресайз и стык split отстают от мыши; пересборка только видимых
// страниц, если это заметят (#373).
func (m Model) relayout(at place) Model {
	pages := make([]Model, len(m.stream.pages))
	for index := range pages {
		pages[index] = m.pageFor(index).rebuild()
	}
	m.stream.pages = pages
	return m.measure().seekPlace(at)
}

// place это место верха ленты.
func (m Model) place() place {
	if len(m.stream.pages) == 0 {
		return place{header: true}
	}

	index := m.stream.current
	local := m.scroll - m.stream.starts[index]
	if local < headerRows {
		return place{page: index, header: true, row: max(local, 0)}
	}

	page := m.pageFor(index)
	page.scroll = local - headerRows
	return place{page: index, row: page.scroll, position: page.topPosition()}
}

// seekPlace ставит наверх место at: строку заголовка, строку файла с тем же
// номером, а если номера нет (надпись над файлом), строку экрана с тем же
// номером в странице.
func (m Model) seekPlace(at place) Model {
	if len(m.stream.pages) == 0 {
		m.scroll = 0
		return m.clamp()
	}

	index := min(max(at.page, 0), len(m.stream.pages)-1)
	m.stream.current = index
	m.scroll = m.stream.starts[index] + at.row
	if !at.header {
		page := m.pageFor(index)
		page.scroll = at.row
		page = page.seek(at.position)
		m.scroll = m.stream.starts[index] + headerRows + page.scroll
	}
	return m.clamp()
}

// clampStream держит верх ленты в пределах. Заголовок последнего файла может
// встать наверх, под ним пусто: иначе короткий последний файл никогда не стал
// бы текущим. При выделении верх не выходит из блока его файла: space и x
// действуют на файл выделения.
func (m Model) clampStream() Model {
	last := len(m.stream.pages) - 1
	if last < 0 {
		m.scroll = 0
		return m
	}

	low, high := 0, max(m.stream.total-m.rows(), m.stream.starts[last])
	selecting := m.focus().selection.on
	if selecting {
		low = m.stream.starts[m.stream.current]
		high = max(low, m.pageEnd(m.stream.current)-m.rows())
	}
	m.scroll = max(min(m.scroll, high), low)
	if !selecting {
		m.stream.current = m.pageAt(m.scroll)
	}
	return m
}

// streamRows рисует видимые строки ленты: у каждой страницы сначала её
// заголовок, потом её строки.
func (m Model) streamRows() []string {
	rows := m.rows()
	rendered := make([]string, 0, m.height)
	for row := m.scroll; row < m.stream.total && len(rendered) < rows; {
		index := m.pageAt(row)
		local := row - m.stream.starts[index]
		for ; local < headerRows && len(rendered) < rows; local++ {
			rendered = append(rendered, m.fileHeader(index, local))
		}

		page := m.pageFor(index)
		from := local - headerRows
		to := min(len(page.lines), from+rows-len(rendered))
		rendered = append(rendered, page.lineRows(from, to)...)
		row = m.pageEnd(index)
	}
	return rendered
}

// fileHeader это строка row заголовка файла index: над подписью и под ней
// пустые строки, подпись «путь +N -M» слева на фоне панели, как заголовок
// файла в critique. Справа место тоста, и подпись слева он не закрывает.
// Счётчики берутся из файла диффа: при выделении на странице дифф вне индекса.
// Без изменённых строк счётчиков нет, как в заголовке экрана.
func (m Model) fileHeader(index, row int) string {
	if row != 1 {
		return ""
	}

	file := m.stream.files[index]
	panel := lipgloss.NewStyle().Background(m.colors.BackgroundPanel)
	tail, tailWidth := "", 0
	if file.Added > 0 || file.Removed > 0 {
		added, removed := fmt.Sprintf("+%d", file.Added), fmt.Sprintf("-%d", file.Removed)
		tail = panel.Render(" ") + panel.Foreground(m.colors.DiffAdded).Render(added) +
			panel.Render(" ") + panel.Foreground(m.colors.DiffRemoved).Render(removed)
		tailWidth = 2 + len(added) + len(removed)
	}

	// поле в одну колонку слева, как у кода
	name := ansi.Truncate(m.stream.titles[index], max(m.width-2-tailWidth, 1), "…")
	used := 1 + ansi.StringWidth(name) + tailWidth
	label := panel.Render(" "+name) + tail + panel.Render(strings.Repeat(" ", max(m.width-used, 0)))
	return ansi.Truncate(label, m.width, "")
}

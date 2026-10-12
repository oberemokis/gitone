package tui

import (
	"image"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/config"
	"gitone/internal/diff"
	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
	"gitone/internal/layout"
	"gitone/internal/tui/diffview"
	filespane "gitone/internal/tui/pane/files"
	"gitone/internal/tui/picker"
	"gitone/internal/tui/theme"
	"gitone/internal/tui/zone"
)

// Строк за один щелчок колеса: по одной слишком медленно, трекпад шлёт
// десятки щелчков на жест.
const wheelStep = 3

// Колонок за одно нажатие стрелки или щелчок колеса вбок.
const shiftStep = 8

type Model struct {
	// dir и args нужны для перечитывания по r: тот же git diff, что при запуске
	dir     string
	args    []string
	loading bool
	// spin кадр спиннера загрузки в подвале
	spin int
	// stale: правка gitone пришла во время загрузки, после неё нужно перечитать ещё раз
	stale bool
	// polling: идёт тихий опрос из live.go; ticks таймер опроса, nil без автообновления
	polling bool
	ticks   <-chan time.Time
	// staged: пути с отличием индекса от HEAD, по ним отметка в панели файлов
	staged map[string]bool
	// location ветка и worktree для левого края подвала
	location git.Location
	// baseArgs аргументы запуска: b возвращает к ним дифф рабочего дерева
	baseArgs []string
	// active окно, которое получает клавиши; cursor курсор открытого списка
	active window
	cursor int
	// данные окон: пути под вопросом discard, тема коммита, записи списков
	confirm    []string
	subject    string
	logEntries []git.LogEntry
	// поиск в логе, ветках и worktree: запрос и открытое поле (window.go).
	// Окно открыто одно, поэтому поиск общий, открытие окна его сбрасывает.
	query     string
	searching bool
	branches  []git.Branch
	worktrees []git.Worktree
	stashes   []git.Stash
	picker    picker.Model
	// ошибка последнего перечитывания, стоит в подвале до следующего нажатия
	notice string
	keys   keymap.Keymap
	ui     config.UI
	layout layout.Node
	// filesOpen: панель файлов открыта, при запуске она скрыта всегда
	filesOpen bool
	width     int
	height    int
	helpOpen  bool
	colors    theme.Theme
	text      i18n.Dictionary
	files     []diff.File
	current   int
	view      diffview.Model
}

// Options это всё, что корню нужно до запуска: каталог и аргументы для
// перечитывания по r, настройки, клавиши, раскладка и текст ошибок их загрузки
// для подвала. Пустая раскладка значит один дифф на весь экран.
type Options struct {
	Dir    string
	Args   []string
	Config config.Config
	Keys   keymap.Keymap
	Theme  theme.Theme
	Layout layout.Node
	Notice string
	// Ticks таймер опроса для автообновления, nil значит без него
	Ticks <-chan time.Time
	// Staged пути, уже лежащие в индексе при запуске
	Staged map[string]bool
	// Location ветка и worktree при запуске, дальше их обновляет перечитывание
	Location git.Location
	// Text тексты интерфейса на выбранном языке, нулевое значение английское
	Text i18n.Dictionary
}

func New(opts Options, files []diff.File) Model {
	colors := opts.Theme

	root := opts.Layout
	if root.Pane == "" && len(root.Children) == 0 {
		root = layout.Focus()
	}

	view := diffview.New(colors, diffview.Mode(opts.Config.Diff.Mode), opts.Config.Diff.AutoSplitMinWidth).
		WithoutNumbers(!opts.Config.UI.LineNumbers).
		WithText(opts.Text).
		WithWrap(opts.Config.Diff.Wrap)
	if len(files) > 0 {
		view = view.SetFile(files[0])
	}

	return Model{
		dir:      opts.Dir,
		args:     opts.Args,
		baseArgs: opts.Args,
		ticks:    opts.Ticks,
		staged:   opts.Staged,
		location: opts.Location,
		notice:   opts.Notice,
		keys:     opts.Keys,
		ui:       opts.Config.UI,
		layout:   root,
		colors:   colors,
		text:     opts.Text,
		files:    files,
		view:     view,
	}
}

func (m Model) Init() tea.Cmd {
	return waitTick(m.ticks)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m = m.resize()
	case tea.KeyPressMsg:
		return m.keyPress(msg)
	case tea.MouseWheelMsg:
		m.view = m.wheel(msg)
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			return m.click(msg.X, msg.Y)
		}
	case tickMsg:
		return m.ticked()
	// возврат в терминал из редактора: правка видна сразу, а не к тику
	case tea.FocusMsg:
		if m.ticks != nil {
			return m.poll()
		}
	case polledMsg:
		m = m.polled(msg)
	case spinMsg:
		if m.loading {
			m.spin++
			return m, spinTick()
		}
	case reloadMsg:
		return m.reloaded(msg)
	case actionDoneMsg:
		return m.actionDone(msg)
	// Списки приходят асинхронно. Если за это время открыли другое окно, список
	// отбрасывается: иначе на экране одно окно, а клавиши уходят другому.
	// Ошибка доходит до подвала при любом окне.
	case stashesMsg:
		switch {
		case msg.err != nil:
			m = m.say(m.text.Error(msg.err))
		case m.active == noWindow:
			m.stashes, m.cursor, m.active = msg.stashes, 0, stashWindow
		}
	case branchesMsg:
		switch {
		case msg.err != nil:
			m = m.say(m.text.Error(msg.err))
		case m.active == noWindow:
			m.branches, m.cursor, m.active = msg.branches, currentIndex(msg.branches), branchWindow
			m.query, m.searching = "", false
		}
	case worktreesMsg:
		switch {
		case msg.err != nil:
			m = m.say(m.text.Error(msg.err))
		case m.active == noWindow:
			m.worktrees, m.cursor, m.active = msg.worktrees, currentWorktree(msg.worktrees), worktreeWindow
			m.query, m.searching = "", false
		}
	case logMsg:
		switch {
		case msg.err != nil:
			m = m.say(m.text.Error(msg.err))
		case m.active == noWindow:
			m.logEntries, m.cursor, m.active = msg.entries, 0, logWindow
			m.query, m.searching = "", false
		}
	}

	return m, nil
}

// say ставит асинхронное сообщение в подвал. Висящий вопрос discard при этом
// отменяется: его текст затёрт, и y не должен стирать правку, о которой
// пользователь вопроса уже не видит.
func (m Model) say(text string) Model {
	m.notice = text
	return m.dropQuestion()
}

func (m Model) dropQuestion() Model {
	if m.active == discardWindow {
		m.active = noWindow
	}
	m.confirm = nil
	return m
}

// startReload запускает перечитывание, если оно не идёт. Повтор во время загрузки
// не нужен: загрузка уже читает актуальный дифф. Висящий вопрос discard
// отменяется по той же причине, что в say: спиннер встаёт на его место в
// центре подвала.
func (m Model) startReload() (Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}

	m.loading = true
	m.stale = false
	m.spin = 0
	m = m.dropQuestion()
	return m, tea.Batch(spinTick(), reload(m.dir, m.args))
}

// resize отдаёт диффу ширину и высоту его панели: раскладка может отрезать
// от экрана список файлов.
func (m Model) resize() Model {
	if diffPane, found := findPane(m.placement(), "diff"); found {
		m.view = m.view.SetSize(diffPane.Width, diffPane.Height)
	}
	return m
}

// placement считает размеры панелей для текущего экрана.
func (m Model) placement() layout.Placed {
	return layout.Split(m.arrangement(), m.width, m.bodyHeight())
}

// arrangement это раскладка с учётом панели файлов. Закрытая панель
// убирается из дерева, открытая встаёт туда, куда её поставила раскладка, а
// в раскладке без неё слева, как в default. Открыть панель решает
// пользователь, поэтому узкий экран её не прячет.
func (m Model) arrangement() layout.Node {
	switch {
	case !m.filesOpen:
		return layout.Without(m.layout, "files")
	case layout.Has(m.layout, "files"):
		return m.layout
	}
	return layout.Default()
}

// wheel прокручивает дифф колесом: вверх и вниз по строкам, вбок по
// колонкам. Колесо с shift тоже идёт вбок: так горизонтальную прокрутку
// шлёт мышь без второго колеса.
func (m Model) wheel(msg tea.MouseWheelMsg) diffview.Model {
	button := msg.Button
	if msg.Mod&tea.ModShift != 0 {
		switch button {
		case tea.MouseWheelUp:
			button = tea.MouseWheelLeft
		case tea.MouseWheelDown:
			button = tea.MouseWheelRight
		}
	}

	switch button {
	case tea.MouseWheelDown:
		return m.view.ScrollBy(wheelStep)
	case tea.MouseWheelUp:
		return m.view.ScrollBy(-wheelStep)
	case tea.MouseWheelRight:
		return m.view.ScrollRight(shiftStep)
	case tea.MouseWheelLeft:
		return m.view.ScrollRight(-shiftStep)
	}
	return m.view
}

func findPane(placed layout.Placed, pane string) (layout.Placed, bool) {
	if placed.Pane == pane && len(placed.Children) == 0 {
		return placed, true
	}

	for _, child := range placed.Children {
		if found, ok := findPane(child, pane); ok {
			return found, true
		}
	}

	return layout.Placed{}, false
}

func (m Model) handleAction(action keymap.Action) (tea.Model, tea.Cmd) {
	switch action {
	case keymap.AppQuit:
		return m, tea.Quit
	case keymap.AppHelp:
		m.helpOpen = !m.helpOpen
	case keymap.HelpClose:
		m.helpOpen = false
	case keymap.FilesPick:
		m = m.openPicker()
	case keymap.FilesToggle:
		m.filesOpen = !m.filesOpen
		m = m.resize()
	case keymap.FilesNext:
		m = m.openFile(m.current + 1)
	case keymap.FilesPrevious:
		m = m.openFile(m.current - 1)
	case keymap.DiffToggleMode:
		m.view = m.view.ToggleMode()
	case keymap.DiffToggleWrap:
		m.view = m.view.ToggleWrap()
	case keymap.DiffScrollLeft:
		m.view = m.view.ScrollRight(-shiftStep)
	case keymap.DiffScrollRight:
		m.view = m.view.ScrollRight(shiftStep)
	case keymap.DiffReload:
		return m.startReload()
	case keymap.FilesStage:
		return m.toggleStage()
	case keymap.FilesDiscard:
		m = m.askDiscard()
	case keymap.FilesCommit:
		m = m.openCommit()
	case keymap.AppLog:
		return m, logCmd(m.dir)
	case keymap.AppBranches:
		return m, branchesCmd(m.dir)
	case keymap.AppWorktrees:
		return m, worktreesCmd(m.dir)
	case keymap.StashPush:
		return m, stashPushCmd(m.dir)
	case keymap.StashList:
		return m, stashListCmd(m.dir)
	case keymap.DiffWorkingTree:
		m.args = m.baseArgs
		return m.startReload()
	case keymap.DiffScrollDown:
		m.view = m.view.ScrollBy(1)
	case keymap.DiffScrollUp:
		m.view = m.view.ScrollBy(-1)
	case keymap.DiffHalfPageDown:
		m.view = m.view.ScrollBy(m.bodyHeight() / 2)
	case keymap.DiffHalfPageUp:
		m.view = m.view.ScrollBy(-m.bodyHeight() / 2)
	case keymap.DiffTop:
		m.view = m.view.Top()
	case keymap.DiffBottom:
		m.view = m.view.Bottom()
	}

	return m, nil
}

// pickerKey отдаёт нажатие окну поиска целиком: q и ? там набираются в
// поиск. Выйти из приложения можно и отсюда, ctrl+c работает всегда.
func (m Model) pickerKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.String() == "ctrl+c" {
		return m, tea.Quit
	}

	var outcome picker.Outcome
	m.picker, outcome = m.picker.HandleKey(key)

	switch outcome {
	case picker.Chosen:
		m.active = noWindow
		if index, found := m.chosenFile(); found {
			m = m.openFile(index)
		}
	case picker.Closed:
		m.active = noWindow
	}

	return m, nil
}

// chosenFile ищет выбранную в окне запись в текущем диффе по подписи и статусу.
// Окно строилось по списку на момент открытия, а дифф мог перечитаться, пока
// оно открыто: номер записи тогда указал бы на другой файл. Подпись и статус
// с обеих сторон собраны одним словарём, поэтому сравнение не зависит от
// языка.
func (m Model) chosenFile() (int, bool) {
	item, found := m.picker.SelectedItem()
	if !found {
		return 0, false
	}

	index := slices.IndexFunc(m.files, func(file diff.File) bool {
		return fileTitle(file, m.text) == item.Label && statusLabel(file.Status, m.text) == item.Detail
	})
	return index, index >= 0
}

// applyReload ставит новый дифф на место старого. Открытый файл остаётся
// открытым, если он ещё в диффе, иначе открывается первый.
func (m Model) applyReload(files []diff.File) Model {
	kept := -1
	if len(m.files) > 0 {
		title := fileTitle(m.files[m.current], m.text)
		kept = slices.IndexFunc(files, func(file diff.File) bool {
			return fileTitle(file, m.text) == title
		})
	}

	m.files = files
	if kept >= 0 {
		m.current = kept
		m.view = m.view.Reload(files[kept])
		return m
	}

	m.current = 0
	if len(files) > 0 {
		m.view = m.view.SetFile(files[0])
	}
	return m
}

// openPicker открывает поиск и без файлов: пустой список с надписью лучше
// клавиши, которая молча не работает.
func (m Model) openPicker() Model {
	items := make([]picker.Item, len(m.files))
	for index, file := range m.files {
		items[index] = picker.Item{Label: fileTitle(file, m.text), Detail: statusLabel(file.Status, m.text)}
	}

	noMatches := "files.noMatches"
	if len(m.files) == 0 {
		noMatches = "files.none"
	}
	texts := picker.Texts{
		Title:      m.text.Text("files.title", nil),
		Search:     m.text.Text("files.search", nil),
		NoMatches:  m.text.Text(noMatches, nil),
		CloseKey:   firstKey(m.keys, keymap.SearchClose),
		CloseLabel: m.text.Text("window.close", nil),
	}
	keys := picker.Keys{
		Choose: m.keys.KeysFor(keymap.SearchChoose),
		Close:  m.keys.KeysFor(keymap.SearchClose),
		Up:     m.keys.KeysFor(keymap.SearchUp),
		Down:   m.keys.KeysFor(keymap.SearchDown),
	}
	m.picker = picker.New(texts, keys, items, m.colors)
	m.active = pickerWindow
	m.helpOpen = false
	return m
}

// openFile открывает файл по номеру в диффе. За краями списка ничего не
// происходит: ] на последнем файле не уводит на первый.
func (m Model) openFile(index int) Model {
	if index < 0 || index >= len(m.files) || index == m.current {
		return m
	}

	m.current = index
	m.view = m.view.SetFile(m.files[index])
	return m
}

func (m Model) View() tea.View {
	view := tea.NewView("")
	view.AltScreen = true
	// Колесо мыши прокручивает дифф. Терминал при этом выделяет текст только
	// с shift: это принятая цена (решение в #10).
	view.MouseMode = tea.MouseModeCellMotion
	view.ReportFocus = m.ticks != nil

	// до первого WindowSizeMsg ширина неизвестна, раскладывать не по чему
	if m.width == 0 {
		return view
	}

	view.Content, _ = m.screen()
	return view
}

// screen рисует экран и находит на нём места клика (#119). Метки снимаются
// с каждой части до того, как окно ляжет поверх панелей: слой Lip Gloss их не
// переносит. Окно идёт в списке мест первым и целиком, клик по пустому месту в
// нём не доходит до панели под ним. Пока открыто окно, клик работает только в
// нём, как клавиши, поэтому места панелей и подвала не собираются.
func (m Model) screen() (string, []zone.Zone) {
	var zones []zone.Zone
	scan := func(content string, x, y int, collect bool) string {
		clean, found := zone.Scan(content)
		for _, place := range found {
			if collect {
				place.Area = place.Area.Add(image.Pt(x, y))
				zones = append(zones, place)
			}
		}
		return clean
	}
	free := m.active == noWindow
	top, height := m.titleHeight(), m.bodyHeight()

	box := m.windowBox()
	if box != "" {
		x, y := centered(box, m.width, height)
		box = scan(box, x, top+y, true)
		zones = append(zones, zone.Zone{Area: image.Rect(x, top+y, x+lipgloss.Width(box), top+y+lipgloss.Height(box))})
	}

	body := scan(lipgloss.Place(m.width, height, lipgloss.Left, lipgloss.Top, m.panes(m.placement())), 0, top, free)
	if box != "" {
		body = overlay(body, box, m.width, height)
	}

	var rows []string
	if m.ui.PaneTitles {
		title := ""
		if len(m.files) > 0 {
			title = header(m.width, m.files, m.current, m.colors, m.text)
		}
		rows = append(rows, "", scan(title, 0, 1, free), "")
	}
	rows = append(rows, body, "", scan(m.footerLine(), 0, top+height+1, free), "")

	return lipgloss.JoinVertical(lipgloss.Left, rows...), zones
}

// footerLine собирает подвал: ветку слева, статус по центру, подсказки справа.
// Пока идёт перечитывание, в центре спиннер, а висящая ошибка git ждёт
// следующего нажатия.
func (m Model) footerLine() string {
	center := ""
	switch {
	case m.loading:
		center = spinner(m.spin, m.colors, m.text)
	case m.notice != "":
		first, _, _ := strings.Cut(m.notice, "\n")
		center = lipgloss.NewStyle().Foreground(m.colors.Error).Render(first)
	}

	hints := ""
	if m.ui.KeyHints {
		hints = keyHints(m.colors, m.text, m.keys.Bindings())
	}

	return footer(m.width, locationLabel(m.location, m.colors, m.keys), center, hints)
}

// panes рисует дерево размещения: лист это панель, контейнер склеивает детей
// по своей оси. Без изменений раскладка та же, а панели говорят об этом
// сами: так видно, что открыто и куда пойдут клавиши.
func (m Model) panes(placed layout.Placed) string {
	noChanges := m.text.Text("screen.noChanges", nil)
	switch {
	case len(placed.Children) > 0:
	case placed.Pane == "files":
		return filespane.View(m.labels(), m.current, noChanges, m.colors, placed.Width, placed.Height)
	case len(m.files) == 0:
		return lipgloss.Place(placed.Width, placed.Height, lipgloss.Center, lipgloss.Center, noChanges)
	default:
		return m.view.View()
	}

	parts := make([]string, len(placed.Children))
	for index, child := range placed.Children {
		parts[index] = m.panes(child)
	}

	if placed.Direction == "column" {
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// labels это подписи файлов для панели: имя и статус, если он не просто
// изменён. Отметки индекса нет (#99): она загромождала список.
func (m Model) labels() []string {
	labels := make([]string, len(m.files))
	for index, file := range m.files {
		labels[index] = fileTitle(file, m.text)
		if status := statusLabel(file.Status, m.text); status != "" {
			labels[index] += " " + status
		}
	}

	return labels
}

func (m Model) bodyHeight() int {
	return max(m.height-m.titleHeight()-footerHeight, 0)
}

// titleHeight это заголовок с пустой строкой под ним. Без paneTitles он
// пропадает, и тело занимает его место.
func (m Model) titleHeight() int {
	if m.ui.PaneTitles {
		return headerHeight
	}
	return 0
}

// overlay ставит окно по центру поверх base, как opencode: дифф остаётся
// виден по краям.
func overlay(base, window string, width, height int) string {
	x, y := centered(window, width, height)

	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(window).X(x).Y(y).Z(1),
	).Render()
}

// centered это левый верхний угол окна по центру области width на height.
// Координаты не уходят в минус: окно шире экрана иначе теряло бы левый край.
func centered(window string, width, height int) (x, y int) {
	return max((width-lipgloss.Width(window))/2, 0), max((height-lipgloss.Height(window))/2, 0)
}

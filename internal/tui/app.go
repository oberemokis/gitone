package tui

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/config"
	"gitone/internal/diff"
	"gitone/internal/tui/diffview"
	"gitone/internal/tui/picker"
	"gitone/internal/tui/theme"
)

// Строк за один щелчок колеса: по одной слишком медленно, трекпад шлёт
// десятки щелчков на жест.
const wheelStep = 3

type Model struct {
	// dir и args нужны для перечитывания по r: тот же git diff, что при запуске
	dir     string
	args    []string
	loading bool
	// ошибка последнего перечитывания, стоит в подвале до следующего нажатия
	notice     string
	width      int
	height     int
	helpOpen   bool
	pickerOpen bool
	picker     picker.Model
	colors     theme.Theme
	files      []diff.File
	current    int
	view       diffview.Model
}

// Options это всё, что корню нужно до запуска: каталог и аргументы для
// перечитывания по r, настройки и текст ошибки их загрузки для подвала.
type Options struct {
	Dir    string
	Args   []string
	Config config.Config
	Notice string
}

func New(opts Options, files []diff.File) Model {
	colors := theme.Dark()

	view := diffview.New(colors, diffview.Mode(opts.Config.Diff.Mode), opts.Config.Diff.AutoSplitMinWidth)
	if len(files) > 0 {
		view = view.SetFile(files[0])
	}

	return Model{
		dir:    opts.Dir,
		args:   opts.Args,
		notice: opts.Notice,
		colors: colors,
		files:  files,
		view:   view,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.view = m.view.SetSize(m.width, m.bodyHeight())
	case tea.KeyPressMsg:
		m.notice = ""
		if m.pickerOpen {
			return m.pickerKey(msg)
		}
		return m.handleAction(actionFor(msg.String()))
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelDown:
			m.view = m.view.ScrollBy(wheelStep)
		case tea.MouseWheelUp:
			m.view = m.view.ScrollBy(-wheelStep)
		}
	case reloadMsg:
		m.loading = false
		// ponytail: окно поиска, открытое во время загрузки, покажет список
		// до перечитывания. Открыть его заново обновит список.
		if msg.err != nil {
			m.notice = msg.err.Error()
		} else {
			m = m.applyReload(msg.files)
		}
	}

	return m, nil
}

func (m Model) handleAction(action action) (tea.Model, tea.Cmd) {
	switch action {
	case actionQuit:
		return m, tea.Quit
	case actionHelp:
		m.helpOpen = !m.helpOpen
	case actionCloseHelp:
		m.helpOpen = false
	case actionFiles:
		m = m.openPicker()
	case actionNextFile:
		m = m.openFile(m.current + 1)
	case actionPrevFile:
		m = m.openFile(m.current - 1)
	case actionMode:
		m.view = m.view.ToggleMode()
	case actionReload:
		if m.loading {
			return m, nil
		}
		m.loading = true
		return m, reload(m.dir, m.args)
	case actionDown:
		m.view = m.view.ScrollBy(1)
	case actionUp:
		m.view = m.view.ScrollBy(-1)
	case actionHalfDown:
		m.view = m.view.ScrollBy(m.bodyHeight() / 2)
	case actionHalfUp:
		m.view = m.view.ScrollBy(-m.bodyHeight() / 2)
	case actionTop:
		m.view = m.view.Top()
	case actionBottom:
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
		m.pickerOpen = false
		m = m.openFile(m.picker.Selected())
	case picker.Closed:
		m.pickerOpen = false
	}

	return m, nil
}

// applyReload ставит новый дифф на место старого. Открытый файл остаётся
// открытым, если он ещё в диффе, иначе открывается первый.
func (m Model) applyReload(files []diff.File) Model {
	kept := -1
	if len(m.files) > 0 {
		title := fileTitle(m.files[m.current])
		kept = slices.IndexFunc(files, func(file diff.File) bool {
			return fileTitle(file) == title
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

func (m Model) openPicker() Model {
	if len(m.files) == 0 {
		return m
	}

	items := make([]picker.Item, len(m.files))
	for index, file := range m.files {
		items[index] = picker.Item{Label: fileTitle(file), Detail: statusLabel(file.Status)}
	}

	m.picker = picker.New("Files", items, m.colors)
	m.pickerOpen = true
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

	// до первого WindowSizeMsg ширина неизвестна, раскладывать не по чему
	if m.width == 0 {
		return view
	}

	title, counter := "", ""
	if len(m.files) > 0 {
		title = header(m.width, m.files[m.current], m.colors)
		counter = fmt.Sprintf("(%d/%d)", m.current+1, len(m.files))
	}

	view.Content = lipgloss.JoinVertical(lipgloss.Left, title, "", m.body(), m.footerLine(counter))
	return view
}

// footerLine показывает подсказки, а пока идёт перечитывание или висит ошибка
// git, вместо них одну строку статуса.
func (m Model) footerLine(counter string) string {
	switch {
	case m.loading:
		return statusFooter(m.width, lipgloss.NewStyle().Foreground(m.colors.TextMuted), "loading")
	case m.notice != "":
		return statusFooter(m.width, lipgloss.NewStyle().Foreground(m.colors.Error), m.notice)
	}

	return footer(m.width, m.colors, counter)
}

// body кладёт окна поверх диффа, как opencode: дифф остаётся виден по краям.
func (m Model) body() string {
	base := lipgloss.Place(m.width, m.bodyHeight(), lipgloss.Left, lipgloss.Top, m.view.View())
	if len(m.files) == 0 {
		base = lipgloss.Place(m.width, m.bodyHeight(), lipgloss.Center, lipgloss.Center, "No changes")
	}

	switch {
	case m.pickerOpen:
		return overlay(base, m.picker.View(m.width, m.bodyHeight()), m.width, m.bodyHeight())
	case m.helpOpen:
		return overlay(base, helpBox(m.colors, m.width), m.width, m.bodyHeight())
	}

	return base
}

func (m Model) bodyHeight() int {
	return max(m.height-headerHeight-footerHeight, 0)
}

// overlay ставит окно по центру поверх base. Координаты не уходят в минус:
// окно шире экрана иначе теряло бы левый край.
func overlay(base, window string, width, height int) string {
	x := max((width-lipgloss.Width(window))/2, 0)
	y := max((height-lipgloss.Height(window))/2, 0)

	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(window).X(x).Y(y).Z(1),
	).Render()
}

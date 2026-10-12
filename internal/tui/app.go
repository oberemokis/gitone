package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/diff"
	"gitone/internal/tui/diffview"
	"gitone/internal/tui/theme"
)

// Строк за один щелчок колеса: по одной слишком медленно, трекпад шлёт
// десятки щелчков на жест.
const wheelStep = 3

type Model struct {
	width    int
	height   int
	helpOpen bool
	colors   theme.Theme
	files    []diff.File
	current  int
	view     diffview.Model
}

func New(files []diff.File) Model {
	colors := theme.Dark()

	view := diffview.New(colors)
	if len(files) > 0 {
		view = view.SetFile(files[0])
	}

	return Model{colors: colors, files: files, view: view}
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
		switch actionFor(msg.String()) {
		case actionQuit:
			return m, tea.Quit
		case actionHelp:
			m.helpOpen = !m.helpOpen
		case actionCloseHelp:
			m.helpOpen = false
		case actionMode:
			m.view = m.view.ToggleMode()
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
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelDown:
			m.view = m.view.ScrollBy(wheelStep)
		case tea.MouseWheelUp:
			m.view = m.view.ScrollBy(-wheelStep)
		}
	}

	return m, nil
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

	title := ""
	if len(m.files) > 0 {
		title = header(m.width, m.files[m.current], m.colors)
	}

	view.Content = lipgloss.JoinVertical(lipgloss.Left, title, "", m.body(), footer(m.width, m.colors))
	return view
}

func (m Model) body() string {
	// ponytail: справка заменяет тело, а не ложится поверх. Поверх или нет,
	// решается в #11 вместе с окном поиска файлов
	switch {
	case m.helpOpen:
		return lipgloss.Place(m.width, m.bodyHeight(), lipgloss.Center, lipgloss.Center, helpBox(m.colors))
	case len(m.files) == 0:
		return lipgloss.Place(m.width, m.bodyHeight(), lipgloss.Center, lipgloss.Center, "No changes")
	}

	return lipgloss.PlaceVertical(m.bodyHeight(), lipgloss.Top, m.view.View())
}

func (m Model) bodyHeight() int {
	return max(m.height-headerHeight-footerHeight, 0)
}

package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const greeting = "hello world"

type Model struct {
	width    int
	height   int
	helpOpen bool
}

func New() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		switch actionFor(msg.String()) {
		case actionQuit:
			return m, tea.Quit
		case actionHelp:
			m.helpOpen = !m.helpOpen
		case actionCloseHelp:
			m.helpOpen = false
		}
	}

	return m, nil
}

func (m Model) View() tea.View {
	view := tea.NewView("")
	view.AltScreen = true

	// до первого WindowSizeMsg ширина неизвестна, раскладывать не по чему
	if m.width == 0 {
		return view
	}

	// ponytail: справка заменяет тело, а не ложится поверх. Поверх через слои
	// Lip Gloss, когда в теле появится дифф
	content := greeting
	if m.helpOpen {
		content = helpBox()
	}

	bodyHeight := max(m.height-footerHeight, 0)
	body := lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, content)

	view.Content = lipgloss.JoinVertical(lipgloss.Left, body, footer(m.width))
	return view
}

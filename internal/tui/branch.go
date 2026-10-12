package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/git"
	"gitone/internal/tui/theme"
)

// branchesMsg несёт список локальных веток или ошибку git.
type branchesMsg struct {
	branches []git.Branch
	err      error
}

func branchesCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		branches, err := git.Branches(dir)
		return branchesMsg{branches: branches, err: err}
	}
}

// switchCmd переключает ветку. Отказ git приходит ошибкой и остаётся в подвале.
func switchCmd(dir, name string) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: git.Switch(dir, name)}
	}
}

func currentIndex(branches []git.Branch) int {
	for index, branch := range branches {
		if branch.Current {
			return index
		}
	}

	return 0
}

// branchKey отдаёт клавиши окну веток: enter переключает на выбранную, esc
// закрывает. Переключение на текущую ничего не делает.
func (m Model) branchKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.branchOpen = false
	case "down", "j":
		m.branchCursor = min(m.branchCursor+1, max(len(m.branches)-1, 0))
	case "up", "k":
		m.branchCursor = max(m.branchCursor-1, 0)
	case "enter":
		if len(m.branches) == 0 {
			return m, nil
		}

		branch := m.branches[m.branchCursor]
		m.branchOpen = false
		if branch.Current {
			return m, nil
		}
		return m, switchCmd(m.dir, branch.Name)
	}

	return m, nil
}

// branchBox рисует список веток по центру. Текущая помечена словом, а не
// цветом: так видно и без цветов.
func branchBox(colors theme.Theme, branches []git.Branch, cursor, screenWidth, screenHeight int) string {
	width := max(min(60, screenWidth-8), 20)
	rows := max(min(len(branches), screenHeight-6), 1)
	start := max(0, cursor-rows+1)

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	muted := panel.Foreground(colors.TextMuted)

	lines := []string{panel.Bold(true).Foreground(colors.Accent).Render("Branches")}
	for index := start; index < len(branches) && index < start+rows; index++ {
		marker := "  "
		if index == cursor {
			marker = "› "
		}

		text := marker + branches[index].Name
		if branches[index].Current {
			text += " (текущая)"
		}
		lines = append(lines, panel.Render(ansi.Truncate(text, width, "…")))
	}

	lines = append(lines, "", muted.Render("enter переключить   esc закрыть"))
	return panel.Padding(1, 2).Width(width).Render(strings.Join(lines, "\n"))
}

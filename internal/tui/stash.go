package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/git"
	"gitone/internal/tui/theme"
)

// stashesMsg несёт список stash или ошибку git.
type stashesMsg struct {
	stashes []git.Stash
	err     error
}

func stashListCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		stashes, err := git.Stashes(dir)
		return stashesMsg{stashes: stashes, err: err}
	}
}

func stashPushCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: git.StashPush(dir)}
	}
}

func stashApplyCmd(dir, ref string) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: git.StashApply(dir, ref)}
	}
}

func stashDropCmd(dir, ref string) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: git.StashDrop(dir, ref)}
	}
}

// stashKey отдаёт клавиши окну stash. enter показывает диф stash в экране, a
// применяет его, d спрашивает удаление. Вопрос отвечается y, иначе отмена.
func (m Model) stashKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.confirmStash != "" {
		ref := m.confirmStash
		m.confirmStash = ""
		if key.String() == "y" {
			m.stashOpen = false
			return m, stashDropCmd(m.dir, ref)
		}
		return m, nil
	}

	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.stashOpen = false
	case "down", "j":
		m.stashCursor = min(m.stashCursor+1, max(len(m.stashes)-1, 0))
	case "up", "k":
		m.stashCursor = max(m.stashCursor-1, 0)
	case "enter":
		if len(m.stashes) == 0 {
			return m, nil
		}
		m.stashOpen = false
		m.args = []string{m.stashes[m.stashCursor].Ref + "^!"}
		return m.startReload()
	case "a":
		if len(m.stashes) == 0 {
			return m, nil
		}
		m.stashOpen = false
		return m, stashApplyCmd(m.dir, m.stashes[m.stashCursor].Ref)
	case "d":
		if len(m.stashes) > 0 {
			m.confirmStash = m.stashes[m.stashCursor].Ref
		}
	}

	return m, nil
}

// stashBox рисует список stash по центру. Вопрос об удалении стоит в последней
// строке окна, пока не отвечен.
func stashBox(colors theme.Theme, stashes []git.Stash, cursor int, confirm string, screenWidth, screenHeight int) string {
	width := max(min(68, screenWidth-8), 20)
	rows := max(min(len(stashes), screenHeight-7), 1)
	start := max(0, cursor-rows+1)

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	muted := panel.Foreground(colors.TextMuted)

	lines := []string{panel.Bold(true).Foreground(colors.Accent).Render("Stash")}
	if len(stashes) == 0 {
		lines = append(lines, muted.Render("Нет stash"))
	}
	for index := start; index < len(stashes) && index < start+rows; index++ {
		marker := "  "
		if index == cursor {
			marker = "› "
		}

		text := marker + stashes[index].Ref + "  " + stashes[index].Subject
		lines = append(lines, panel.Render(ansi.Truncate(text, width, "…")))
	}

	if confirm != "" {
		lines = append(lines, "", panel.Foreground(colors.Error).Render(fmt.Sprintf("удалить %s? y да, любая клавиша нет", confirm)))
	} else {
		lines = append(lines, "", muted.Render("enter диф   a применить   d удалить   esc закрыть"))
	}

	return panel.Padding(1, 2).Width(width).Render(strings.Join(lines, "\n"))
}

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

// logLimit ограничивает лог: большой репозиторий без предела грузился бы
// секундами ради списка, который всё равно не листают до конца.
const logLimit = 200

// logMsg несёт список коммитов или ошибку git.
type logMsg struct {
	entries []git.LogEntry
	err     error
}

func logCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		entries, err := git.Log(dir, logLimit)
		return logMsg{entries: entries, err: err}
	}
}

// logKey отдаёт клавиши окну лога. Enter показывает диф коммита в этом же
// экране, esc закрывает окно.
func (m Model) logKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.logOpen = false
	case "down", "j":
		m.logCursor = min(m.logCursor+1, max(len(m.logEntries)-1, 0))
	case "up", "k":
		m.logCursor = max(m.logCursor-1, 0)
	case "enter":
		if len(m.logEntries) == 0 {
			return m, nil
		}

		entry := m.logEntries[m.logCursor]
		m.logOpen = false
		if entry.Root {
			m.notice = "первый коммит не показывается"
			return m, nil
		}

		m.args = []string{entry.Hash + "^!"}
		return m.startReload()
	}

	return m, nil
}

// logBox рисует окно лога по центру. Строки обрезаются по ширине, а окно
// прокручивается так, чтобы курсор оставался видимым.
func logBox(colors theme.Theme, entries []git.LogEntry, cursor, screenWidth, screenHeight int) string {
	width := max(min(72, screenWidth-8), 20)
	rows := max(min(len(entries), screenHeight-6), 1)
	start := max(0, cursor-rows+1)

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	accent := panel.Foreground(colors.Accent)
	muted := panel.Foreground(colors.TextMuted)

	lines := []string{accent.Bold(true).Render("Log")}
	if len(entries) == 0 {
		lines = append(lines, "", muted.Render("No commits"))
	}

	for index := start; index < len(entries) && index < start+rows; index++ {
		entry := entries[index]
		marker := "  "
		if index == cursor {
			marker = "› "
		}

		text := fmt.Sprintf("%s%s  %s  %s · %s", marker, entry.Hash[:7], entry.Subject, entry.Author, entry.When)
		lines = append(lines, panel.Render(ansi.Truncate(text, width, "…")))
	}

	lines = append(lines, "", muted.Render("enter диф коммита   esc закрыть"))
	return panel.Padding(1, 2).Width(width).Render(strings.Join(lines, "\n"))
}

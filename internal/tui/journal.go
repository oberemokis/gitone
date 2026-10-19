package tui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// journalMsg несёт записи журнала команд git (#247).
type journalMsg struct {
	entries []git.JournalEntry
}

// journalCmd читает журнал в команде: журнал общий для горутин команд git и
// живёт под мьютексом, в Update ему не место.
func journalCmd() tea.Cmd {
	return func() tea.Msg {
		return journalMsg{entries: git.Journal()}
	}
}

// journalArrived открывает окно журнала, как журнал команд lazygit: окном,
// а не в подвале, где мало места для списка.
func (m Model) journalArrived(msg journalMsg) Model {
	if m.active != noWindow {
		return m
	}
	m.journal, m.cursor, m.active = msg.entries, 0, journalWindow
	return m
}

// journalKey отдаёт окну журнала навигацию и закрытие, своих действий у
// него нет.
func (m Model) journalKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	next, cmd, _ := m.navigate(key, len(m.journal))
	return next, cmd
}

// journalBox рисует журнал от новых записей к старым: время приглушённое,
// команда целиком, отказ с кодом выхода цветом ошибки.
func journalBox(p paint, entries []git.JournalEntry, cursor int, screenWidth, screenHeight int) string {
	box := list{
		title:  p.text.Text("journal.title", nil),
		close:  p.close(keymap.ListClose),
		cursor: cursor,
		empty:  []string{p.text.Text("journal.empty", nil)},
		fill:   true,
	}

	panel, muted := p.panel(), p.muted()
	failed := panel.Foreground(p.colors.Error)

	box.rows = make([]string, len(entries))
	for index, entry := range entries {
		exit := ""
		if entry.Exit != 0 {
			exit = p.text.Text("journal.exit", i18n.Args{"code": strconv.Itoa(entry.Exit)})
		}
		command := "git " + strings.Join(entry.Args, " ")
		widths := []int{len("15:04:05"), max(box.rowWidth(screenWidth)-len("15:04:05")-len(exit)-2*columnGap, 1), len(exit)}
		box.rows[index] = tableRow(panel, widths, []lipgloss.Style{muted, panel, failed}, entry.When.Format("15:04:05"), command, exit)
	}

	return box.view(p, screenWidth, screenHeight)
}

package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

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
	if next, cmd, handled := m.navigate(key, len(m.logEntries)); handled {
		return next, cmd
	}
	if key.String() != "enter" || len(m.logEntries) == 0 {
		return m, nil
	}

	entry := m.logEntries[m.cursor]
	m.active = noWindow
	if entry.Root {
		m.notice = "первый коммит не показывается"
		return m, nil
	}

	m.args = []string{entry.Hash + "^!"}
	return m.startReload()
}

func logBox(colors theme.Theme, entries []git.LogEntry, cursor, screenWidth, screenHeight int) string {
	rows := make([]string, len(entries))
	for index, entry := range entries {
		rows[index] = fmt.Sprintf("%s  %s  %s · %s", entry.Hash[:7], entry.Subject, entry.Author, entry.When)
	}

	return list{
		title:  "Log",
		width:  72,
		rows:   rows,
		cursor: cursor,
		empty:  []string{"", "No commits"},
		hint:   "enter диф коммита   esc закрыть",
	}.view(colors, screenWidth, screenHeight)
}

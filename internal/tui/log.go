package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
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

// logKey отдаёт клавиши окну лога. log.open показывает диф коммита в этом же
// экране, list.close закрывает окно.
func (m Model) logKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.navigate(key, len(m.logEntries)); handled {
		return next, cmd
	}
	if !m.keys.Is(keymap.LogOpen, key.String()) || len(m.logEntries) == 0 {
		return m, nil
	}

	entry := m.logEntries[m.cursor]
	m.active = noWindow
	if entry.Root {
		m.notice = m.text.Text("log.firstCommitHidden", nil)
		return m, nil
	}

	m.args = []string{entry.Hash + "^!"}
	return m.startReload()
}

// logBox рисует лог на весь экран с полями: темы коммитов длинные, и узкое
// окно их обрезало. Хэш выделен цветом accent, по нему коммит ищут глазами.
// Каждый кусок строки несёт фон окна сам: вложенный стиль со сбросом иначе
// погасил бы фон после хэша.
func logBox(colors theme.Theme, text i18n.Dictionary, keys keymap.Keymap, entries []git.LogEntry, cursor, screenWidth, screenHeight int) string {
	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	hash := panel.Foreground(colors.Accent)

	rows := make([]string, len(entries))
	for index, entry := range entries {
		rows[index] = hash.Render(entry.Hash[:7]) + panel.Render(fmt.Sprintf("  %s  %s · %s", entry.Subject, entry.Author, entry.When))
	}

	return list{
		title:  text.Text("log.title", nil),
		rows:   rows,
		cursor: cursor,
		empty:  []string{"", text.Text("log.empty", nil)},
		hint:   windowHint(keys, text, keymap.LogOpen, keymap.ListClose),
		fill:   true,
	}.view(colors, screenWidth, screenHeight)
}

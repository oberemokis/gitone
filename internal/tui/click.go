package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/keymap"
	"gitone/internal/tui/picker"
	"gitone/internal/tui/zone"
)

// Цели мест клика, кроме строк списков (zone.Row): подсказка нажимает
// клавишу действия, строка справки закрывает справку и делает своё действие.
const (
	pressTarget = "press:"
	helpTarget  = "help:"
)

// click делает то, что лежит под мышью (#119). Место ищется по тому же
// рисунку, что выводит View, поэтому клик не расходится с экраном.
func (m Model) click(x, y int) (tea.Model, tea.Cmd) {
	_, zones := m.screen()
	target := zone.At(zones, x, y)

	if index, found := zone.RowIndex(target); found {
		return m.clickRow(index)
	}
	if action, found := strings.CutPrefix(target, pressTarget); found {
		return m.press(keymap.Action(action))
	}
	if action, found := strings.CutPrefix(target, helpTarget); found {
		m.helpOpen = false
		return m.press(keymap.Action(action))
	}
	if target == picker.CloseTarget {
		return m.press(keymap.SearchClose)
	}

	return m, nil
}

// press нажимает первую клавишу действия: клик идёт тем же путём, что
// клавиатура, и попадает в то окно, которое открыто. Событие несёт клавишу
// текстом, его String() совпадает с привязкой из keymap.
func (m Model) press(action keymap.Action) (tea.Model, tea.Cmd) {
	keys := m.keys.KeysFor(action)
	if len(keys) == 0 {
		return m, nil
	}

	return m.keyPress(tea.KeyPressMsg{Text: keys[0]})
}

// clickRow ставит курсор на строку и сразу делает enter окна: пользователь
// выбрал действие с первого клика. Без окна строки есть только у панели
// файлов. Строка соединения графа в логе коммитом не является.
func (m Model) clickRow(index int) (tea.Model, tea.Cmd) {
	switch m.active {
	case noWindow:
		return m.openFile(index), nil
	case pickerWindow:
		m.picker = m.picker.Select(index)
		return m.press(keymap.SearchChoose)
	case logWindow:
		if m.logShown()[index].Hash == "" {
			return m, nil
		}
		m.cursor = index
		return m.press(keymap.LogOpen)
	case branchWindow:
		m.cursor = index
		return m.press(keymap.BranchesSwitch)
	case worktreeWindow:
		m.cursor = index
		return m.press(keymap.WorktreesSwitch)
	case stashWindow:
		m.cursor = index
		return m.press(keymap.StashDiff)
	}

	return m, nil
}

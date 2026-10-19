package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/keymap"
)

// menu это выбор одного варианта из нескольких поверх окна или главного экрана
// (#236), как меню lazygit: параметры stash, режим reset. Строки выбираются
// курсором и menu.choose, клавиш у строк нет: все клавиши окон переназначаются
// через keybinds.json, а у строк меню переменный состав. У меню свой курсор:
// окно под ним, например stash, своего не теряет. back это окно, куда меню
// возвращает по list.close и перед выбором.
type menu struct {
	title  string
	items  []menuItem
	cursor int
	back   window
}

// menuItem это строка меню и то, что делает её выбор.
type menuItem struct {
	label string
	run   func(m Model) (tea.Model, tea.Cmd)
}

// openMenu открывает меню с заголовком title поверх того, что на экране.
func (m Model) openMenu(title string, items []menuItem) Model {
	m.menu = menu{title: title, items: items, back: m.active}
	m.active = menuWindow
	m.helpOpen = false
	return m
}

// menuKey отдаёт клавиши меню: list.up и list.down двигают курсор, menu.choose
// выбирает строку, list.close возвращает в прежнее окно. ctrl+c зашит.
func (m Model) menuKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	pressed := key.String()
	switch {
	case pressed == "ctrl+c":
		return m, tea.Quit
	case m.keys.Is(keymap.ListClose, pressed):
		m.active = m.menu.back
	case m.keys.Is(keymap.ListDown, pressed):
		m.menu = m.menu.move(1)
	case m.keys.Is(keymap.ListUp, pressed):
		m.menu = m.menu.move(-1)
	case m.keys.Is(keymap.MenuChoose, pressed):
		m.active = m.menu.back
		return m.menu.items[m.menu.cursor].run(m)
	}

	return m, nil
}

// move сдвигает курсор меню на delta строк, не дальше краёв.
func (mn menu) move(delta int) menu {
	mn.cursor = max(min(mn.cursor+delta, len(mn.items)-1), 0)
	return mn
}

// menuBox рисует меню общим списком: строки по подписям, подсказка выбора.
func menuBox(p paint, mn menu, screenWidth, screenHeight int) string {
	rows := make([]string, len(mn.items))
	for index, item := range mn.items {
		rows[index] = item.label
	}

	return list{
		title:  mn.title,
		close:  p.close(keymap.ListClose),
		width:  60,
		rows:   rows,
		cursor: mn.cursor,
		hint:   p.hint(keymap.MenuChoose),
	}.view(p, screenWidth, screenHeight)
}

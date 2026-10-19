package tui

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/keymap"
	"gitone/internal/tui/zone"
)

// Цели мест клика, кроме строк списков (zone.Row): подсказка нажимает
// клавишу действия, строка справки закрывает справку и делает своё действие,
// стрелка заголовка открывает файл по номеру.
const (
	pressTarget = "press:"
	helpTarget  = "help:"
	fileTarget  = "file:"
)

// Двойной клик по строке окна закрывает окно первым кликом, а вторым попадает
// в панель под ним (#150). Клик в ту же клетку быстрее этого времени после
// клика, закрывшего окно, отбрасывается.
const doubleClick = 400 * time.Millisecond

// closedClick это клик, закрывший окно: когда и в какой клетке.
type closedClick struct {
	at   time.Time
	x, y int
}

// click принимает клик, если ему есть куда попасть, и помнит тот, что закрыл
// окно. Вопрос с главного экрана клики не принимает: справка под ним видна,
// но из неё нельзя безопасно выполнить действие, а ответ на вопрос идёт
// клавишей.
func (m Model) click(x, y int) (tea.Model, tea.Cmd) {
	if m.footerQuestion() || m.repeatsClosing(x, y) {
		return m, nil
	}

	next, cmd := m.clickZone(x, y)
	if after, isModel := next.(Model); isModel && m.active != noWindow && after.active == noWindow {
		after.closing = closedClick{at: m.now(), x: x, y: y}
		return after, cmd
	}
	return next, cmd
}

// repeatsClosing говорит, что клик в клетку x, y повторяет только что
// закрывший окно клик.
func (m Model) repeatsClosing(x, y int) bool {
	return x == m.closing.x && y == m.closing.y && m.now().Sub(m.closing.at) < doubleClick
}

// clickZone делает то, что лежит под мышью (#119). Место ищется по тому же
// рисунку, что выводит View, поэтому клик не расходится с экраном.
func (m Model) clickZone(x, y int) (tea.Model, tea.Cmd) {
	_, zones := m.screen()
	target := zone.At(zones, x, y, button)

	if index, found := zone.RowIndex(target); found {
		return m.clickRow(index)
	}
	if number, found := strings.CutPrefix(target, fileTarget); found {
		index, err := strconv.Atoi(number)
		if err != nil {
			return m, nil
		}
		return m.openFile(index), nil
	}
	if action, found := strings.CutPrefix(target, pressTarget); found {
		return m.press(keymap.Action(action))
	}
	if action, found := strings.CutPrefix(target, helpTarget); found {
		return m.clickHelp(keymap.Action(action))
	}
	if target == filesEdgeTarget || target == splitEdgeTarget {
		return m.startDrag(target, x), nil
	}

	return m, nil
}

// button говорит, что цель это кнопка: подсказка или стрелка. Кнопка ловит
// клик и на клетку вокруг себя (#134), строки списков нет: клик над первой
// строкой открывал бы то, чего не выбирали.
func button(target string) bool {
	return strings.HasPrefix(target, pressTarget) || strings.HasPrefix(target, fileTarget)
}

// clickHelp закрывает справку и делает действие строки. Действие идёт
// напрямую, без клавиши: справка лежит только на экране без окон (вопрос
// discard клики не принимает), и клавиша ушла бы в чужой контекст. Строка
// app.help только закрывает справку, иначе действие открыло бы её снова.
func (m Model) clickHelp(action keymap.Action) (tea.Model, tea.Cmd) {
	m.helpOpen = false
	if action == keymap.AppHelp {
		return m, nil
	}

	m.notice = ""
	return m.handleAction(action)
}

// press нажимает первую клавишу действия: клик идёт тем же путём, что
// клавиатура, и попадает в то окно, которое открыто. Событие несёт клавишу
// текстом, его String() совпадает с привязкой из keymap. Пока в окне набирается
// поиск, клавиша ушла бы в запрос, поэтому набор сначала заканчивается; свои
// подсказки поля (listSearch.done и listSearch.cancel) набор не прерывают.
func (m Model) press(action keymap.Action) (tea.Model, tea.Cmd) {
	keys := m.keys.KeysFor(action)
	if len(keys) == 0 {
		return m, nil
	}

	if action != keymap.ListSearchDone && action != keymap.ListSearchCancel {
		m.searching = false
	}
	return m.keyPress(tea.KeyPressMsg{Text: keys[0]})
}

// clickRow ставит курсор на строку и сразу делает enter окна (windowKind.enter):
// пользователь выбрал действие с первого клика. Без окна строки есть только у
// панели файлов.
func (m Model) clickRow(index int) (tea.Model, tea.Cmd) {
	if m.active == noWindow {
		return m.openFile(index), nil
	}

	kind := windowKinds[m.active]
	m, picked := kind.pickRow(m, index)
	if !picked || kind.enter == "" {
		return m, nil
	}
	return m.press(kind.enter)
}

package tui

import (
	"maps"
	"reflect"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Автообновление как в lazygit: файлы не слушаются, репозиторий опрашивается
// по таймеру. Слежение через fsnotify в большом репозитории упиралось в лимит
// дескрипторов и не видело git add из другого терминала (решение в #81).

// tickMsg приходит от таймера опроса.
type tickMsg struct{}

// polledMsg несёт итог опроса и аргументы, с которыми он читал дифф.
type polledMsg struct {
	reloadMsg
	args []string
}

// waitTick ждёт один тик. Без таймера ждать нечего, и команды нет. Ожидание
// возобновляется после каждого тика, иначе второй тик не дошёл бы до модели.
func waitTick(ticks <-chan time.Time) tea.Cmd {
	if ticks == nil {
		return nil
	}

	return func() tea.Msg {
		if _, ok := <-ticks; !ok {
			return nil
		}
		return tickMsg{}
	}
}

// ticked опрашивает репозиторий и ждёт следующий тик.
func (m Model) ticked() (tea.Model, tea.Cmd) {
	m, pollCmd := m.poll()
	return m, tea.Batch(pollCmd, waitTick(m.ticks))
}

// poll читает дифф без следа на экране: подвал не мигает, вопрос discard не
// закрывается. Пока идёт опрос или перечитывание по r, второй не нужен:
// следующий тик всё равно придёт.
func (m Model) poll() (Model, tea.Cmd) {
	if m.loading || m.polling {
		return m, nil
	}

	m.polling = true
	dir, args := m.dir, m.args
	return m, func() tea.Msg {
		return polledMsg{reloadMsg: readDiff(dir, args), args: args}
	}
}

// polled ставит итог опроса. Ошибку покажет r: опрос повторял бы её каждые
// две секунды. Опрос со старыми аргументами (пока он шёл, открыли коммит из
// лога) и одинаковый дифф экран не трогают, подсветка не пересчитывается
// впустую.
func (m Model) polled(msg polledMsg) Model {
	m.polling = false
	if msg.err != nil || m.loading || !slices.Equal(msg.args, m.args) {
		return m
	}

	m.location = msg.location
	if reflect.DeepEqual(msg.files, m.files) && maps.Equal(msg.staged, m.staged) {
		return m
	}

	m = m.applyReload(msg.files)
	m.staged = msg.staged
	return m
}

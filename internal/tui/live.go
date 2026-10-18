package tui

import (
	"maps"
	"reflect"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Автообновление как в lazygit: файлы не слушаются, репозиторий опрашивается
// по таймеру. Слежение через fsnotify в большом репозитории упиралось в лимит
// дескрипторов и не видело git add из другого терминала (решение в #81).

// tickMsg приходит от таймера опроса.
type tickMsg struct{}

// polledMsg несёт итог опроса: тот же ответ, что у перечитывания, но экран его
// принимает тихо.
type polledMsg struct {
	reloadMsg
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
	dir, args, flags := m.dir, m.args, m.diffFlags()
	return m, func() tea.Msg {
		return polledMsg{readDiff(dir, args, flags)}
	}
}

// polled ставит итог опроса. Ошибку покажет r: опрос повторял бы её каждые
// две секунды. Опрос со старым каталогом или аргументами (пока он шёл,
// переключили worktree или открыли коммит из лога) и одинаковый дифф экран не
// трогают, подсветка не пересчитывается впустую.
func (m Model) polled(msg polledMsg) (Model, tea.Cmd) {
	m.polling = false
	if msg.err != nil || m.loading || !msg.fits(m) {
		return m, nil
	}

	m.location = msg.location
	if reflect.DeepEqual(msg.files, m.files) && maps.Equal(msg.staged, m.staged) && maps.Equal(msg.partial, m.partial) {
		return m, nil
	}

	m = m.applyReload(msg.files)
	m.staged, m.partial = msg.staged, msg.partial
	return m.refreshSelection()
}

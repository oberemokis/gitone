package tui

import tea "charm.land/bubbletea/v2"

// changedMsg приходит, когда в рабочей копии что-то изменилось: канал слежения
// прислал сигнал.
type changedMsg struct{}

// watchStoppedMsg приходит, когда слежение остановилось и канал закрыт.
type watchStoppedMsg struct{}

// waitChange ждёт один сигнал. Без канала слежения ждать нечего, и команды нет.
// Ожидание возобновляется после каждого сигнала, иначе второе изменение не
// дошло бы до экрана.
func waitChange(changes <-chan struct{}) tea.Cmd {
	if changes == nil {
		return nil
	}

	return func() tea.Msg {
		if _, ok := <-changes; !ok {
			return watchStoppedMsg{}
		}
		return changedMsg{}
	}
}

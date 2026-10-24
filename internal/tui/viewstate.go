package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/config"
)

// Вид, который меняют клавиши m, w, f, ctrl+w, } и {, переживает выход (#369):
// Update пишет его в state.json при каждой смене, New ставит запомненный при
// запуске. Ширина панели файлов и доля половин split живут до выхода.

// startView это вид при запуске: запомненный в state.json, а без него
// diff.mode и diff.wrap из config.json и флаги git по умолчанию.
func (opts Options) startView() config.View {
	if !opts.View.Saved.IsZero() {
		return opts.View
	}
	return config.View{Mode: opts.Config.Diff.Mode, Wrap: opts.Config.Diff.Wrap, Context: -1}
}

// DiffFlags это флаги git diff вида при запуске: первый дифф грузится с ними
// так же, как потом перечитывание.
func (opts Options) DiffFlags() []string {
	return flagsOf(opts.startView())
}

// viewState это вид на экране в той форме, в какой его хранит state.json.
func (m Model) viewState() config.View {
	return config.View{
		Mode:             string(m.view.Mode()),
		Wrap:             m.view.Wrapping(),
		Files:            m.filesOpen,
		IgnoreWhitespace: m.ignoreSpace,
		Context:          m.context,
	}
}

// wantSave дописывает к cmd запись вида в state.json, если вид сменился с
// before. Без каталога состояния (тесты) записывать некуда. Ошибка записи
// интерфейс не останавливает: запомненный вид это удобство.
func (m Model) wantSave(before config.View, cmd tea.Cmd) tea.Cmd {
	view := m.viewState()
	if m.stateDir == "" || view == before {
		return cmd
	}

	dir := m.stateDir
	return tea.Batch(cmd, func() tea.Msg {
		_ = config.SaveView(dir, view)
		return nil
	})
}

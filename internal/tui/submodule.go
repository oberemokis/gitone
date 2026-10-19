package tui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/diff"
	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// submodulesMsg несёт список подмодулей или ошибку git.
type submodulesMsg struct {
	submodules []git.Submodule
	err        error
}

func submodulesCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		submodules, err := git.Submodules(dir)
		return submodulesMsg{submodules: submodules, err: err}
	}
}

// submodulesArrived открывает окно подмодулей или обновляет открытое.
func (m Model) submodulesArrived(msg submodulesMsg) Model {
	switch {
	case msg.err != nil:
		return m.say(m.text.Error(msg.err))
	case m.active == noWindow:
		m.submodules, m.cursor, m.active = msg.submodules, 0, submoduleWindow
	case m.active == submoduleWindow:
		m.submodules = msg.submodules
		m = m.moveCursor(0, len(m.submodules))
	}
	return m
}

// submoduleActions это действия окна подмодулей (#244): submodules.update
// клонирует подмодуль, если его нет, и ставит записанный коммит.
var submoduleActions = []windowAction[git.Submodule]{
	{action: keymap.SubmodulesUpdate, changesRepo: true, run: func(m Model, submodule git.Submodule) (tea.Model, tea.Cmd) {
		return m.startSubmoduleUpdate(submodule.Path)
	}},
}

var submoduleHint = actionsOf(submoduleActions)

func (m Model) submoduleKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.navigate(key, len(m.submodules)); handled {
		return next, cmd
	}
	return dispatch(m, submoduleActions, key.String(), m.submodules)
}

// startSubmoduleUpdate обновляет подмодуль тем же фоновым путём, что push:
// клон идёт по сети, со спиннером и отменой по esc.
func (m Model) startSubmoduleUpdate(path string) (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	m.remote = remote{kind: updatingSubmodule, id: m.remote.id + 1, cancel: cancel}

	dir, id := m.dir, m.remote.id
	return m, tea.Batch(remoteSpinTick(id), func() tea.Msg {
		return remoteDoneMsg{id: id, kind: updatingSubmodule, target: path, err: git.UpdateSubmodule(ctx, dir, path)}
	})
}

// submoduleBox рисует подмодули таблицей: путь цветом accent, короткий
// коммит, адрес и состояние, приглушённые. Не клонированный подмодуль виден
// с пометкой, а не пропадает.
func submoduleBox(p paint, submodules []git.Submodule, cursor int, screenWidth, screenHeight int) string {
	box := list{
		title:  p.text.Text("submodules.title", nil),
		close:  p.close(keymap.ListClose),
		width:  100,
		cursor: cursor,
		empty:  []string{p.text.Text("submodules.empty", nil)},
		hint:   p.hint(submoduleHint...),
	}

	panel, muted := p.panel(), p.muted()

	states := make([]string, len(submodules))
	titles := []string{p.text.Text("submodules.column.path", nil), p.text.Text("log.column.commit", nil), p.text.Text("submodules.column.url", nil), p.text.Text("submodules.column.state", nil)}
	pathWidth, stateWidth := ansi.StringWidth(titles[0]), ansi.StringWidth(titles[3])
	for index, submodule := range submodules {
		states[index] = submoduleState(p.text, submodule)
		pathWidth = max(pathWidth, ansi.StringWidth(visible(submodule.Path)))
		stateWidth = max(stateWidth, ansi.StringWidth(states[index]))
	}
	widths := []int{min(pathWidth, maxNameColumn), hashColumn, 0, stateWidth}
	widths[2] = max(box.rowWidth(screenWidth)-widths[0]-widths[1]-widths[3]-3*columnGap, 1)

	box.header = tableRow(panel, widths, []lipgloss.Style{muted, muted, muted, muted}, titles...)

	styles := []lipgloss.Style{panel.Foreground(p.colors.Accent), muted, muted, muted}
	box.rows = make([]string, len(submodules))
	for index, submodule := range submodules {
		box.rows[index] = tableRow(panel, widths, styles, submodule.Path, submodule.Commit[:min(len(submodule.Commit), hashColumn)], submodule.URL, states[index])
	}

	return box.view(p, screenWidth, screenHeight)
}

// submoduleState это состояние подмодуля словом: не клонирован, стоит на
// другом коммите или пусто, если всё совпадает.
func submoduleState(text i18n.Dictionary, submodule git.Submodule) string {
	switch {
	case !submodule.Initialized:
		return text.Text("submodules.uninitialized", nil)
	case submodule.Moved:
		return text.Text("submodules.moved", nil)
	}
	return ""
}

// isSubmodule говорит, что запись диффа это подмодуль (#244): git с
// --submodule=short показывает его строками «Subproject commit <хэш>».
func isSubmodule(file diff.File) bool {
	if len(file.Hunks) == 0 {
		return false
	}
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if !strings.HasPrefix(line.Text, "Subproject commit ") {
				return false
			}
		}
	}
	return true
}

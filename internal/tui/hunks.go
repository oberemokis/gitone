package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/diff"
	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// unstagedMsg несёт дифф файла path между индексом и рабочим деревом для
// выделения (#212). refresh значит перечитывание открытого выделения после
// правки или опроса.
type unstagedMsg struct {
	path    string
	file    diff.File
	found   bool
	refresh bool
	err     error
}

func unstagedCmd(dir, path string, flags []string, refresh bool) tea.Cmd {
	return func() tea.Msg {
		text, err := git.UnstagedDiff(dir, path, flags...)
		msg := unstagedMsg{path: path, refresh: refresh, err: err}
		if files := diff.Parse(text); len(files) > 0 {
			msg.file, msg.found = files[0], true
		}
		return msg
	}
}

// startSelect включает выделение хунков (#212), как панель staging в
// lazygit: на экране вместо диффа от HEAD правки файла вне индекса, и space
// ставит выделенное из них. Повторное нажатие переключает выделение хунка и
// строк. Хунки есть только у изменённого текстового файла: новый,
// неотслеженный, удалённый и переименованный ставятся целиком.
func (m Model) startSelect() (tea.Model, tea.Cmd) {
	if m.view.Selecting() {
		m.view = m.view.ToggleLines()
		return m, nil
	}

	paths, note := m.target()
	if len(paths) == 0 {
		return m.putNotice(note, true), nil
	}
	file := m.files[m.current]
	if file.Status != diff.StatusModified || file.Binary {
		return m.putNotice(m.text.Text("select.wholeFile", i18n.Args{"key": firstKey(m.keys, keymap.FilesStage)}), true), nil
	}

	return m, unstagedCmd(m.dir, stagedKey(file), m.diffFlags(), false)
}

// unstagedArrived открывает выделение или обновляет открытое. Ответ про файл,
// который уже не открыт, отбрасывается. Когда вне индекса не осталось ничего,
// выделение снимается: ставить нечего.
func (m Model) unstagedArrived(msg unstagedMsg) Model {
	if len(m.files) == 0 || stagedKey(m.files[m.current]) != msg.path || (msg.refresh && !m.view.Selecting()) {
		return m
	}

	switch {
	case msg.err != nil:
		return m.say(m.text.Error(msg.err))
	case !msg.found || len(msg.file.Hunks) == 0:
		m.view = m.view.Deselect(m.files[m.current])
		return m.putNotice(m.text.Text("select.nothingUnstaged", nil), true)
	case msg.refresh:
		m.view = m.view.Reload(msg.file)
	default:
		m.view = m.view.Select(msg.file)
		m = m.putNotice(m.text.Text("select.started", i18n.Args{
			"move":    firstKey(m.keys, keymap.DiffScrollDown) + "/" + firstKey(m.keys, keymap.DiffScrollUp),
			"stage":   firstKey(m.keys, keymap.FilesStage),
			"discard": firstKey(m.keys, keymap.FilesDiscard),
			"select":  firstKey(m.keys, keymap.DiffSelect),
			"close":   firstKey(m.keys, keymap.HelpClose),
		}), false)
	}
	return m
}

// refreshSelection перечитывает выделение после перечитывания диффа: правка
// индекса или файла меняет то, что вне индекса. Вид только для просмотра
// (аргументы, коммит из лога) выделение снимает.
func (m Model) refreshSelection() (Model, tea.Cmd) {
	if !m.view.Selecting() || len(m.files) == 0 {
		return m, nil
	}

	file := m.files[m.current]
	if len(m.args) > 0 || file.Status != diff.StatusModified {
		m.view = m.view.Deselect(file)
		return m, nil
	}
	return m, unstagedCmd(m.dir, stagedKey(file), m.diffFlags(), true)
}

// selectionAction разбирает действия, которые при выделении работают иначе:
// diff.scrollDown и diff.scrollUp ведут курсор, files.stage ставит
// выделенное, files.discard отменяет его, help.close снимает выделение, если
// справка закрыта. handled false значит обычное действие.
func (m Model) selectionAction(action keymap.Action) (next tea.Model, cmd tea.Cmd, handled bool) {
	if !m.view.Selecting() {
		return m, nil, false
	}

	switch {
	case action == keymap.DiffScrollDown:
		m.view = m.view.MoveCursor(1)
	case action == keymap.DiffScrollUp:
		m.view = m.view.MoveCursor(-1)
	case action == keymap.FilesStage:
		next, cmd = m.stageSelection()
		return next, cmd, true
	case action == keymap.FilesDiscard:
		next, cmd = m.askDiscardSelection()
		return next, cmd, true
	case action == keymap.HelpClose && !m.helpOpen:
		return m.stopSelect(), nil, true
	default:
		return m, nil, false
	}
	return m, nil, true
}

// stopSelect снимает выделение и возвращает дифф от HEAD.
func (m Model) stopSelect() Model {
	if len(m.files) > 0 {
		m.view = m.view.Deselect(m.files[m.current])
	}
	return m
}

// stageSelection ставит выделенные строки в индекс.
func (m Model) stageSelection() (tea.Model, tea.Cmd) {
	selected := m.selectedSet()
	if len(selected) == 0 {
		return m, nil
	}

	return m, applySelection(m.dir, stagedKey(m.files[m.current]), selected, false, m.text.Text("select.staged", nil))
}

// askDiscardSelection спрашивает, прежде чем вернуть выделенные строки
// рабочего дерева к индексу (#213): отмена необратима, и вопрос тот же, что у
// файла целиком, хоть выделена одна строка.
func (m Model) askDiscardSelection() (tea.Model, tea.Cmd) {
	selected := m.selectedSet()
	if len(selected) == 0 {
		return m, nil
	}

	file := m.files[m.current]
	dir, path := m.dir, stagedKey(file)
	args := questionArgs(m.keys)
	args["file"] = fileTitle(file, m.text)
	note := m.text.Text("select.discarded", nil)
	return m.ask(m.text.Text("select.discardQuestion", args), func(m Model) (tea.Model, tea.Cmd) {
		return m, applySelection(dir, path, selected, true, note)
	}), nil
}

func (m Model) selectedSet() map[diff.LineRef]bool {
	selected := make(map[diff.LineRef]bool)
	for _, ref := range m.view.Selected() {
		selected[ref] = true
	}
	return selected
}

// applySelection ставит выделенные строки в индекс, а discard возвращает их в
// рабочем дереве к индексу обратным патчем. Патч собирается из свежего диффа
// без флагов вида: строки ищутся по номерам, и патч с полным контекстом git
// применяет надёжнее.
func applySelection(dir, path string, selected map[diff.LineRef]bool, discard bool, note string) tea.Cmd {
	return func() tea.Msg {
		text, err := git.UnstagedDiff(dir, path)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		patch := diff.Patch(text, selected, discard)
		if patch == "" {
			return actionDoneMsg{err: i18n.New("select.gone", nil)}
		}
		return actionDoneMsg{err: git.ApplyPatch(dir, patch, !discard, discard), note: note}
	}
}

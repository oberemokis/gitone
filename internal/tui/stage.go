package tui

import (
	"cmp"
	"slices"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/diff"
	"gitone/internal/git"
	"gitone/internal/keymap"
)

// actionDoneMsg несёт итог правки индекса или дерева: ошибку git либо nil.
type actionDoneMsg struct {
	err error
}

// stageCmd ставит пути в индекс или убирает их оттуда. Команда, а не Update:
// правка идёт через git и не должна блокировать экран.
func stageCmd(dir string, paths []string, unstage bool) tea.Cmd {
	return func() tea.Msg {
		if unstage {
			return actionDoneMsg{err: git.Unstage(dir, paths)}
		}
		return actionDoneMsg{err: git.Stage(dir, paths)}
	}
}

func discardCmd(dir string, paths []string) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: git.Discard(dir, paths)}
	}
}

// pathsOf возвращает пути файла: у переименования оба, у удалённого старый.
func pathsOf(file diff.File) []string {
	var paths []string
	for _, path := range []string{file.OldPath, file.NewPath} {
		if path != "" && !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}

	return paths
}

// stagedKey это путь, по которому файл ищется в индексе: новый путь, а у
// удалённого старый.
func stagedKey(file diff.File) string {
	return cmp.Or(file.NewPath, file.OldPath)
}

// toggleStage ставит текущий файл в индекс, а если он там уже есть, убирает.
func (m Model) toggleStage() (tea.Model, tea.Cmd) {
	if len(m.files) == 0 {
		return m, nil
	}

	file := m.files[m.current]
	return m, stageCmd(m.dir, pathsOf(file), m.staged[stagedKey(file)])
}

// askDiscard спрашивает в подвале, прежде чем отменять правки файла.
func (m Model) askDiscard() Model {
	if len(m.files) == 0 {
		return m
	}

	file := m.files[m.current]
	m.active = discardWindow
	m.confirm = pathsOf(file)
	args := questionArgs(m.keys)
	args["file"] = fileTitle(file, m.text)
	m.notice = m.text.Text("discard.question", args)
	return m
}

// answerDiscard разбирает ответ на вопрос. Выполняет только question.yes:
// любая другая клавиша отменяет вопрос и не выполняет действие.
func (m Model) answerDiscard(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	paths := m.confirm
	m.active = noWindow
	m.confirm = nil
	m.notice = ""

	if !m.keys.Is(keymap.QuestionYes, key.String()) {
		return m, nil
	}
	return m, discardCmd(m.dir, paths)
}

// actionDone после правки перечитывает дифф и индекс, даже если правка не
// удалась: так видно, что на самом деле осталось.
func (m Model) actionDone(msg actionDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m = m.say(m.text.Error(msg.err))
	}

	m.stale = true
	return m.startReload()
}

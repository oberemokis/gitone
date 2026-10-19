package tui

import (
	"cmp"
	"slices"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/diff"
	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// actionDoneMsg несёт итог правки индекса или дерева: ошибку git либо nil.
// note это итог успеха для подвала на языке словаря, пустой без него: после
// перемотки или rebase на экране может не измениться ничего. warning это
// предупреждение вместо итога: оно стоит до следующего нажатия, как ошибка,
// потому что говорит о последствии, которое иначе всплывёт только при push.
type actionDoneMsg struct {
	err     error
	note    string
	warning string
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

// pathsOf возвращает пути файла: у переименования оба, у удалённого старый, у
// копии только новый: у источника копии своя правка, и отмена копии её стёрла бы.
func pathsOf(file diff.File) []string {
	candidates := []string{file.OldPath, file.NewPath}
	if file.Status == diff.StatusCopied {
		candidates = candidates[1:]
	}

	var paths []string
	for _, path := range candidates {
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

// target возвращает пути текущего файла для пробела и x. Пустой список значит,
// что действия нет: тогда note объясняет почему, а без файлов он пуст. Записи
// без пути («git output») нечего ставить, а индекс и дерево меняет только дифф
// рабочего дерева: путь из чужого диффа указал бы на правку, которой на экране
// нет.
func (m Model) target() (paths []string, note string) {
	if len(m.files) == 0 {
		return nil, ""
	}

	paths = pathsOf(m.files[m.current])
	switch {
	case len(paths) == 0:
		return nil, m.text.Text("stage.noFile", nil)
	case len(m.args) > 0 && len(m.baseArgs) == 0:
		return nil, m.text.Text("stage.viewOnly", i18n.Args{"key": firstKey(m.keys, keymap.DiffWorkingTree)})
	case len(m.args) > 0:
		// b вернул бы к аргументам запуска, а это тоже вид только для просмотра:
		// совет с клавишей вёл бы в то же место
		return nil, m.text.Text("stage.viewOnlyArgs", nil)
	}

	return paths, ""
}

// toggleStage ставит текущий файл в индекс, а если он там уже есть, убирает.
func (m Model) toggleStage() (tea.Model, tea.Cmd) {
	paths, note := m.target()
	if len(paths) == 0 {
		return m.putNotice(note, true), nil
	}

	return m, stageCmd(m.dir, paths, m.index.Staged[stagedKey(m.files[m.current])])
}

// ignoreFile дописывает текущий файл в корневой .gitignore (#214), как i в
// lazygit. Отслеженный файл отвергает git-слой с причиной в подвале. После
// записи файл пропадает из диффа при перечитывании, а подвал говорит, куда он
// ушёл.
func (m Model) ignoreFile() (tea.Model, tea.Cmd) {
	paths, note := m.target()
	if len(paths) == 0 {
		return m.putNotice(note, true), nil
	}

	dir, path := m.dir, stagedKey(m.files[m.current])
	done := m.text.Text("stage.ignored", i18n.Args{"path": visible(path)})
	return m, func() tea.Msg {
		return actionDoneMsg{err: git.Ignore(dir, path), note: done}
	}
}

// askDiscard спрашивает в подвале, прежде чем отменять правки файла.
func (m Model) askDiscard() Model {
	paths, note := m.target()
	if len(paths) == 0 {
		return m.putNotice(note, true)
	}

	file := m.files[m.current]
	m.active = discardWindow
	m.confirm = paths
	args := questionArgs(m.keys)
	args["file"] = fileTitle(file, m.text)
	return m.putNotice(m.text.Text("discard.question", args), false)
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
	switch {
	case msg.err != nil:
		m = m.say(m.text.Error(msg.err))
	case msg.warning != "":
		m = m.say(msg.warning)
	case msg.note != "":
		m = m.sayCalm(msg.note)
	}

	return m.startReload()
}

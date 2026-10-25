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
// note это итог успеха для тоста на языке словаря, пустой без него: после
// перемотки или rebase на экране может не измениться ничего. warning это
// предупреждение вместо итога: его тост цвета ошибки, как у ошибки, потому
// что оно говорит о последствии, которое иначе всплывёт только при push.
type actionDoneMsg struct {
	err     error
	note    string
	warning string
}

func (msg actionDoneMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.actionDone(msg) }

// stageCmd ставит пути в индекс или убирает их оттуда. Команда, а не Update:
// правка идёт через git и не должна блокировать экран.
func stageCmd(repo git.Repo, paths []string, unstage bool) tea.Cmd {
	return func() tea.Msg {
		if unstage {
			return actionDoneMsg{err: repo.Unstage(paths)}
		}
		return actionDoneMsg{err: repo.Stage(paths)}
	}
}

func discardCmd(repo git.Repo, paths []string) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: repo.Discard(paths)}
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
		return m.putNotice(note), nil
	}

	return m, stageCmd(m.repo, paths, m.index.Staged[stagedKey(m.files[m.current])])
}

// ignoreFile дописывает текущий файл в корневой .gitignore (#214), как i в
// lazygit. Отслеженный файл отвергает git-слой с причиной в тосте. После
// записи файл пропадает из диффа при перечитывании, а тост говорит, куда он
// ушёл.
func (m Model) ignoreFile() (tea.Model, tea.Cmd) {
	paths, note := m.target()
	if len(paths) == 0 {
		return m.putNotice(note), nil
	}

	repo, path := m.repo, stagedKey(m.files[m.current])
	done := m.text.Text("stage.ignored", i18n.Args{"path": visible(path)})
	return m, func() tea.Msg {
		return actionDoneMsg{err: repo.Ignore(path), note: done}
	}
}

// askDiscard спрашивает в подвале, прежде чем отменять правки файла. Вопрос
// общий (ask): его текст рисуется из вопроса, и запоздавшая заметка его не
// затрёт, пока вопрос висит.
func (m Model) askDiscard() Model {
	paths, note := m.target()
	if len(paths) == 0 {
		return m.putNotice(note)
	}

	args := questionArgs(m.keys)
	args["file"] = fileTitle(m.files[m.current], m.text)
	return m.ask(m.text.Text("discard.question", args), func(m Model) (tea.Model, tea.Cmd) {
		return m, discardCmd(m.repo, paths)
	})
}

// conflictKeys это клавиши для совета при остановке на конфликте
// (git.stoppedOnConflict): git их не знает, их подставляет корень.
func (m Model) conflictKeys() i18n.Args {
	return i18n.Args{"select": firstKey(m.keys, keymap.DiffSelect), "next": firstKey(m.keys, keymap.GitOperation)}
}

// actionDone после правки перечитывает дифф и индекс, даже если правка не
// удалась: так видно, что на самом деле осталось.
func (m Model) actionDone(msg actionDoneMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.err != nil:
		m = m.say(m.text.ErrorWith(msg.err, m.conflictKeys()))
	case msg.warning != "":
		m = m.say(msg.warning)
	case msg.note != "":
		m = m.sayCalm(msg.note)
	}

	return m.startReload()
}

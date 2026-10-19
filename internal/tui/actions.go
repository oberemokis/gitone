package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/keymap"
	"gitone/internal/tui/diffview"
)

// actionFunc выполняет действие над моделью.
type actionFunc func(Model) (tea.Model, tea.Cmd)

// globalAction это действие главного экрана. changesRepo значит, что действие
// меняет репозиторий или само запускает pull, push или fetch. Пока идёт
// сетевая операция, такое действие отказывает: git pull меняет дерево, и x
// посреди него стёр бы не те правки. Просмотр, окна и перечитывание диффа
// работают.
type globalAction struct {
	run         actionFunc
	changesRepo bool
}

// windowAction это действие окна-списка над выбранной строкой типа T (#353).
// Таблица окна записана в порядке его подсказки: из неё идут и разбор клавиш,
// и подсказка. changesRepo значит то же, что у globalAction. anyRow значит,
// что действие работает и на пустом списке (новый тег, новый stash), тогда
// строка нулевая.
type windowAction[T any] struct {
	action      keymap.Action
	run         func(Model, T) (tea.Model, tea.Cmd)
	changesRepo bool
	anyRow      bool
}

// dispatch выполняет действие таблицы по нажатию над строкой rows[m.cursor].
// Внутри окна клавиши не конфликтуют, это проверяет keymap, поэтому порядок
// поиска поведения не задаёт. Клавиша не из таблицы ничего не делает.
func dispatch[T any](m Model, actions []windowAction[T], pressed string, rows []T) (tea.Model, tea.Cmd) {
	for _, entry := range actions {
		if !m.keys.Is(entry.action, pressed) {
			continue
		}

		var row T
		switch {
		case m.cursor < len(rows):
			row = rows[m.cursor]
		case !entry.anyRow:
			return m, nil
		}
		if entry.changesRepo {
			if next, refused := m.refuseWhileRemote(); refused {
				return next, nil
			}
		}
		return entry.run(m, row)
	}

	return m, nil
}

// actionsOf это действия таблицы в её порядке и за ними extra: так подсказка
// окна собирается из таблицы.
func actionsOf[T any](actions []windowAction[T], extra ...keymap.Action) []keymap.Action {
	listed := make([]keymap.Action, 0, len(actions)+len(extra))
	for _, entry := range actions {
		listed = append(listed, entry.action)
	}
	return append(listed, extra...)
}

// globalActions это все действия главного экрана (#350). Действие keymap без
// записи здесь роняет TestGlobalActionsComplete, а не молчит на клавише.
var globalActions = map[keymap.Action]globalAction{
	keymap.AppQuit: {run: func(m Model) (tea.Model, tea.Cmd) { return m, tea.Quit }},
	keymap.AppHelp: {run: stay(func(m Model) Model {
		m.helpOpen, m.helpTop = !m.helpOpen, 0
		return m
	})},
	// esc закрывает справку, а без неё отменяет идущий pull или push
	keymap.HelpClose: {run: stay(func(m Model) Model {
		if !m.helpOpen {
			m = m.stopRemote()
		}
		m.helpOpen = false
		return m
	})},

	keymap.GitPull:      {run: remoteAction(pulling), changesRepo: true},
	keymap.GitFetch:     {run: remoteAction(fetching), changesRepo: true},
	keymap.GitPush:      {run: remoteAction(pushing), changesRepo: true},
	keymap.GitOperation: {run: stay(Model.askOperation), changesRepo: true},
	keymap.GitUndo:      {run: inDir(func(dir string) tea.Cmd { return undoStepCmd(dir, false) }), changesRepo: true},
	keymap.GitRedo:      {run: inDir(func(dir string) tea.Cmd { return undoStepCmd(dir, true) }), changesRepo: true},

	keymap.FilesPick: {run: stay(Model.openPicker)},
	keymap.FilesToggle: {run: stay(func(m Model) Model {
		m.filesOpen = !m.filesOpen
		return m.resize()
	})},
	keymap.FilesNext:     {run: stay(func(m Model) Model { return m.openFile(m.current + 1) })},
	keymap.FilesPrevious: {run: stay(func(m Model) Model { return m.openFile(m.current - 1) })},
	keymap.FilesStage:    {run: Model.toggleStage, changesRepo: true},
	keymap.FilesDiscard:  {run: stay(Model.askDiscard), changesRepo: true},
	keymap.FilesCommit:   {run: stay(Model.openCommit), changesRepo: true},
	keymap.FilesAmend:    {run: inDir(lastMessageCmd), changesRepo: true},
	keymap.FilesHistory:  {run: Model.fileHistory},
	keymap.FilesEdit:     {run: Model.editFile},
	keymap.FilesIgnore:   {run: Model.ignoreFile, changesRepo: true},

	keymap.AppLog:        {run: inDir(func(dir string) tea.Cmd { return logCmd(dir, git.LogFilter{}) })},
	keymap.AppBranches:   {run: inDir(branchesCmd)},
	keymap.AppWorktrees:  {run: inDir(worktreesCmd)},
	keymap.AppTags:       {run: inDir(tagsCmd)},
	keymap.AppRemotes:    {run: inDir(remotesCmd)},
	keymap.AppReflog:     {run: inDir(reflogCmd)},
	keymap.AppSubmodules: {run: inDir(submodulesCmd)},
	keymap.AppJournal:    {run: func(m Model) (tea.Model, tea.Cmd) { return m, journalCmd() }},
	keymap.AppRecent:     {run: func(m Model) (tea.Model, tea.Cmd) { return m, recentCmd(m.stateDir, m.dir) }},
	keymap.StashPush:     {run: inDir(stashPushCmd), changesRepo: true},
	keymap.StashList:     {run: inDir(stashListCmd)},

	keymap.DiffToggleMode:       {run: onView(diffview.Model.ToggleMode)},
	keymap.DiffToggleWrap:       {run: onView(diffview.Model.ToggleWrap)},
	keymap.DiffToggleWhitespace: {run: Model.toggleWhitespace},
	keymap.DiffMoreContext:      {run: func(m Model) (tea.Model, tea.Cmd) { return m.changeContext(1) }},
	keymap.DiffLessContext:      {run: func(m Model) (tea.Model, tea.Cmd) { return m.changeContext(-1) }},
	keymap.DiffScrollLeft:       {run: onView(func(view diffview.Model) diffview.Model { return view.ScrollRight(-shiftStep) })},
	keymap.DiffScrollRight:      {run: onView(func(view diffview.Model) diffview.Model { return view.ScrollRight(shiftStep) })},
	keymap.DiffReload:           {run: func(m Model) (tea.Model, tea.Cmd) { return m.startReload() }},
	keymap.DiffSelect:           {run: Model.startSelect},
	keymap.DiffWorkingTree: {run: func(m Model) (tea.Model, tea.Cmd) {
		m.args = m.baseArgs
		return m.startReload()
	}},
	keymap.DiffScrollDown: {run: onView(func(view diffview.Model) diffview.Model { return view.ScrollBy(1) })},
	keymap.DiffScrollUp:   {run: onView(func(view diffview.Model) diffview.Model { return view.ScrollBy(-1) })},
	keymap.DiffHalfPageDown: {run: stay(func(m Model) Model {
		m.view = m.view.ScrollBy(m.bodyHeight() / 2)
		return m
	})},
	keymap.DiffHalfPageUp: {run: stay(func(m Model) Model {
		m.view = m.view.ScrollBy(-m.bodyHeight() / 2)
		return m
	})},
	keymap.DiffTop:    {run: onView(diffview.Model.Top)},
	keymap.DiffBottom: {run: onView(diffview.Model.Bottom)},
}

// stay это действие, которое меняет только модель, без команды.
func stay(change func(Model) Model) actionFunc {
	return func(m Model) (tea.Model, tea.Cmd) {
		return change(m), nil
	}
}

// onView это действие, которое меняет только вид диффа.
func onView(change func(diffview.Model) diffview.Model) actionFunc {
	return func(m Model) (tea.Model, tea.Cmd) {
		m.view = change(m.view)
		return m, nil
	}
}

// inDir запускает команду над каталогом репозитория.
func inDir(command func(dir string) tea.Cmd) actionFunc {
	return func(m Model) (tea.Model, tea.Cmd) {
		return m, command(m.dir)
	}
}

// remoteAction запускает pull, push или fetch в фоне.
func remoteAction(kind remoteKind) actionFunc {
	return func(m Model) (tea.Model, tea.Cmd) {
		return m.startRemote(kind)
	}
}

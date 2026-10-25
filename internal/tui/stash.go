package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// stashesMsg несёт список stash или ошибку git. edit значит, что список
// пришёл после правки, at это строка, на которую тогда встаёт курсор.
type stashesMsg struct {
	stashes []git.Stash
	err     error
	edit    bool
	at      int
}

func (msg stashesMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.stashesArrived(msg) }

func stashListCmd(repo git.Repo) tea.Cmd {
	return func() tea.Msg {
		stashes, err := repo.Stashes()
		return stashesMsg{stashes: stashes, err: err}
	}
}

// stashesArrived открывает окно stash или обновляет открытое (arrived). Список
// после нового stash или переименования ставит курсор на новую или
// переименованную запись, а дифф перечитывается: правки могли уйти из дерева.
func (m Model) stashesArrived(msg stashesMsg) (tea.Model, tea.Cmd) {
	open := m.beneath() == stashWindow
	m = m.arrived(listArrival{shown: stashWindow, err: msg.err, edit: msg.edit, set: func(m *Model) { m.stashes = msg.stashes }})
	if msg.err != nil || !msg.edit {
		return m, nil
	}

	if open {
		m.cursor = 0
		m = m.moveCursor(msg.at, len(m.stashes))
	}
	return m.startReload()
}

// stashTreeMsg несёт то, с чем сравнивать родителя stash hash в его диффе.
type stashTreeMsg struct {
	hash string
	tree string
	err  error
}

func (msg stashTreeMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.openStashDiff(msg) }

func stashTreeCmd(repo git.Repo, hash string) tea.Cmd {
	return func() tea.Msg {
		tree, err := repo.StashTree(hash)
		return stashTreeMsg{hash: hash, tree: tree, err: err}
	}
}

// openStashDiff добавляет в открытый дифф stash его неотслеженные файлы. stash
// это коммит слияния HEAD и индекса: <хэш>^! дал бы комбинированный diff --cc.
// Родитель и дерево stash с неотслеженными файлами дают обычный дифф, как git
// stash show -p -u. Хэш, а не stash@{N}: перечитывание не уйдёт на другой
// stash после внешнего git stash push. Если за это время открыли другой дифф
// или неотслеженных нет, ответ ничего не меняет.
func (m Model) openStashDiff(msg stashTreeMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.err != nil:
		return m.say(m.text.Error(msg.err)), nil
	case msg.tree == msg.hash || !slices.Equal(m.args, []string{msg.hash + "^1", msg.hash}):
		return m, nil
	}

	m.args = []string{msg.hash + "^1", msg.tree}
	return m.startReload()
}

func stashPushCmd(repo git.Repo) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: repo.StashPush(git.StashOptions{})}
	}
}

// stashNewCmd сохраняет stash с параметрами и отвечает новым списком: окно
// stash остаётся открытым, и новая запись видна сразу.
func stashNewCmd(repo git.Repo, opts git.StashOptions) tea.Cmd {
	return func() tea.Msg {
		if err := repo.StashPush(opts); err != nil {
			return stashesMsg{err: err}
		}
		msg := stashListCmd(repo)().(stashesMsg)
		msg.edit = true
		return msg
	}
}

// stashRenameCmd меняет сообщение записи и отвечает новым списком с курсором
// на ней же: порядок записей переименование не меняет.
func stashRenameCmd(repo git.Repo, stash git.Stash, at int, message string) tea.Cmd {
	return func() tea.Msg {
		if err := repo.StashRename(stash, message); err != nil {
			return stashesMsg{err: err}
		}
		msg := stashListCmd(repo)().(stashesMsg)
		msg.edit, msg.at = true, at
		return msg
	}
}

// askStashMessage спрашивает новое сообщение записи (#237), в поле стоит
// прежнее. Пустое поле не закрывается молча: Repo.StashRename отвечает
// причиной в подвале.
func (m Model) askStashMessage(stash git.Stash) Model {
	at := m.cursor
	title := m.text.Text("stash.renameTitle", i18n.Args{"ref": visible(stash.Ref)})
	m = m.openPrompt(title, stash.Subject, func(m Model, message string) (tea.Model, tea.Cmd) {
		return m, stashRenameCmd(m.repo, stash, at, message)
	})
	m.prompt.empty = true
	return m
}

// askStashOptions спрашивает, что унести в stash, а потом его сообщение (#236),
// как меню stash в lazygit. Пустое сообщение оставляет сообщение git.
func (m Model) askStashOptions() Model {
	option := func(labelKey string, opts git.StashOptions) menuItem {
		label := m.text.Text(labelKey, nil)
		return menuItem{label: label, run: func(m Model) (tea.Model, tea.Cmd) {
			m = m.openPrompt(label, "", func(m Model, message string) (tea.Model, tea.Cmd) {
				opts.Message = message
				return m, stashNewCmd(m.repo, opts)
			})
			m.prompt.empty = true
			return m, nil
		}}
	}

	return m.openMenu(m.text.Text("stash.newTitle", nil), []menuItem{
		option("stash.tracked", git.StashOptions{}),
		option("stash.untracked", git.StashOptions{Untracked: true}),
		option("stash.keepIndex", git.StashOptions{KeepIndex: true}),
	})
}

func stashApplyCmd(repo git.Repo, stash git.Stash) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: repo.StashApply(stash)}
	}
}

func stashDropCmd(repo git.Repo, stash git.Stash) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: repo.StashDrop(stash)}
	}
}

func stashPopCmd(repo git.Repo, stash git.Stash) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: repo.StashPop(stash)}
	}
}

// stashActions это действия окна stash. stash.diff показывает диф stash в
// экране, stash.apply применяет его, stash.pop применяет и удаляет (#235),
// stash.drop спрашивает удаление, stash.new сохраняет новый с параметрами
// (#236) и в пустом списке тоже.
var stashActions = []windowAction[git.Stash]{
	// дифф stash открывается сразу, а неотслеженные файлы добавит
	// openStashDiff, когда git соберёт дерево с ними
	{action: keymap.StashDiff, run: func(m Model, stash git.Stash) (tea.Model, tea.Cmd) {
		m.active = noWindow
		m.args = []string{stash.Hash + "^1", stash.Hash}
		next, reload := m.startReload()
		return next, tea.Batch(reload, stashTreeCmd(m.repo, stash.Hash))
	}},
	{action: keymap.StashApply, changesRepo: true, run: func(m Model, stash git.Stash) (tea.Model, tea.Cmd) {
		m.active = noWindow
		return m, stashApplyCmd(m.repo, stash)
	}},
	{action: keymap.StashPop, changesRepo: true, run: func(m Model, stash git.Stash) (tea.Model, tea.Cmd) {
		m.active = noWindow
		return m, stashPopCmd(m.repo, stash)
	}},
	{action: keymap.StashDrop, changesRepo: true, run: func(m Model, stash git.Stash) (tea.Model, tea.Cmd) {
		m.active = stashDropWindow
		m.dropping = stash
		return m, nil
	}},
	{action: keymap.StashRename, changesRepo: true, run: func(m Model, stash git.Stash) (tea.Model, tea.Cmd) {
		return m.askStashMessage(stash), nil
	}},
	{action: keymap.StashNew, anyRow: true, changesRepo: true, run: func(m Model, _ git.Stash) (tea.Model, tea.Cmd) {
		return m.askStashOptions(), nil
	}},
}

var stashHint = actionsOf(stashActions)

func (m Model) stashKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.navigate(key, len(m.stashes)); handled {
		return next, cmd
	}
	return dispatch(m, stashActions, key.String(), m.stashes)
}

// answerStashDrop разбирает ответ на вопрос об удалении. Удаляет только
// question.yes, любая другая клавиша возвращает к списку и сама не
// выполняется.
func (m Model) answerStashDrop(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if !m.keys.Is(keymap.QuestionYes, key.String()) {
		m.active = stashWindow
		return m, nil
	}

	m.active = noWindow
	return m, stashDropCmd(m.repo, m.droppingNow())
}

// droppingNow это stash из вопроса об удалении под его нынешним номером.
// Список, пришедший под вопросом, сдвигает номера и курсор: после нового
// stash прежний stash@{1} стал stash@{2}, и удаление по курсору унесло бы
// другую запись (#379). Запись ищется по хэшу. Если её в списке нет, остаётся
// номер из вопроса, и StashDrop откажет: ссылка указывает на другой коммит.
func (m Model) droppingNow() git.Stash {
	for _, stash := range m.stashes {
		if stash.Hash == m.dropping.Hash {
			return stash
		}
	}
	return m.dropping
}

// stashBox рисует список stash. Вопрос об удалении stash asking стоит в
// последней строке окна, пока не отвечен; без вопроса asking пустой.
func stashBox(p paint, stashes []git.Stash, cursor int, asking git.Stash, screenWidth, screenHeight int) string {
	rows := make([]string, len(stashes))
	for index, stash := range stashes {
		rows[index] = visible(stash.Ref + "  " + stash.Subject)
	}

	box := list{
		title:  p.text.Text("stash.title", nil),
		close:  p.close(keymap.ListClose),
		width:  68,
		rows:   rows,
		cursor: cursor,
		empty:  []string{p.text.Text("stash.empty", nil)},
		hint:   p.hint(stashHint...),
	}
	if asking.Ref != "" {
		args := questionArgs(p.keys)
		args["ref"] = visible(asking.Ref)
		box.question = p.text.Text("stash.dropQuestion", args)
	}

	return box.view(p, screenWidth, screenHeight)
}

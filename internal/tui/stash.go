package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
	"gitone/internal/tui/theme"
)

// stashesMsg несёт список stash или ошибку git. edit значит, что список
// пришёл после правки, at это строка, на которую тогда встаёт курсор.
type stashesMsg struct {
	stashes []git.Stash
	err     error
	edit    bool
	at      int
}

func stashListCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		stashes, err := git.Stashes(dir)
		return stashesMsg{stashes: stashes, err: err}
	}
}

func stashPushCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: git.StashPush(dir, git.StashOptions{})}
	}
}

// stashNewCmd сохраняет stash с параметрами и отвечает новым списком: окно
// stash остаётся открытым, и новая запись видна сразу.
func stashNewCmd(dir string, opts git.StashOptions) tea.Cmd {
	return func() tea.Msg {
		if err := git.StashPush(dir, opts); err != nil {
			return stashesMsg{err: err}
		}
		msg := stashListCmd(dir)().(stashesMsg)
		msg.edit = true
		return msg
	}
}

// stashRenameCmd меняет сообщение записи и отвечает новым списком с курсором
// на ней же: порядок записей переименование не меняет.
func stashRenameCmd(dir string, stash git.Stash, at int, message string) tea.Cmd {
	return func() tea.Msg {
		if err := git.StashRename(dir, stash, message); err != nil {
			return stashesMsg{err: err}
		}
		msg := stashListCmd(dir)().(stashesMsg)
		msg.edit, msg.at = true, at
		return msg
	}
}

// askStashMessage спрашивает новое сообщение записи (#237), в поле стоит
// прежнее. Пустое поле не закрывается молча: git.StashRename отвечает
// причиной в подвале.
func (m Model) askStashMessage(stash git.Stash) Model {
	at := m.cursor
	title := m.text.Text("stash.renameTitle", i18n.Args{"ref": visible(stash.Ref)})
	m = m.openPrompt(title, stash.Subject, func(m Model, message string) (tea.Model, tea.Cmd) {
		return m, stashRenameCmd(m.dir, stash, at, message)
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
				return m, stashNewCmd(m.dir, opts)
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

func stashApplyCmd(dir string, stash git.Stash) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: git.StashApply(dir, stash)}
	}
}

func stashDropCmd(dir string, stash git.Stash) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: git.StashDrop(dir, stash)}
	}
}

func stashPopCmd(dir string, stash git.Stash) tea.Cmd {
	return func() tea.Msg {
		return actionDoneMsg{err: git.StashPop(dir, stash)}
	}
}

// stashKey отдаёт клавиши окну stash. stash.diff показывает диф stash в
// экране, stash.apply применяет его, stash.pop применяет и удаляет (#235),
// stash.drop спрашивает удаление, stash.new сохраняет новый с параметрами
// (#236) и в пустом списке тоже.
func (m Model) stashKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.navigate(key, len(m.stashes)); handled {
		return next, cmd
	}
	if m.keys.Is(keymap.StashNew, key.String()) {
		if next, refused := m.refuseWhileRemote(); refused {
			return next, nil
		}
		return m.askStashOptions(), nil
	}
	if len(m.stashes) == 0 {
		return m, nil
	}

	stash := m.stashes[m.cursor]
	pressed := key.String()
	switch {
	case m.keys.Is(keymap.StashDiff, pressed):
		m.active = noWindow
		// stash это коммит слияния HEAD и индекса: <хэш>^! дал бы комбинированный
		// diff --cc. Родитель и сам stash дают обычный дифф, как git stash show -p.
		// Хэш, а не stash@{N}: перечитывание не уйдёт на другой stash после
		// внешнего git stash push.
		m.args = []string{stash.Hash + "^1", stash.Hash}
		return m.startReload()
	case m.keys.Is(keymap.StashApply, pressed):
		if next, refused := m.refuseWhileRemote(); refused {
			return next, nil
		}
		m.active = noWindow
		return m, stashApplyCmd(m.dir, stash)
	case m.keys.Is(keymap.StashPop, pressed):
		if next, refused := m.refuseWhileRemote(); refused {
			return next, nil
		}
		m.active = noWindow
		return m, stashPopCmd(m.dir, stash)
	case m.keys.Is(keymap.StashDrop, pressed):
		if next, refused := m.refuseWhileRemote(); refused {
			return next, nil
		}
		m.active = stashDropWindow
	case m.keys.Is(keymap.StashRename, pressed):
		if next, refused := m.refuseWhileRemote(); refused {
			return next, nil
		}
		return m.askStashMessage(stash), nil
	}

	return m, nil
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
	return m, stashDropCmd(m.dir, m.stashes[m.cursor])
}

// stashBox рисует список stash. Вопрос об удалении стоит в последней строке
// окна, пока не отвечен.
func stashBox(colors theme.Theme, text i18n.Dictionary, keys keymap.Keymap, stashes []git.Stash, cursor int, asking bool, screenWidth, screenHeight int) string {
	rows := make([]string, len(stashes))
	for index, stash := range stashes {
		rows[index] = visible(stash.Ref + "  " + stash.Subject)
	}

	box := list{
		title:  text.Text("stash.title", nil),
		close:  closeHint(colors, keys, text, keymap.ListClose),
		width:  68,
		rows:   rows,
		cursor: cursor,
		empty:  []string{text.Text("stash.empty", nil)},
		hint:   windowHint(colors, keys, text, keymap.StashDiff, keymap.StashApply, keymap.StashPop, keymap.StashDrop, keymap.StashRename, keymap.StashNew),
	}
	if asking {
		args := questionArgs(keys)
		args["ref"] = visible(stashes[cursor].Ref)
		box.question = text.Text("stash.dropQuestion", args)
	}

	return box.view(colors, screenWidth, screenHeight)
}

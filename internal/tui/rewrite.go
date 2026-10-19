package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// rewrittenMsg несёт итог правки истории (#219): ошибку git, переписаны ли
// коммиты из upstream, итог для подвала и строку лога, на которую встанет
// курсор, если лог ещё открыт.
type rewrittenMsg struct {
	err       error
	published bool
	note      string
	at        int
}

// rewriteKeys это клавиши правки истории в логе и что они делают.
var rewriteKeys = []struct {
	action keymap.Action
	change git.Rewrite
}{
	{keymap.LogSquash, git.RewriteSquash},
	{keymap.LogFixup, git.RewriteFixup},
	{keymap.LogDrop, git.RewriteDrop},
	{keymap.LogReword, git.RewriteReword},
	{keymap.LogMoveUp, git.RewriteMoveUp},
	{keymap.LogMoveDown, git.RewriteMoveDown},
}

// rewriteKey разбирает клавиши правки истории в логе, как в lazygit: squash и
// fixup в предыдущий коммит, drop после вопроса, reword через окно коммита,
// перенос вверх и вниз. handled false значит, что клавиша не про историю.
func (m Model) rewriteKey(pressed string, entry git.LogEntry) (next tea.Model, cmd tea.Cmd, handled bool) {
	for _, key := range rewriteKeys {
		if !m.keys.Is(key.action, pressed) {
			continue
		}
		if refused, stop := m.refuseWhileRemote(); stop {
			return refused, nil, true
		}

		short := entry.Hash[:7]
		switch key.change {
		case git.RewriteDrop:
			args := questionArgs(m.keys)
			args["hash"] = short
			return m.ask(m.text.Text("rewrite.dropQuestion", args), func(m Model) (tea.Model, tea.Cmd) {
				return m, m.rewrite(entry.Hash, git.RewriteDrop, "", m.cursor)
			}), nil, true
		case git.RewriteReword:
			return m, rewordMessageCmd(m.dir, entry.Hash), true
		}

		at := m.cursor
		shown := m.logShown()
		switch key.change {
		case git.RewriteMoveUp:
			at = commitRow(shown, m.cursor, m.cursor-1)
		case git.RewriteMoveDown:
			at = commitRow(shown, m.cursor, m.cursor+1)
		}
		return m, m.rewrite(entry.Hash, key.change, "", at), true
	}

	return m.fixupKey(pressed, entry)
}

// fixupKey разбирает fixup-коммиты (#225), как F и S в lazygit: F создаёт из
// индекса коммит «fixup!» к коммиту под курсором, S сливает такие коммиты в их
// цели от коммита под курсором. Курсор остаётся на том же коммите: после F он
// сдвигается на строку вниз, ведь сверху появился новый коммит.
func (m Model) fixupKey(pressed string, entry git.LogEntry) (next tea.Model, cmd tea.Cmd, handled bool) {
	creates := m.keys.Is(keymap.LogFixupCommit, pressed)
	if !creates && !m.keys.Is(keymap.LogAutosquash, pressed) {
		return m, nil, false
	}
	if refused, stop := m.refuseWhileRemote(); stop {
		return refused, nil, true
	}

	dir, short, at := m.dir, entry.Hash[:7], m.cursor
	if creates {
		note := m.text.Text("rewrite.fixupCreated", i18n.Args{"hash": short})
		return m, func() tea.Msg {
			return rewrittenMsg{err: git.Fixup(dir, entry.Hash), note: note, at: at + 1}
		}, true
	}

	note := m.text.Text("rewrite.autosquashed", i18n.Args{"hash": short})
	return m, func() tea.Msg {
		published, err := git.Autosquash(dir, entry.Hash)
		return rewrittenMsg{err: err, published: published, note: note, at: at}
	}, true
}

// rewrite запускает правку истории в команде: итог придёт rewrittenMsg.
func (m Model) rewrite(hash string, change git.Rewrite, message string, at int) tea.Cmd {
	dir := m.dir
	note := m.text.Text("rewrite.done."+string(change), i18n.Args{"hash": hash[:7]})
	return func() tea.Msg {
		published, err := git.RewriteHistory(dir, hash, change, message)
		return rewrittenMsg{err: err, published: published, note: note, at: at}
	}
}

// rewritten ставит итог в подвал и перечитывает дифф, а открытый лог
// перечитывает с курсором на строке msg.at: перенос коммита подряд вверх или
// вниз не требует искать его заново. После отказа история не менялась, и
// курсор остаётся на своём коммите. Переписанные коммиты из upstream дают
// предупреждение, как amend и reset.
func (m Model) rewritten(msg rewrittenMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.err != nil:
		m = m.say(m.text.Error(msg.err))
		msg.at = -1
	case msg.published:
		m = m.say(m.text.Text("rewrite.published", nil))
	default:
		m = m.sayCalm(msg.note)
	}

	m.stale = true
	next, reload := m.startReload()
	// ответ мог прийти, пока над логом висит вопрос: лог под ним тоже устарел
	if next.beneath() != logWindow {
		return next, reload
	}

	filter, dir := next.logFilter, next.dir
	return next, tea.Batch(reload, func() tea.Msg {
		entries, err := git.Log(dir, logLimit, filter)
		return logMsg{entries: entries, filter: filter, err: err, at: msg.at}
	})
}

// rewordMsg несёт сообщение коммита для reword или ошибку git.
type rewordMsg struct {
	hash        string
	subject     string
	description string
	err         error
}

func rewordMessageCmd(dir, hash string) tea.Cmd {
	return func() tea.Msg {
		subject, description, err := git.Message(dir, hash)
		return rewordMsg{hash: hash, subject: subject, description: description, err: err}
	}
}

// openReword открывает окно коммита с сообщением коммита из лога (#219):
// enter перепишет его сообщение rebase. Ответ, пришедший после закрытия лога,
// отбрасывается.
func (m Model) openReword(msg rewordMsg) Model {
	switch {
	case msg.err != nil:
		return m.say(m.text.Error(msg.err))
	case m.active != logWindow:
		return m
	}

	m = m.openCommit()
	m.rewording = msg.hash
	m.subject = msg.subject
	m.description = m.description.Type(msg.description)
	m.opened = fullMessage(m.subject, m.description.Text())
	return m
}

// fullMessage склеивает тему и описание через пустую строку, как git.
func fullMessage(subject, description string) string {
	if body := strings.TrimSpace(description); body != "" {
		return subject + "\n\n" + body
	}
	return subject
}

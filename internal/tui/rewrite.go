package tui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
)

// rewrittenMsg несёт итог правки истории (#219): ошибку git, переписаны ли
// коммиты из upstream, итог для подвала и коммит, на который встанет курсор,
// если лог ещё открыт: follow по теме и автору, at строкой на случай, если
// его не найти. reword говорит, что правку отправило окно коммита: его
// черновик ждёт этого ответа (commitDraft).
type rewrittenMsg struct {
	err       error
	published bool
	note      string
	follow    git.LogEntry
	at        int
	reword    bool
}

func (msg rewrittenMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.rewritten(msg) }

// rewriteAction это правка истории в логе, как в lazygit: squash и fixup в
// предыдущий коммит, drop после вопроса, reword через окно коммита, перенос
// вверх и вниз. Клавиши стоят в таблице logActions.
func rewriteAction(change git.Rewrite) func(Model, git.LogEntry) (tea.Model, tea.Cmd) {
	return func(m Model, entry git.LogEntry) (tea.Model, tea.Cmd) {
		switch change {
		case git.RewriteDrop:
			args := questionArgs(m.keys)
			args["hash"] = entry.Hash[:7]
			return m.ask(m.text.Text("rewrite.dropQuestion", args), func(m Model) (tea.Model, tea.Cmd) {
				return m, m.rewrite(entry.Hash, git.RewriteDrop, "", git.LogEntry{}, m.cursor)
			}), nil
		case git.RewriteReword:
			return m, rewordMessageCmd(m.repo, entry.Hash)
		}

		// git двигает коммит по всей истории, а не по строкам под поиском:
		// курсор ищет коммит, строка остаётся запасным путём
		at, follow := m.cursor, entry
		shown := m.logShown()
		switch change {
		case git.RewriteMoveUp:
			at = commitRow(shown, m.cursor, m.cursor-1)
		case git.RewriteMoveDown:
			at = commitRow(shown, m.cursor, m.cursor+1)
		// слитый коммит берёт тему старшего
		case git.RewriteSquash, git.RewriteFixup:
			follow = m.parentEntry(entry.Hash)
		}
		return m, m.rewrite(entry.Hash, change, "", follow, at)
	}
}

// entryOf это коммит лога с хэшем hash, пустой без него.
func (m Model) entryOf(hash string) git.LogEntry {
	index := slices.IndexFunc(m.logEntries, func(entry git.LogEntry) bool { return entry.Hash == hash })
	if index < 0 {
		return git.LogEntry{}
	}
	return m.logEntries[index]
}

// parentEntry это коммит лога под коммитом hash, пустой без него.
func (m Model) parentEntry(hash string) git.LogEntry {
	found := false
	for _, entry := range m.logEntries {
		switch {
		case entry.Hash == hash:
			found = true
		case found && entry.Hash != "":
			return entry
		}
	}
	return git.LogEntry{}
}

// fixupCommit и autosquash это fixup-коммиты (#225), как F и S в lazygit: F
// создаёт из индекса коммит «fixup!» к коммиту под курсором, S сливает такие
// коммиты в их цели от коммита под курсором. Курсор остаётся на том же
// коммите, его тема не меняется; строкой он сдвигается и после F (сверху новый
// коммит), и после S (fixup-коммиты над ним пропали).
func (m Model) fixupCommit(entry git.LogEntry) (tea.Model, tea.Cmd) {
	repo, at := m.repo, m.cursor
	note := m.text.Text("rewrite.fixupCreated", i18n.Args{"hash": entry.Hash[:7]})
	return m, func() tea.Msg {
		return rewrittenMsg{err: repo.Fixup(entry.Hash), note: note, follow: entry, at: at + 1}
	}
}

func (m Model) autosquash(entry git.LogEntry) (tea.Model, tea.Cmd) {
	repo, at := m.repo, m.cursor
	note := m.text.Text("rewrite.autosquashed", i18n.Args{"hash": entry.Hash[:7]})
	return m, func() tea.Msg {
		published, err := repo.Autosquash(entry.Hash)
		return rewrittenMsg{err: err, published: published, note: note, follow: entry, at: at}
	}
}

// rewrite запускает правку истории в команде: итог придёт rewrittenMsg.
func (m Model) rewrite(hash string, change git.Rewrite, message string, follow git.LogEntry, at int) tea.Cmd {
	repo := m.repo
	note := m.text.Text("rewrite.done."+string(change), i18n.Args{"hash": hash[:7]})
	return func() tea.Msg {
		published, err := repo.RewriteHistory(hash, change, message)
		return rewrittenMsg{err: err, published: published, note: note, follow: follow, at: at, reword: change == git.RewriteReword}
	}
}

// rewritten ставит итог в подвал и перечитывает дифф, а открытый лог
// перечитывает с курсором на строке msg.at: перенос коммита подряд вверх или
// вниз не требует искать его заново. После отказа история не менялась, и
// курсор остаётся на своём коммите. Переписанные коммиты из upstream дают
// предупреждение, как amend и reset.
func (m Model) rewritten(msg rewrittenMsg) (tea.Model, tea.Cmd) {
	if msg.reword {
		m = m.settleDraft(msg.err)
	}

	switch {
	case msg.err != nil:
		m = m.say(m.text.ErrorWith(msg.err, m.conflictKeys()))
		msg.at, msg.follow = -1, git.LogEntry{}
	case msg.published:
		m = m.say(m.text.Text("rewrite.published", nil))
	default:
		m = m.sayCalm(msg.note)
	}

	next, reload := m.startReload()
	// ответ мог прийти, пока над логом висит вопрос: лог под ним тоже устарел
	if next.beneath() != logWindow {
		return next, reload
	}

	next, read := next.requestLog(next.logFilter)
	return next, tea.Batch(reload, func() tea.Msg {
		reply := read().(logMsg)
		reply.follow, reply.at = msg.follow, msg.at
		return reply
	})
}

// rewordMsg несёт сообщение коммита для reword или ошибку git.
type rewordMsg struct {
	hash        string
	subject     string
	description string
	err         error
}

func (msg rewordMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.openReword(msg), nil }

func rewordMessageCmd(repo git.Repo, hash string) tea.Cmd {
	return func() tea.Msg {
		subject, description, err := repo.Message(hash)
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

	m = m.blankCommit()
	m.rewording = msg.hash
	if restored, ok := m.restoreDraft(); ok {
		return restored
	}
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

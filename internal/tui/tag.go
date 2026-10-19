package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
	"gitone/internal/tui/theme"
)

// tagsMsg несёт список тегов или ошибку git. note это итог правки, после
// которой список пришёл в открытое окно.
type tagsMsg struct {
	tags []git.Tag
	err  error
	note string
}

func tagsCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		tags, err := git.Tags(dir)
		return tagsMsg{tags: tags, err: err}
	}
}

// thenTags выполняет правку тегов и отвечает новым списком с итогом note, а
// отказ git ошибкой: окно тегов остаётся открытым.
func thenTags(dir, note string, change func() error) tea.Cmd {
	return func() tea.Msg {
		if err := change(); err != nil {
			return tagsMsg{err: err}
		}
		msg := tagsCmd(dir)().(tagsMsg)
		msg.note = note
		return msg
	}
}

// tagsArrived открывает окно тегов или обновляет открытое: курсор не уходит
// за край, подвал говорит итог правки.
func (m Model) tagsArrived(msg tagsMsg) Model {
	switch {
	case msg.err != nil:
		return m.say(m.text.Error(msg.err))
	case m.active == noWindow:
		m.tags, m.cursor, m.active = msg.tags, 0, tagWindow
		m.query, m.searching = "", false
	case m.active == tagWindow:
		m.tags = msg.tags
		m = m.moveCursor(0, len(m.tagsShown()))
		if msg.note != "" {
			m = m.sayCalm(msg.note)
		}
	}
	return m
}

// tagKey отдаёт клавиши окну тегов (#238): tags.diff показывает дифф коммита
// тега, tags.new ставит тег на HEAD, tags.delete удаляет после вопроса,
// tags.push отправляет на origin, list.search ищет по имени.
func (m Model) tagKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.searchKey(key); handled {
		return next, cmd
	}

	shown := m.tagsShown()
	if next, cmd, handled := m.navigate(key, len(shown)); handled {
		return next, cmd
	}

	pressed := key.String()
	if m.keys.Is(keymap.TagsNew, pressed) {
		if next, refused := m.refuseWhileRemote(); refused {
			return next, nil
		}
		return m.askTagName(), nil
	}
	if len(shown) == 0 {
		return m, nil
	}

	tag := shown[m.cursor]
	switch {
	// у первого коммита родителя нет, ^! показал бы дифф до рабочего дерева
	case m.keys.Is(keymap.TagsDiff, pressed):
		return m.showCommit(git.LogEntry{Hash: tag.Commit, Root: tag.Root})
	case m.keys.Is(keymap.TagsDelete, pressed):
		if next, refused := m.refuseWhileRemote(); refused {
			return next, nil
		}
		args := questionArgs(m.keys)
		args["name"] = visible(tag.Name)
		note := m.text.Text("tags.deleted", i18n.Args{"name": visible(tag.Name)})
		return m.ask(m.text.Text("tags.deleteQuestion", args), func(m Model) (tea.Model, tea.Cmd) {
			return m, thenTags(m.dir, note, func() error { return git.DeleteTag(m.dir, tag.Name) })
		}), nil
	case m.keys.Is(keymap.TagsPush, pressed):
		if next, refused := m.refuseWhileRemote(); refused {
			return next, nil
		}
		return m.startTagPush(tag.Name)
	}

	return m, nil
}

// askTagName спрашивает имя нового тега на HEAD, потом сообщение: с
// сообщением тег аннотированный, без него лёгкий, как в lazygit.
func (m Model) askTagName() Model {
	return m.openPrompt(m.text.Text("tags.newTitle", nil), "", func(m Model, name string) (tea.Model, tea.Cmd) {
		title := m.text.Text("tags.messageTitle", i18n.Args{"name": visible(name)})
		m = m.openPrompt(title, "", func(m Model, message string) (tea.Model, tea.Cmd) {
			note := m.text.Text("tags.created", i18n.Args{"name": visible(name)})
			return m, thenTags(m.dir, note, func() error { return git.CreateTag(m.dir, name, message) })
		})
		m.prompt.empty = true
		return m, nil
	})
}

// startTagPush отправляет тег на origin тем же фоновым путём, что push: со
// спиннером и отменой по esc.
func (m Model) startTagPush(name string) (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	m.remote = remote{kind: pushingTag, id: m.remote.id + 1, cancel: cancel}

	dir, id := m.dir, m.remote.id
	return m, tea.Batch(remoteSpinTick(id), func() tea.Msg {
		return remoteDoneMsg{id: id, kind: pushingTag, target: name, err: git.PushTag(ctx, dir, name)}
	})
}

// tagsShown это теги под фильтром поиска по имени и сообщению.
func (m Model) tagsShown() []git.Tag {
	return matching(m.tags, m.query, func(tag git.Tag) string { return tag.Name + "\x00" + tag.Subject })
}

// tagBox рисует теги таблицей, как ветки: имя цветом accent, короткий хэш
// коммита, сообщение аннотированного тега или тема коммита лёгкого и время,
// приглушённые. Аннотированный тег помечен словом за именем.
func tagBox(colors theme.Theme, text i18n.Dictionary, keys keymap.Keymap, tags []git.Tag, cursor int, query string, typing bool, question string, screenWidth, screenHeight int) string {
	box := list{
		title:    text.Text("tags.title", nil),
		close:    closeHint(colors, keys, text, keymap.ListClose),
		width:    100,
		cursor:   cursor,
		empty:    []string{text.Text("tags.empty", nil)},
		hint:     windowHint(colors, keys, text, keymap.TagsDiff, keymap.TagsNew, keymap.TagsDelete, keymap.TagsPush, keymap.ListSearch),
		question: question,
	}.withSearch(colors, text, keys, query, typing, text.Text("list.noMatches", nil))

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	muted := panel.Foreground(colors.TextMuted)
	annotated := text.Text("tags.annotated", nil)

	titles := []string{text.Text("tags.column.name", nil), text.Text("tags.column.commit", nil), text.Text("tags.column.message", nil), text.Text("tags.column.date", nil)}
	nameWidth, dateWidth := ansi.StringWidth(titles[0]), ansi.StringWidth(titles[3])
	for _, tag := range tags {
		nameWidth = max(nameWidth, ansi.StringWidth(visible(tag.Name)+" "+annotated))
		dateWidth = max(dateWidth, ansi.StringWidth(visible(tag.When)))
	}
	widths := []int{min(nameWidth, maxNameColumn), hashColumn, 0, min(dateWidth, maxDateColumn)}
	widths[2] = max(box.rowWidth(screenWidth)-widths[0]-widths[1]-widths[3]-3*columnGap, 1)

	box.header = tableRow(panel, widths, []lipgloss.Style{muted, muted, muted, muted}, titles...)

	name := panel.Foreground(colors.Accent)
	gap := panel.Render("  ")
	box.rows = make([]string, len(tags))
	for index, tag := range tags {
		box.rows[index] = markedCell(panel, name, widths[0], tag.Name, tag.Annotated, annotated) + gap +
			tableRow(panel, widths[1:], []lipgloss.Style{muted, panel, muted}, tag.Commit[:min(len(tag.Commit), hashColumn)], tag.Subject, tag.When)
	}

	return box.view(colors, screenWidth, screenHeight)
}

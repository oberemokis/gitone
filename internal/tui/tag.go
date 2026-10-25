package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// tagsMsg несёт список тегов или ошибку git. note это итог правки, после
// которой список пришёл в открытое окно.
type tagsMsg struct {
	repo git.Repo
	tags []git.Tag
	err  error
	note string
}

func (msg tagsMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.tagsArrived(msg), nil }
func (msg tagsMsg) from() git.Repo                     { return msg.repo }

func tagsCmd(repo git.Repo) tea.Cmd {
	return func() tea.Msg {
		tags, err := repo.Tags()
		return tagsMsg{repo: repo, tags: tags, err: err}
	}
}

// thenTags выполняет правку тегов и отвечает новым списком с итогом note, а
// отказ git ошибкой: окно тегов остаётся открытым.
func thenTags(repo git.Repo, note string, change func() error) tea.Cmd {
	return func() tea.Msg {
		if err := change(); err != nil {
			return tagsMsg{repo: repo, err: err}
		}
		msg := tagsCmd(repo)().(tagsMsg)
		msg.note = note
		return msg
	}
}

// tagsArrived открывает окно тегов или обновляет открытое (arrived), подвал
// говорит итог правки. Ответ правки это ответ с note.
func (m Model) tagsArrived(msg tagsMsg) Model {
	m = m.arrived(listArrival{shown: tagWindow, err: msg.err, edit: msg.note != "", set: func(m *Model) { m.tags = msg.tags }})
	if msg.err == nil && msg.note != "" {
		m = m.sayCalm(msg.note)
	}
	return m
}

// tagActions это действия окна тегов (#238): tags.diff показывает дифф коммита
// тега, tags.new ставит тег на HEAD, tags.delete удаляет после вопроса,
// tags.push отправляет на origin.
var tagActions = []windowAction[git.Tag]{
	// у первого коммита родителя нет, ^! показал бы дифф до рабочего дерева
	{action: keymap.TagsDiff, run: func(m Model, tag git.Tag) (tea.Model, tea.Cmd) {
		return m.showCommit(git.LogEntry{Hash: tag.Commit, Root: tag.Root})
	}},
	{action: keymap.TagsNew, anyRow: true, changesRepo: true, run: func(m Model, _ git.Tag) (tea.Model, tea.Cmd) {
		return m.askTagName(), nil
	}},
	{action: keymap.TagsDelete, changesRepo: true, run: Model.askDeleteTag},
	{action: keymap.TagsPush, changesRepo: true, run: func(m Model, tag git.Tag) (tea.Model, tea.Cmd) {
		return m.startTagPush(tag.Name)
	}},
}

var tagHint = actionsOf(tagActions, keymap.ListSearch)

// tagKey отдаёт клавиши окну тегов: list.search ищет по имени, остальное
// tagActions.
func (m Model) tagKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.searchKey(key); handled {
		return next, cmd
	}

	shown := m.tagsShown()
	if next, cmd, handled := m.navigate(key, len(shown)); handled {
		return next, cmd
	}
	return dispatch(m, tagActions, key.String(), shown)
}

// askDeleteTag спрашивает, удалить ли тег локально.
func (m Model) askDeleteTag(tag git.Tag) (tea.Model, tea.Cmd) {
	args := questionArgs(m.keys)
	args["name"] = visible(tag.Name)
	note := m.text.Text("tags.deleted", i18n.Args{"name": visible(tag.Name)})
	return m.ask(m.text.Text("tags.deleteQuestion", args), func(m Model) (tea.Model, tea.Cmd) {
		return m, thenTags(m.repo, note, func() error { return m.repo.DeleteTag(tag.Name) })
	}), nil
}

// askTagName спрашивает имя нового тега на HEAD, потом сообщение: с
// сообщением тег аннотированный, без него лёгкий, как в lazygit.
func (m Model) askTagName() Model {
	return m.openPrompt(m.text.Text("tags.newTitle", nil), "", func(m Model, name string) (tea.Model, tea.Cmd) {
		title := m.text.Text("tags.messageTitle", i18n.Args{"name": visible(name)})
		m = m.openPrompt(title, "", func(m Model, message string) (tea.Model, tea.Cmd) {
			note := m.text.Text("tags.created", i18n.Args{"name": visible(name)})
			return m, thenTags(m.repo, note, func() error { return m.repo.CreateTag(name, message) })
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

	repo, id := m.repo, m.remote.id
	return m, tea.Batch(remoteSpinTick(id), func() tea.Msg {
		return remoteDoneMsg{id: id, kind: pushingTag, target: name, err: repo.PushTag(ctx, name)}
	})
}

// tagsShown это теги под фильтром поиска по имени и сообщению.
func (m Model) tagsShown() []git.Tag {
	return matching(m.tags, m.query, func(tag git.Tag) string { return tag.Name + "\x00" + tag.Subject })
}

// tagBox рисует теги таблицей, как ветки: имя цветом accent, короткий хэш
// коммита, сообщение аннотированного тега или тема коммита лёгкого и время,
// приглушённые. Аннотированный тег помечен словом за именем.
func tagBox(p paint, tags []git.Tag, cursor int, query string, typing bool, question string, screenWidth, screenHeight int) string {
	box := list{
		title:    p.text.Text("tags.title", nil),
		close:    p.close(keymap.ListClose),
		width:    100,
		cursor:   cursor,
		empty:    []string{p.text.Text("tags.empty", nil)},
		hint:     p.hint(tagHint...),
		question: question,
	}.withSearch(p, query, typing, p.text.Text("list.noMatches", nil))

	panel, muted := p.panel(), p.muted()
	annotated := p.text.Text("tags.annotated", nil)

	titles := []string{p.text.Text("tags.column.name", nil), p.text.Text("tags.column.commit", nil), p.text.Text("tags.column.message", nil), p.text.Text("tags.column.date", nil)}
	nameWidth, dateWidth := ansi.StringWidth(titles[0]), ansi.StringWidth(titles[3])
	for _, tag := range tags {
		nameWidth = max(nameWidth, ansi.StringWidth(visible(tag.Name)+" "+annotated))
		dateWidth = max(dateWidth, ansi.StringWidth(visible(tag.When)))
	}
	widths := []int{min(nameWidth, maxNameColumn), hashColumn, 0, min(dateWidth, maxDateColumn)}
	widths[2] = max(box.rowWidth(screenWidth)-widths[0]-widths[1]-widths[3]-3*columnGap, 1)

	box.header = tableRow(panel, widths, []lipgloss.Style{muted, muted, muted, muted}, titles...)

	name := panel.Foreground(p.colors.Accent)
	gap := panel.Render("  ")
	box.rows = make([]string, len(tags))
	for index, tag := range tags {
		box.rows[index] = markedCell(panel, name, widths[0], tag.Name, tag.Annotated, annotated) + gap +
			tableRow(panel, widths[1:], []lipgloss.Style{muted, panel, muted}, tag.Commit[:min(len(tag.Commit), hashColumn)], tag.Subject, tag.When)
	}

	return box.view(p, screenWidth, screenHeight)
}

package tui

import (
	"slices"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
	"gitone/internal/tui/editor"
	"gitone/internal/tui/theme"
)

// commitField это поле окна коммита, в которое идёт набранное.
type commitField int

const (
	subjectField commitField = iota
	descriptionField
)

// commitCmd записывает индекс с темой и описанием. Команда, а не Update: хук
// может работать долго, и экран не должен висеть.
func commitCmd(repo git.Repo, subject, description string) tea.Cmd {
	return func() tea.Msg {
		return commitDoneMsg{actionDoneMsg{err: repo.Commit(subject, description)}}
	}
}

// amendCmd переписывает последний коммит. Если прежний коммит уже лежал в
// upstream, тост предупреждает: обычный push ветку не отправит.
func amendCmd(repo git.Repo, subject, description, note, warning string) tea.Cmd {
	return func() tea.Msg {
		published, err := repo.Amend(subject, description)
		if !published {
			warning = ""
		}
		return commitDoneMsg{actionDoneMsg{err: err, note: note, warning: warning}}
	}
}

// commitDoneMsg это ответ git на коммит или amend из окна коммита.
type commitDoneMsg struct {
	actionDoneMsg
}

func (msg commitDoneMsg) apply(m Model) (tea.Model, tea.Cmd) {
	return m.settleDraft(msg.err).actionDone(msg.actionDoneMsg)
}

// commitDraft это сообщение, отправленное из окна коммита, со своим режимом.
// Окно закрывается до ответа git, а отказ (пустой индекс, хук) не должен
// стирать набранное: refused возвращает его в следующее открытие окна в том
// же режиме (#393).
type commitDraft struct {
	subject, description, opened, rewording string
	amending, refused                       bool
}

// keepDraft запоминает набранное перед отправкой.
func (m Model) keepDraft() Model {
	m.draft = commitDraft{subject: m.subject, description: m.description.Text(), opened: m.opened, rewording: m.rewording, amending: m.amending}
	return m
}

// settleDraft разбирает ответ git: успех черновик забывает, отказ отдаёт его
// следующему открытию окна.
func (m Model) settleDraft(err error) Model {
	if err == nil {
		m.draft = commitDraft{}
	} else {
		m.draft.refused = true
	}
	return m
}

// restoreDraft возвращает в окно отвергнутое git сообщение, если окно открыто
// в том же режиме: коммит, amend или reword того же коммита.
func (m Model) restoreDraft() (Model, bool) {
	draft := m.draft
	if !draft.refused || draft.amending != m.amending || draft.rewording != m.rewording {
		return m, false
	}

	m.draft = commitDraft{}
	m.subject, m.opened = draft.subject, draft.opened
	m.description = m.description.Type(draft.description)
	return m, true
}

// lastMessageMsg несёт тему и описание последнего коммита для amend или
// ошибку: коммитов ещё нет.
type lastMessageMsg struct {
	subject     string
	description string
	err         error
}

func (msg lastMessageMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.openAmend(msg), nil }

func lastMessageCmd(repo git.Repo) tea.Cmd {
	return func() tea.Msg {
		subject, description, err := repo.LastMessage()
		return lastMessageMsg{subject: subject, description: description, err: err}
	}
}

// openAmend открывает окно коммита в режиме amend (#218) с сообщением
// последнего коммита: его правят, а не набирают заново. Ответ, пришедший при
// открытом окне, отбрасывается, как списки.
func (m Model) openAmend(msg lastMessageMsg) Model {
	switch {
	case msg.err != nil:
		return m.say(m.text.Error(msg.err))
	case m.active != noWindow:
		return m
	}

	m = m.blankCommit()
	m.amending = true
	if restored, ok := m.restoreDraft(); ok {
		return restored
	}
	m.subject = msg.subject
	m.description = m.description.Type(msg.description)
	m.opened = fullMessage(m.subject, m.description.Text())
	return m
}

// openCommit открывает окно коммита с пустой темой и описанием, а после отказа
// git с тем, что было набрано (restoreDraft).
func (m Model) openCommit() Model {
	m, _ = m.blankCommit().restoreDraft()
	return m
}

// blankCommit открывает окно коммита с пустыми полями, курсор в теме. Клавиши
// уходят в окно целиком, как в окно поиска.
func (m Model) blankCommit() Model {
	m.active = commitWindow
	m.helpOpen = false
	return m.clearCommit()
}

// clearCommit сбрасывает набранное в окне коммита и режимы amend и reword.
func (m Model) clearCommit() Model {
	m.subject = ""
	m.description = editor.Model{}.Resize(commitFieldSize(m.width, m.bodyHeight()))
	m.commitField = subjectField
	m.amending, m.rewording, m.opened = false, "", ""
	return m
}

// commitTitle это заголовок окна коммита по его режиму: amend (#218) и reword
// (#219) правят уже сделанный коммит.
func (m Model) commitTitle() string {
	switch {
	case m.amending:
		return m.text.Text("commit.amendTitle", nil)
	case m.rewording != "":
		return m.text.Text("commit.rewordTitle", i18n.Args{"hash": m.rewording[:7]})
	}
	return m.text.Text("commit.title", nil)
}

// wheelCommit прокручивает поле описания колесом на столько же строк, что и
// дифф: оно прокручивается, и когда курсор в теме.
func (m Model) wheelCommit(button tea.MouseButton) Model {
	switch button {
	case tea.MouseWheelDown:
		m.description = m.description.Scroll(wheelStep)
	case tea.MouseWheelUp:
		m.description = m.description.Scroll(-wheelStep)
	}

	return m
}

// typeCommit отдаёт клавиши окну коммита. commit.close закрывает окно из
// любого поля, commit.field переключает поле. В описании остальное разбирает
// редактор: enter там ставит новую строку, а коммитит commit.submit из темы, в
// теме набранное идёт в тему. Выйти из приложения можно по зашитому ctrl+c,
// стереть символ по зашитому backspace. Окно reword открыто из лога и после
// отправки или закрытия возвращает в него.
func (m Model) typeCommit(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	pressed := key.String()
	back := noWindow
	if m.rewording != "" {
		back = logWindow
	}

	switch {
	case pressed == "ctrl+c":
		return m, tea.Quit
	case m.keys.Is(keymap.CommitClose, pressed):
		m.active = back
		m = m.clearCommit()
	case m.keys.Is(keymap.CommitField, pressed):
		m.commitField = otherField(m.commitField)
	case m.commitField == descriptionField:
		m.description = m.description.HandleKey(key)
	case m.keys.Is(keymap.CommitSubmit, pressed):
		subject := strings.TrimSpace(m.subject)
		if subject == "" {
			return m, nil
		}
		description, amending, rewording := m.description.Text(), m.amending, m.rewording
		// без правки сообщение остаётся байт в байт: поле показывает табы
		// пробелами, и отправка их бы заменила
		unchanged := fullMessage(m.subject, description) == m.opened
		m.active = back
		m = m.keepDraft().clearCommit()
		switch {
		case amending && unchanged:
			note := m.text.Text("commit.amended", nil)
			return m, amendCmd(m.repo, "", "", note, m.text.Text("commit.amendPublished", nil))
		case amending:
			note := m.text.Text("commit.amended", nil)
			return m, amendCmd(m.repo, subject, description, note, m.text.Text("commit.amendPublished", nil))
		case rewording != "" && unchanged:
			return m, nil
		case rewording != "":
			follow := m.entryOf(rewording)
			follow.Subject = subject
			return m, m.rewrite(rewording, git.RewriteReword, fullMessage(subject, description), follow, m.cursor)
		}
		return m, commitCmd(m.repo, subject, description)
	case pressed == "backspace":
		_, size := utf8.DecodeLastRuneInString(m.subject)
		m.subject = m.subject[:len(m.subject)-size]
	default:
		m.subject += key.Text
	}

	return m, nil
}

func otherField(field commitField) commitField {
	if field == subjectField {
		return descriptionField
	}
	return subjectField
}

// commitWidth это ширина окна коммита. Ограничена экраном, как у окна справки.
func commitWidth(screenWidth int) int {
	return max(min(60, screenWidth-8), 20)
}

// Поле описания в шесть строк. В окне кроме них десять строк: отступы сверху и
// снизу, заголовок, подписи двух полей, тема, подсказка и три пустые.
const (
	descriptionRows = 6
	commitChrome    = 10
)

// commitFieldSize это ширина и высота поля описания на экране: ширина окна без
// отступов, а на низком экране поле сжимается, чтобы окно поместилось в тело.
func commitFieldSize(screenWidth, bodyHeight int) (width, height int) {
	return commitWidth(screenWidth) - 2*windowPaddingX, max(min(descriptionRows, bodyHeight-commitChrome), 1)
}

// commitBox рисует окно коммита: тему и описание, у каждого подпись, активное
// поле с курсором и подписью accent. Подсказка внизу называет поле, куда ведёт
// commit.field. Заголовок называет режим окна (commitTitle).
func commitBox(p paint, heading, subject string, description editor.Model, field commitField, screenWidth int) string {
	width := commitWidth(screenWidth)

	panel := p.panel()
	content := width - 2*windowPaddingX
	title := p.title(content, heading, p.close(keymap.CommitClose))
	label := func(key string, active bool) string {
		color := p.colors.TextMuted
		if active {
			color = p.colors.Accent
		}
		return panel.Foreground(color).Render(p.text.Text(key, nil))
	}

	input := "> " + visible(subject)
	if field == subjectField {
		input += "_"
	}
	// длинная тема не переносится, иначе окно выросло бы выше экрана: виден
	// конец, где идёт набор
	if over := ansi.StringWidth(input) - content; over > 0 {
		input = ansi.TruncateLeft(input, over+1, "…")
	}

	lines := []string{title, "", label("commit.subject", field == subjectField), panel.Render(input), "", label("commit.description", field == descriptionField)}
	cursorRow, cursorOffset := description.Cursor()
	for row, line := range description.Rows() {
		if field == descriptionField && row == cursorRow {
			line = withCursor(p.colors, panel, line, cursorOffset)
		} else {
			line = panel.Render(line)
		}
		lines = append(lines, line)
	}

	// подсказка обрезается: длинная переназначенная клавиша перенесла бы строку
	hint := truncate(commitHint(p, field), content, p.muted().Render("…"))
	lines = append(lines, "", hint)

	return panel.Padding(1, windowPaddingX).Width(width).Render(strings.Join(lines, "\n"))
}

// withCursor рисует строку line с курсором перед руной номер offset: графема
// под ним цветом primary, а в конце строки пробел.
func withCursor(colors theme.Theme, panel lipgloss.Style, line string, offset int) string {
	runes := []rune(line)
	under, after := " ", ""
	if offset < len(runes) {
		under, after, _, _ = uniseg.FirstGraphemeClusterInString(string(runes[offset:]), -1)
	}

	cursor := panel.Background(colors.Primary).Foreground(colors.BackgroundPanel)
	return panel.Render(string(runes[:offset])) + cursor.Render(under) + panel.Render(after)
}

// commitHint это подсказка окна: enter коммитит только из темы, а tab называет
// поле, куда ведёт. Отключённое действие в подсказку не идёт.
func commitHint(p paint, field commitField) string {
	panel := p.panel()

	var hints []string
	if field == subjectField {
		hints = append(hints, p.hint(keymap.CommitSubmit))
	}
	if key := firstKey(p.keys, keymap.CommitField); key != "" {
		target := "commit.hintDescription"
		if field == descriptionField {
			target = "commit.hintSubject"
		}
		hints = append(hints, pressable(keymap.CommitField, keyHint(panel, p.colors, key, p.text.Text(target, nil))))
	}

	return strings.Join(slices.DeleteFunc(hints, func(hint string) bool { return hint == "" }), panel.Render("   "))
}

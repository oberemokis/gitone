package tui

import (
	"image"
	"os/exec"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/config"
	"gitone/internal/diff"
	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
	"gitone/internal/layout"
	"gitone/internal/tui/diffview"
	"gitone/internal/tui/editor"
	filespane "gitone/internal/tui/pane/files"
	"gitone/internal/tui/picker"
	"gitone/internal/tui/theme"
	"gitone/internal/tui/zone"
)

// Строк за один щелчок колеса: по одной слишком медленно, трекпад шлёт
// десятки щелчков на жест.
const wheelStep = 3

// Колонок за одно нажатие стрелки или щелчок колеса вбок.
const shiftStep = 8

type Model struct {
	// repo и args нужны для перечитывания по r: тот же git diff, что при запуске
	repo    git.Repo
	args    []string
	loading bool
	// load номер последней начатой загрузки: шаги спиннера других загрузок
	// отбрасываются; spin кадр спиннера в подвале
	load int
	spin int
	// stale: правка gitone пришла во время загрузки, после неё нужно перечитать ещё раз
	stale bool
	// polling: идёт тихий опрос из live.go; ticks таймер опроса, nil без автообновления
	polling bool
	ticks   <-chan time.Time
	// index: поставленные, частично поставленные и конфликтные пути (load.go)
	index Index
	// location ветка и worktree для левого края подвала
	location git.Location
	// baseArgs аргументы запуска: b возвращает к ним дифф рабочего дерева
	baseArgs []string
	// active окно, которое получает клавиши; cursor курсор открытого списка
	active window
	cursor int
	// данные окон: вопрос, тема и описание коммита и поле, в которое они
	// набираются, записи списков
	question    question
	prompt      prompt
	menu        menu
	subject     string
	description editor.Model
	commitField commitField
	// amending: окно коммита переписывает последний коммит (#218), rewording
	// хэш коммита, сообщение которого оно правит (#219), opened сообщение, с
	// которым окно открылось: без правки его не переписывают
	amending   bool
	rewording  string
	opened     string
	logEntries []git.LogEntry
	// logFilter фильтр открытого лога по автору и пути (#226)
	logFilter git.LogFilter
	// copied коммит, скопированный для cherry-pick (#222), до выхода из gitone
	copied git.LogEntry
	// поиск в логе, ветках и worktree: запрос и открытое поле (window.go).
	// Окно открыто одно, поэтому поиск общий, открытие окна его сбрасывает.
	query     string
	searching bool
	branches  []git.Branch
	worktrees []git.Worktree
	stashes   []git.Stash
	tags      []git.Tag
	remotes   []git.Remote
	reflog    []git.ReflogEntry
	// подмодули для окна (#244)
	submodules []git.Submodule
	// журнал команд git для окна (#247)
	journal []git.JournalEntry
	// недавние репозитории для окна и каталог state.json (#248), пустой без
	// записи состояния
	recent   []recentRepo
	stateDir string
	picker   picker.Model
	// заметка стоит тостом (toast.go) до следующего нажатия и пропадает сама
	// через noticeLifetime (notice.go); noticeID номер заметки, у таймера свой
	// номер; noFade выключает таймер (Options.KeepNotices). calm это текст
	// итога pull или push: заметка с таким текстом идёт не цветом ошибки
	// (sayCalm)
	notice   string
	noticeID int
	noFade   bool
	calm     string
	// идущий pull или push (remote.go)
	remote remote
	keys   keymap.Keymap
	ui     config.UI
	layout layout.Node
	// filesOpen: панель файлов открыта; при запуске как в прошлый раз
	// (viewstate.go), а впервые скрыта
	filesOpen bool
	// filesWidth ширина панели файлов, вытянутая мышью (#121), 0 значит
	// ширина раскладки; drag идущее перетаскивание (drag.go)
	filesWidth int
	drag       drag
	width      int
	height     int
	helpOpen   bool
	// helpTop первая показанная строка справки, когда она выше тела (help.go)
	helpTop int
	colors  theme.Theme
	// terminal цвета терминала по умолчанию для подложки под окнами
	// (backdrop.go)
	terminal terminalColors
	text     i18n.Dictionary
	files    []diff.File
	current  int
	view     diffview.Model
	// from откуда дифф на экране: в ленте пропавший при перечитывании файл
	// сменяет сосед, только если дифф тот же (stream.go)
	from source
	// вид диффа (diffflags.go, #217): без пробелов и строки контекста, -1
	// значит число git по умолчанию
	ignoreSpace bool
	context     int
	// compare точка последнего сравнения (compare.go, #216)
	compare comparison
	// reading ключ версий файла, которые сейчас читаются (versions.go)
	reading string
	// now часы для отбрасывания двойного клика, closing последний клик,
	// закрывший окно (click.go)
	now     func() time.Time
	closing closedClick
	// exec отдаёт терминал внешней программе, редактору (edit.go)
	exec func(*exec.Cmd, tea.ExecCallback) tea.Cmd
}

// Options это всё, что корню нужно до запуска: репозиторий (Open) и аргументы
// для перечитывания по r, настройки, клавиши, раскладка и текст ошибок их
// загрузки для подвала. Пустая раскладка значит один дифф на весь экран.
type Options struct {
	Repo   git.Repo
	Args   []string
	Config config.Config
	Keys   keymap.Keymap
	Theme  theme.Theme
	Layout layout.Node
	Notice string
	// Ticks таймер опроса для автообновления, nil значит без него
	Ticks <-chan time.Time
	// Index состояние индекса при запуске
	Index Index
	// Location ветка и worktree при запуске, дальше их обновляет перечитывание
	Location git.Location
	// Text тексты интерфейса на выбранном языке, нулевое значение английское
	Text i18n.Dictionary
	// Now часы для отбрасывания двойного клика (#150), nil значит time.Now.
	// Тесты подставляют свои, чтобы не ждать.
	Now func() time.Time
	// KeepNotices отключает таймер заметок (#209): они висят до нажатия клавиши.
	// Нужен тестам, которые доводят команды до конца и не ждут пять секунд.
	KeepNotices bool
	// Exec отдаёт терминал редактору (#215), nil значит tea.ExecProcess. Тест
	// подставляет запуск без терминала: Bubble Tea выполняет ExecProcess сам,
	// и команда без программы его не выполнит.
	Exec func(*exec.Cmd, tea.ExecCallback) tea.Cmd
	// StateDir каталог state.json со списком недавних репозиториев (#248) и
	// видом (#369), пусто значит без записи состояния: так работают тесты
	StateDir string
	// View вид из state.json (config.ReadView), нулевой значит вид из Config
	View config.View
}

func New(opts Options, files []diff.File) Model {
	colors := opts.Theme
	start := opts.startView()

	root := opts.Layout
	if root.Pane == "" && len(root.Children) == 0 {
		root = layout.Focus()
	}

	view := diffview.New(colors, diffview.Mode(start.Mode), opts.Config.Diff.AutoSplitMinWidth).
		WithoutNumbers(!opts.Config.UI.LineNumbers).
		WithText(opts.Text).
		WithWrap(start.Wrap)
	files = withConflicts(files, opts.Args, opts.Index.Conflicted)
	switch {
	case start.AllFiles:
		view = view.ShowAll(files, titlesOf(files, opts.Text), 0)
	case len(files) > 0:
		view = view.SetFile(files[0])
	}

	now := opts.Now
	if now == nil {
		now = time.Now
	}
	run := opts.Exec
	if run == nil {
		run = tea.ExecProcess
	}

	return Model{
		now:      now,
		exec:     run,
		noFade:   opts.KeepNotices,
		repo:     opts.Repo,
		args:     opts.Args,
		baseArgs: opts.Args,
		ticks:    opts.Ticks,
		index:    opts.Index,
		location: opts.Location,
		notice:   opts.Notice,
		keys:     opts.Keys,
		ui:       opts.Config.UI,
		layout:   root,
		colors:   colors,
		text:     opts.Text,
		files:    files,
		view:     view,
		from:     source{repo: opts.Repo, args: opts.Args},
		stateDir: opts.StateDir,

		filesOpen:   start.Files,
		ignoreSpace: start.IgnoreWhitespace,
		context:     start.Context,
	}
}

// Init ждёт тик опроса и записывает открытый репозиторий в недавние (#248).
func (m Model) Init() tea.Cmd {
	return tea.Batch(waitTick(m.ticks), rememberRepoCmd(m.stateDir, m.repo.Root()))
}

// Update разбирает сообщение, а после него следит, чтобы видимые файлы ленты
// были раскрашены, а текущим был файл наверху (stream.go), чтобы открытый
// однофайловый компонент красился по файлу целиком (versions.go), чтобы у
// новой гаснущей заметки был таймер (notice.go) и чтобы сменившийся вид попал
// в state.json (viewstate.go).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	if model, isModel := next.(Model); isModel {
		model = model.followStream()
		cmd = model.wantSave(m.viewState(), model.wantExpiry(m.noticeID, cmd))
		return model.wantVersions(cmd)
	}
	return next, cmd
}

// applier это сообщение пакета: его вернула команда, и оно само знает, как
// лечь в модель (#355). Так сообщение и его разбор живут в файле своего окна
// или своей команды, а update разбирает только сообщения Bubble Tea.
type applier interface {
	apply(Model) (tea.Model, tea.Cmd)
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if own, ok := msg.(applier); ok {
		return own.apply(m)
	}

	switch msg := msg.(type) {
	// с первым размером экрана gitone спрашивает цвета терминала для подложки
	// под окнами (backdrop.go)
	case tea.WindowSizeMsg:
		first := m.width == 0
		m.width, m.height = msg.Width, msg.Height
		m = m.resize()
		if first {
			return m, tea.Batch(tea.RequestForegroundColor, tea.RequestBackgroundColor)
		}
	case tea.KeyPressMsg:
		return m.keyPress(msg)
	case tea.PasteMsg:
		m = m.paste(msg.Content)
	// окно лежит поверх диффа: крутить то, что закрыто, нечем и незачем (#150).
	// Вопрос с главного экрана окна не рисует, под ним дифф виден и крутится.
	// Колесо в окне-списке двигает его курсор (#197).
	case tea.MouseWheelMsg:
		switch {
		case m.helpOverflow() > 0:
			m = m.wheelHelp(msg.Button)
		case m.active == commitWindow:
			m = m.wheelCommit(msg.Button)
		case m.active == noWindow || m.footerQuestion():
			m.view = m.wheel(msg)
		default:
			m = m.wheelList(msg.Button)
		}
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			return m.click(msg.X, msg.Y)
		}
	case tea.MouseMotionMsg, tea.MouseReleaseMsg:
		m = m.mouseDrag(msg)
	// возврат в терминал из редактора: правка видна сразу, а не к тику
	case tea.FocusMsg:
		if m.ticks != nil {
			return m.poll()
		}
	case tea.ForegroundColorMsg:
		m.terminal.fg = msg.Color
	case tea.BackgroundColorMsg:
		m.terminal.bg = msg.Color
	}

	return m, nil
}

// say ставит тостом асинхронное сообщение, ошибку. Вопрос discard в подвале
// остаётся: тост его не закрывает (#371).
func (m Model) say(text string) Model {
	return m.putNotice(text)
}

// sayCalm ставит тостом итог, а не ошибку: рамка тоста у него не цвета ошибки.
// Заметка считается итогом, пока её текст равен m.calm: любая следующая
// заметка с другим текстом снова красная, и сбрасывать флаг в каждом месте, где
// заметка пишется или стирается, не нужно.
func (m Model) sayCalm(text string) Model {
	m = m.putNotice(text)
	m.calm = text
	return m
}

// dropQuestion отменяет вопрос, который стоит в подвале: на его место встаёт
// спиннер. Вопрос в окне остаётся.
func (m Model) dropQuestion() Model {
	if m.footerQuestion() {
		m.active, m.question = noWindow, question{}
	}
	return m
}

// startReload запускает перечитывание, если оно не идёт. Во время загрузки
// второе чтение рядом не стартует, но запрос не теряется: stale заставит
// прочитать заново, когда придёт ответ. Так W, лог, stash и b, сменившие
// каталог или аргументы до вызова, не оставляют на экране старый дифф. Висящий
// вопрос discard отменяется: спиннер встаёт на его место в центре подвала, и y
// не должен стирать правку, о которой вопроса уже не видно.
func (m Model) startReload() (Model, tea.Cmd) {
	if m.loading {
		m.stale = true
		return m, nil
	}

	m.loading = true
	m.load++
	m.stale = false
	m.spin = 0
	m = m.dropQuestion()
	return m, tea.Batch(spinTick(m.load), reload(m.repo, m.args, m.diffFlags()))
}

// resize отдаёт диффу ширину и высоту его панели: раскладка может отрезать
// от экрана список файлов. Положение панели здесь не нужно, только размер.
func (m Model) resize() Model {
	if area, found := paneArea(m.placement(), "diff", image.Point{}); found {
		m.view = m.view.SetSize(area.Dx(), area.Dy())
	}
	m.description = m.description.Resize(commitFieldSize(m.width, m.bodyHeight()))
	return m
}

// placement считает размеры панелей для текущего экрана.
func (m Model) placement() layout.Placed {
	return layout.Split(m.arrangement(), m.width, m.bodyHeight())
}

// arrangement это раскладка с учётом панели файлов. Закрытая панель
// убирается из дерева, открытая встаёт туда, куда её поставила раскладка, а
// в раскладке без неё слева, как в default. Открыть панель решает
// пользователь, поэтому узкий экран её не прячет. Ширина, вытянутая мышью,
// заменяет ширину раскладки, но урезается по экрану при каждом чтении, а не
// при перетаскивании (#151): после сужения окна дифф не остаётся без места,
// а после возврата размера ширина мыши действует снова. Если и урезанная
// ширина меньше minFilesWidth, действует раскладка.
func (m Model) arrangement() layout.Node {
	root := layout.Default()
	switch {
	case !m.filesOpen:
		return layout.Without(m.layout, "files")
	case layout.Has(m.layout, "files"):
		root = m.layout
	}

	if width := min(m.filesWidth, m.width-minDiffWidth); width >= minFilesWidth {
		return layout.Resized(root, "files", layout.Size{Cells: width})
	}
	return root
}

// wheel прокручивает дифф колесом: вверх и вниз по строкам, вбок по
// колонкам. Колесо с shift тоже идёт вбок: так горизонтальную прокрутку
// шлёт мышь без второго колеса.
func (m Model) wheel(msg tea.MouseWheelMsg) diffview.Model {
	button := msg.Button
	if msg.Mod&tea.ModShift != 0 {
		switch button {
		case tea.MouseWheelUp:
			button = tea.MouseWheelLeft
		case tea.MouseWheelDown:
			button = tea.MouseWheelRight
		}
	}

	switch button {
	case tea.MouseWheelDown:
		return m.view.ScrollBy(wheelStep)
	case tea.MouseWheelUp:
		return m.view.ScrollBy(-wheelStep)
	case tea.MouseWheelRight:
		return m.view.ScrollRight(shiftStep)
	case tea.MouseWheelLeft:
		return m.view.ScrollRight(-shiftStep)
	}
	return m.view
}

// handleAction выполняет действие главного экрана из таблицы globalActions.
// Действие, которое правит репозиторий, отказывает, пока идёт pull, push или
// fetch; при выделении часть действий работает иначе (selectionAction).
func (m Model) handleAction(action keymap.Action) (tea.Model, tea.Cmd) {
	entry, found := globalActions[action]
	if entry.changesRepo {
		if next, refused := m.refuseWhileRemote(); refused {
			return next, nil
		}
	}
	if next, cmd, handled := m.selectionAction(action); handled {
		return next, cmd
	}
	if !found {
		return m, nil
	}

	return entry.run(m)
}

// pickerKey отдаёт нажатие окну поиска целиком: q и ? там набираются в
// поиск. Выйти из приложения можно и отсюда, ctrl+c работает всегда.
func (m Model) pickerKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.String() == "ctrl+c" {
		return m, tea.Quit
	}

	var outcome picker.Outcome
	m.picker, outcome = m.picker.HandleKey(key)

	switch outcome {
	case picker.Chosen:
		m.active = noWindow
		if index, found := m.chosenFile(); found {
			m = m.openFile(index)
		}
	case picker.Closed:
		m.active = noWindow
	}

	return m, nil
}

// sameEntry говорит, что запись диффа это та, что пользователь видел под
// подписью label и статусом status. Запись находят по подписи и статусу вместе:
// у файла, ставшего симлинком, две записи с одним путём, удалённая и
// добавленная. Подпись и статус с обеих сторон собраны одним словарём, поэтому
// сравнение не зависит от языка.
func sameEntry(file diff.File, text i18n.Dictionary, label, status string) bool {
	return fileTitle(file, text) == label && statusLabel(file.Status, text) == status
}

// indexIn это место записи file в files по подписи и статусу (sameEntry), -1
// без неё.
func (m Model) indexIn(files []diff.File, file diff.File) int {
	label, status := fileTitle(file, m.text), statusLabel(file.Status, m.text)
	return slices.IndexFunc(files, func(candidate diff.File) bool {
		return sameEntry(candidate, m.text, label, status)
	})
}

// chosenFile ищет выбранную в окне запись в текущем диффе (sameEntry). Окно
// строилось по списку на момент открытия, а дифф мог перечитаться, пока оно
// открыто: номер записи тогда указал бы на другой файл.
func (m Model) chosenFile() (int, bool) {
	item, found := m.picker.SelectedItem()
	if !found {
		return 0, false
	}

	index := slices.IndexFunc(m.files, func(file diff.File) bool {
		return sameEntry(file, m.text, item.Label, item.Detail)
	})
	return index, index >= 0
}

// applyReload ставит новый дифф на место старого. Открытая запись остаётся
// открытой, если она ещё в диффе (sameEntry), иначе открывается первая. Пустой
// дифф сбрасывает вид: выделение без файла держать нечему. Ленту ставит
// reloadStream.
func (m Model) applyReload(msg reloadMsg) Model {
	files := msg.files
	kept := -1
	if len(m.files) > 0 {
		kept = m.indexIn(files, m.files[m.current])
	}

	from := source{repo: msg.repo, args: msg.args}
	sameDiff := m.from.same(from)
	m.from = from
	if m.view.Streaming() {
		return m.reloadStream(files, kept, sameDiff)
	}

	m.files = files
	if kept >= 0 {
		m.current = kept
		// при выделении на экране дифф вне индекса, его обновит refreshSelection
		if !m.view.Selecting() {
			m.view = m.view.Reload(files[kept])
		}
		return m
	}

	m.current = 0
	if len(files) > 0 {
		m.view = m.view.SetFile(files[0])
	} else {
		m.view = m.view.SetFile(diff.File{})
	}
	return m
}

// openPicker открывает поиск и без файлов: пустой список с надписью лучше
// клавиши, которая молча не работает.
func (m Model) openPicker() Model {
	items := make([]picker.Item, len(m.files))
	for index, file := range m.files {
		items[index] = picker.Item{Label: fileTitle(file, m.text), Detail: statusLabel(file.Status, m.text)}
	}

	keys := picker.Keys{
		Choose: m.keys.KeysFor(keymap.SearchChoose),
		Close:  m.keys.KeysFor(keymap.SearchClose),
		Up:     m.keys.KeysFor(keymap.SearchUp),
		Down:   m.keys.KeysFor(keymap.SearchDown),
	}
	m.picker = picker.New(keys, items)
	m.active = pickerWindow
	m.helpOpen = false
	return m
}

// pickerBox рисует поиск файлов тем же списком, что лог и ветки (#137):
// строка поиска «/ запрос» открыта всегда, у выбранного файла «›», статус
// файла справа приглушённый, внизу подсказка выбора. Без файлов вместо
// списка надпись об этом, а не о пустом поиске.
func pickerBox(p paint, files picker.Model, noFiles bool, screenWidth, screenHeight int) string {
	noMatches := p.text.Text("files.noMatches", nil)
	if noFiles {
		noMatches = p.text.Text("files.none", nil)
	}

	box := list{
		title:  p.text.Text("files.title", nil),
		close:  p.close(keymap.SearchClose),
		width:  90,
		cursor: files.Selected(),
		empty:  []string{noMatches},
	}.withSearch(p, files.Query(), true, noMatches)
	box.hint = p.hint(keymap.SearchChoose)

	panel, muted := p.panel(), p.muted()
	width := box.rowWidth(screenWidth)
	for _, item := range files.Shown() {
		box.rows = append(box.rows, justify(panel, width, panel.Render(item.Label), muted.Render(item.Detail)))
	}

	return box.view(p, screenWidth, screenHeight)
}

// openFile открывает файл по номеру в диффе. За краями списка ничего не
// происходит: ] на последнем файле не уводит на первый. В ленте файл встаёт
// наверх заголовком, и текущий тоже: из его середины.
func (m Model) openFile(index int) Model {
	if index < 0 || index >= len(m.files) {
		return m
	}
	if m.view.Streaming() {
		m.current = index
		m.view = m.view.Open(index)
		return m
	}
	if index == m.current {
		return m
	}

	m.current = index
	m.view = m.view.SetFile(m.files[index])
	return m
}

func (m Model) View() tea.View {
	view := tea.NewView("")
	view.AltScreen = true
	// Колесо мыши прокручивает дифф. Терминал при этом выделяет текст только
	// с shift: это принятая цена (решение в #10).
	view.MouseMode = tea.MouseModeCellMotion
	view.ReportFocus = m.ticks != nil

	// до первого WindowSizeMsg ширина неизвестна, раскладывать не по чему
	if m.width == 0 {
		return view
	}

	view.Content, _ = m.screen()
	return view
}

// screen рисует экран и находит на нём места клика (#119). Метки снимаются
// с каждой части до того, как окно ляжет поверх панелей: слой Lip Gloss их не
// переносит. Окно идёт в списке мест первым и целиком, клик по пустому месту в
// нём не доходит до панели под ним. Пока открыто окно, клик работает только в
// нём, как клавиши, поэтому места панелей и подвала не собираются, а экран
// под окном гаснет (backdrop.go).
func (m Model) screen() (string, []zone.Zone) {
	var zones []zone.Zone
	scan := func(content string, x, y int, collect bool) string {
		clean, found := zone.Scan(content)
		for _, place := range found {
			if collect {
				place.Area = place.Area.Add(image.Pt(x, y))
				zones = append(zones, place)
			}
		}
		return clean
	}
	free := m.active == noWindow
	top, height := m.titleHeight(), m.bodyHeight()

	box, modal := m.windowBox()
	x, y := centered(box, m.width, height)
	if box != "" {
		box = scan(box, x, top+y, true)
		zones = append(zones, zone.Zone{Area: image.Rect(x, top+y, x+lipgloss.Width(box), top+y+lipgloss.Height(box))})
	}
	// подложка под окном ловит клик мимо него (#367)
	if modal {
		zones = append(zones, zone.Zone{Target: backdropTarget, Area: image.Rect(0, 0, m.width, m.height)})
	}
	// стык панелей лежит поверх строк панели: за край берутся и по строке
	if free {
		zones = append(zones, m.edgeZones()...)
	}

	body := scan(lipgloss.Place(m.width, height, lipgloss.Left, lipgloss.Top, m.panes(m.placement())), 0, top, free)
	if box != "" && !modal {
		body = overlay(body, box, x, y)
	}

	var rows []string
	if m.ui.PaneTitles {
		title := ""
		if len(m.files) > 0 {
			title = header(m.width, m.files, m.current, m.backFile(), m.viewLabel(), m.colors, m.keys, m.text)
		}
		rows = append(rows, "", scan(title, 0, 1, free), "")
	}
	rows = append(rows, body, "", scan(m.footerLine(), 0, top+height+1, free), "")

	screen := lipgloss.JoinVertical(lipgloss.Left, rows...)
	if modal {
		screen = backdrop(screen, box, x, top+y, m.terminal)
	}

	// тост ложится последним: поверх окна, и подложка его не гасит. Он стоит
	// на пустой строке под заголовком и не ниже тела
	toastTop := max(top-1, 0)
	if toast := m.toast(m.width, top+height-toastTop); toast != "" {
		screen = overlay(screen, toast, m.width-toastMargin-lipgloss.Width(toast), toastTop)
	}
	return screen, zones
}

// footerLine собирает подвал: ветку слева, состояние по центру, подсказки
// справа. Пока идёт перечитывание, в центре спиннер. Пока идёт pull или push,
// спиннер с его надписью занимает то же место и важнее загрузки. Ошибки и
// итоги стоят не здесь, а тостом (toast.go).
func (m Model) footerLine() string {
	center := ""
	switch {
	// вопрос с главного экрана стоит в подвале, пока на него не ответили
	case m.footerQuestion():
		center = lipgloss.NewStyle().Foreground(m.colors.Error).Render(visible(m.question.text))
	case m.remote.running():
		center = spinner(m.remote.spin, m.colors, m.text.Text(m.remote.labelKey(), nil))
	case m.loading:
		center = spinner(m.spin, m.colors, m.text.Text("screen.loading", nil))
	}

	hints := ""
	if m.ui.KeyHints {
		hints = keyHints(m.colors, m.text, m.keys.Bindings())
	}

	return footer(m.width, locationLabel(m.location, m.colors, m.keys, m.text), center, hints)
}

// panes рисует дерево размещения: лист это панель, контейнер склеивает детей
// по своей оси. Без изменений раскладка та же, а панели говорят об этом
// сами: так видно, что открыто и куда пойдут клавиши.
func (m Model) panes(placed layout.Placed) string {
	noChanges := m.text.Text("screen.noChanges", nil)
	switch {
	case len(placed.Children) > 0:
	case placed.Pane == "files":
		return filespane.View(m.labels(), m.current, noChanges, m.colors, placed.Width, placed.Height)
	case len(m.files) == 0:
		return m.emptyBody(noChanges, placed.Width, placed.Height)
	default:
		return m.view.View()
	}

	parts := make([]string, len(placed.Children))
	for index, child := range placed.Children {
		parts[index] = m.panes(child)
	}

	if placed.Direction == "column" {
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// emptyBody рисует тело без изменений: надпись посередине, под ней после двух
// пустых строк один ряд клавиш окон (#196, #208). Ряд режется по высоте тела,
// а надпись стоит там же, где стояла без него, и видна всегда.
func (m Model) emptyBody(noChanges string, width, height int) string {
	lines := append(make([]string, (height-1)/2), noChanges)
	if row := emptyHints(m.colors, m.text, m.keys, width); row != "" {
		lines = append(lines, "", "", row)
	}
	lines = lines[:min(len(lines), max(height, 1))]

	// каждая строка по центру тела сама: ряд шире надписи не сдвигает её
	for index, line := range lines {
		lines[index] = lipgloss.PlaceHorizontal(width, lipgloss.Center, line)
	}

	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, strings.Join(lines, "\n"))
}

// labels это подписи файлов для панели: имя и статус, если он не просто
// изменён. Отметки полного индекса нет (#99): она загромождала список.
func (m Model) labels() []string {
	labels := make([]string, len(m.files))
	for index, file := range m.files {
		labels[index] = fileTitle(file, m.text)
		if status := statusLabel(file.Status, m.text); status != "" {
			labels[index] += " " + status
		}
		// частичный индекс отмечен словом (#212): space на таком файле уберёт
		// из индекса и поставленную часть; конфликт тоже словом (#242), а
		// подмодуль словом, чтобы его не путали с файлом (#244)
		switch key := stagedKey(file); {
		case isSubmodule(file):
			labels[index] += " " + m.text.Text("status.submodule", nil)
		case m.index.Conflicted[key]:
			labels[index] += " " + m.text.Text("status.conflict", nil)
		case m.index.Partial[key]:
			labels[index] += " " + m.text.Text("status.partlyStaged", nil)
		}
	}

	return labels
}

func (m Model) bodyHeight() int {
	return max(m.height-m.titleHeight()-footerHeight, 0)
}

// titleHeight это заголовок с пустой строкой под ним. Без paneTitles он
// пропадает, и тело занимает его место.
func (m Model) titleHeight() int {
	if m.ui.PaneTitles {
		return headerHeight
	}
	return 0
}

// overlay ставит справку в клетку x, y поверх base: дифф остаётся виден по
// краям и не гаснет, справка клавиш не забирает (#152).
func overlay(base, window string, x, y int) string {
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(window).X(x).Y(y).Z(1),
	).Render()
}

// centered это левый верхний угол окна по центру области width на height.
// Координаты не уходят в минус: окно шире экрана иначе теряло бы левый край.
func centered(window string, width, height int) (x, y int) {
	return max((width-lipgloss.Width(window))/2, 0), max((height-lipgloss.Height(window))/2, 0)
}

package tui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/diff"
	"gitone/internal/git"
)

// Load читает дифф каталога: cmd/gitone при запуске, а по r reload одним и тем
// же путём. flags это флаги вида (#217), при запуске их нет.
func Load(dir string, args []string, flags ...string) ([]diff.File, error) {
	text, err := git.Diff(dir, args, flags...)
	if err != nil {
		return nil, err
	}

	return diff.Parse(text), nil
}

// withConflicts дописывает в дифф рабочего дерева файлы с конфликтом, которых
// в нём нет (#242): у бинарного конфликта git оставляет в дереве нашу версию,
// она совпадает с HEAD, и файл пропал бы из списка, а решать его надо. Дифф с
// аргументами только для просмотра, туда ничего не дописывается.
func withConflicts(files []diff.File, args []string, conflicted map[string]bool) []diff.File {
	if len(args) > 0 {
		return files
	}

	var missing []string
	for path := range conflicted {
		if !slices.ContainsFunc(files, func(file diff.File) bool { return stagedKey(file) == path }) {
			missing = append(missing, path)
		}
	}
	slices.Sort(missing)

	for _, path := range missing {
		files = append(files, diff.File{OldPath: path, NewPath: path, Status: diff.StatusModified})
	}
	return files
}

// Index это состояние индекса по путям. Staged: индекс отличается от HEAD, по
// ним space решает, ставить файл в индекс или убирать. Partial: из них те, у
// которых и рабочее дерево отличается от индекса, файл поставлен частично
// (#212). Conflicted: нерешённый конфликт слияния (#242).
type Index struct {
	Staged     map[string]bool
	Partial    map[string]bool
	Conflicted map[string]bool
}

// ReadIndex читает состояние индекса.
func ReadIndex(dir string) (Index, error) {
	staged, err := git.StagedPaths(dir)
	if err != nil {
		return Index{}, err
	}
	unstaged, err := git.UnstagedPaths(dir)
	if err != nil {
		return Index{}, err
	}
	conflicted, err := git.ConflictedPaths(dir)
	if err != nil {
		return Index{}, err
	}

	index := Index{Staged: pathSet(staged), Partial: make(map[string]bool), Conflicted: pathSet(conflicted)}
	for _, path := range unstaged {
		if index.Staged[path] {
			index.Partial[path] = true
		}
	}
	return index, nil
}

func pathSet(paths []string) map[string]bool {
	set := make(map[string]bool, len(paths))
	for _, path := range paths {
		set[path] = true
	}
	return set
}

// Locate называет ветку и worktree для подвала.
func Locate(dir string) (git.Location, error) {
	return git.Locate(dir)
}

// Printable готовит текст ошибки к печати в stderr, когда интерфейс ещё не
// запущен: cmd/gitone не знает пакет diff, а текст из git (имя ветки, отказ
// хука) проходит мимо границы показа в окнах. Каждая строка идёт через
// diff.Printable отдельно: переводы строк многострочного отказа git остаются,
// ESC и `\r` становятся значками.
func Printable(text string) string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = diff.Printable(line)
	}

	return strings.Join(lines, "\n")
}

// reloadMsg несёт итог перечитывания: каталог, аргументы и флаги вида, с
// которыми читали, новый дифф, индекс, ветку или ошибку git.
type reloadMsg struct {
	dir      string
	args     []string
	flags    []string
	files    []diff.File
	index    Index
	location git.Location
	err      error
}

func (msg reloadMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.reloaded(msg) }

// fits говорит, что ответ прочитан для диффа, который открыт сейчас: пока git
// читал, W, лог, stash или b могли сменить каталог или аргументы, а клавиши
// вида флаги.
func (msg reloadMsg) fits(m Model) bool {
	return msg.dir == m.dir && slices.Equal(msg.args, m.args) && slices.Equal(msg.flags, m.diffFlags())
}

// reload читает дифф и индекс в команде, а не в Update, чтобы модель оставалась
// без ввода-вывода.
func reload(dir string, args, flags []string) tea.Cmd {
	return func() tea.Msg {
		return readDiff(dir, args, flags)
	}
}

// readDiff читает дифф, индекс и ветку: по r и при опросе одним путём. Ветку
// могли переключить в другом терминале, поэтому она читается вместе с диффом.
func readDiff(dir string, args, flags []string) reloadMsg {
	msg := reloadMsg{dir: dir, args: args, flags: flags}

	msg.files, msg.err = Load(dir, args, flags...)
	if msg.err != nil {
		return msg
	}
	msg.index, msg.err = ReadIndex(dir)
	if msg.err != nil {
		return msg
	}
	msg.files = withConflicts(msg.files, args, msg.index.Conflicted)

	msg.location, msg.err = Locate(dir)
	return msg
}

// reloaded ставит перечитанный дифф на экран, а ошибку git в подвал. Если за
// время загрузки gitone сам что-то поменял, загрузка идёт ещё раз. Ответ про
// другой каталог или другие аргументы на экран не идёт, как у опроса: его
// место занимает чтение нового диффа.
func (m Model) reloaded(msg reloadMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if !msg.fits(m) {
		return m.startReload()
	}

	// ponytail: окно поиска, открытое во время загрузки, показывает список
	// до перечитывания, выбор при этом ищется в новом по подписи и статусу.
	// Открыть окно заново обновит список.
	if msg.err != nil {
		m = m.say(m.text.Error(msg.err))
	} else {
		m = m.applyReload(msg.files)
		m.index, m.location = msg.index, msg.location
	}

	if m.stale {
		return m.startReload()
	}
	return m.refreshSelection()
}

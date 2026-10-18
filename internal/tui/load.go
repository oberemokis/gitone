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

// Staged возвращает пути, у которых индекс отличается от HEAD. По ним space
// решает, ставить файл в индекс или убирать.
func Staged(dir string) (map[string]bool, error) {
	paths, err := git.StagedPaths(dir)
	if err != nil {
		return nil, err
	}

	staged := make(map[string]bool, len(paths))
	for _, path := range paths {
		staged[path] = true
	}

	return staged, nil
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
	staged   map[string]bool
	location git.Location
	err      error
}

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
	msg.staged, msg.err = Staged(dir)
	if msg.err != nil {
		return msg
	}

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
		m.staged, m.location = msg.staged, msg.location
	}

	if m.stale {
		return m.startReload()
	}
	return m, nil
}

package tui_test

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
)

// tableLog открывает лог репозитория, где коммиты с темами subjects сделаны
// авторами authors по порядку (последний коммит наверху лога).
func tableLog(t *testing.T, subjects, authors []string) []string {
	t.Helper()

	repo := newRepo(t)
	for index, subject := range subjects {
		write(t, repo, "a.txt", subject+"\n")
		runGit(t, repo, "add", "-A")
		t.Setenv("GIT_AUTHOR_NAME", authors[index])
		runGit(t, repo, "commit", "-q", "-m", subject)
	}

	model := resize(open(t, options(repo, layout.Focus())), 140, 40)
	return screenOf(openAsync(model, key('l')))
}

// column это колонка первого вхождения text в строке, в клетках экрана.
func column(line, text string) int {
	at := strings.Index(line, text)
	if at < 0 {
		return -1
	}
	return ansi.StringWidth(line[:at])
}

// rowWith это строка экрана, в которой есть text.
func rowWith(t *testing.T, screen []string, text string) string {
	t.Helper()

	for _, line := range screen {
		if strings.Contains(line, text) {
			return line
		}
	}
	t.Fatalf("строки с %q нет:\n%s", text, strings.Join(screen, "\n"))
	return ""
}

// relativeDate это дата коммита в строке лога: «0 seconds ago» или «1 second
// ago», если коммит попал на границу секунды.
var relativeDate = regexp.MustCompile(`\d+ \S+ ago`)

// Заголовки столбцов стоят над началом своих столбцов, а автор и дата
// начинаются с одной колонки при разной длине тем (#109).
func TestLogColumnsLineUp(t *testing.T) {
	screen := tableLog(t, []string{"short", "a much longer subject line"}, []string{"ann", "bob"})

	header := rowWith(t, screen, "Message")
	short, long := rowWith(t, screen, "short"), rowWith(t, screen, "a much longer")

	for _, pair := range []struct{ title, text, other string }{
		{title: "Message", text: "short", other: "a much longer"},
		{title: "Author", text: "ann", other: "bob"},
		{title: "Date", text: relativeDate.FindString(short), other: relativeDate.FindString(long)},
	} {
		want := column(header, pair.title)
		if want < 0 {
			t.Fatalf("в заголовке нет %q: %q", pair.title, header)
		}
		if got := column(short, pair.text); got != want {
			t.Errorf("%q с колонки %d, заголовок %q с %d:\n%s\n%s", pair.text, got, pair.title, want, header, short)
		}
		if got := column(long, pair.other); got != want {
			t.Errorf("%q с колонки %d, заголовок %q с %d:\n%s\n%s", pair.other, got, pair.title, want, header, long)
		}
	}
	if column(header, "Commit") != column(short, "short")-9 {
		t.Errorf("Commit не над хэшем: %q / %q", header, short)
	}
}

// Длинная тема обрезается «…» и не сдвигает автора; длинное имя автора
// обрезается до 20 колонок.
func TestLogLongCellsAreCut(t *testing.T) {
	longSubject := strings.Repeat("subject ", 30)
	longAuthor := "Alexander Konstantinovich Very-Long-Surname"
	screen := tableLog(t, []string{"short", longSubject}, []string{"ann", longAuthor})

	header := rowWith(t, screen, "Message")
	row := rowWith(t, screen, "subject subject")
	if !strings.Contains(row, "…") {
		t.Errorf("длинная тема не обрезана: %q", row)
	}
	if got, want := column(row, "Alexander"), column(header, "Author"); got != want {
		t.Errorf("автор с колонки %d, want %d: %q", got, want, row)
	}
	// 20 колонок вместе с «…»
	if strings.Contains(row, longAuthor) || !strings.Contains(row, "Alexander Konstanti…") {
		t.Errorf("имя автора не обрезано до 20 колонок: %q", row)
	}
}

// В окне worktree ветка цветом accent, путь приглушённый.
func TestWorktreeColors(t *testing.T) {
	_, model := threeFileRepo(t)
	raw := openAsync(model, key('W')).View().Content

	if code := styleBefore(t, raw, "(current)", "main"); !strings.Contains(code, "38;2;157;124;216") {
		t.Errorf("ветка не цветом accent: %q", code)
	}
	if code := styleBefore(t, raw, "(current)", string(filepath.Separator)); !strings.Contains(code, "90") {
		t.Errorf("путь не приглушён: %q", code)
	}
}

// В окне веток имя каждой ветки цветом accent, как в окне worktree, а
// отметка текущей цветом текста (#117).
func TestBranchColors(t *testing.T) {
	repo, model := threeFileRepo(t)
	runGit(t, repo, "branch", "feature")
	raw := openAsync(model, key('B')).View().Content

	for _, name := range []string{"main", "feature"} {
		if code := styleBefore(t, raw, name, name); !strings.Contains(code, "38;2;157;124;216") {
			t.Errorf("ветка %s не цветом accent: %q", name, code)
		}
	}
	if code := styleBefore(t, raw, "(current)", "(current)"); strings.Contains(code, "38;2;157;124;216") {
		t.Errorf("отметка текущей цветом accent: %q", code)
	}
}

// Окно веток это таблица: у ветки автор и время последнего коммита, столбцы
// начинаются с одной колонки под своими заголовками, длинный автор обрезан
// «…» до 20 колонок (#124).
func TestBranchColumnsLineUp(t *testing.T) {
	repo := newRepo(t)
	t.Setenv("GIT_AUTHOR_NAME", "ann")
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "a-much-longer-feature")
	t.Setenv("GIT_AUTHOR_NAME", "bob with a very long author name")
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")

	model := resize(open(t, options(repo, layout.Focus())), 140, 40)
	screen := screenOf(openAsync(model, key('B')))

	header := rowWith(t, screen, "Author")
	short, long := rowWith(t, screen, "main"), rowWith(t, screen, "a-much-longer-feature")
	for _, pair := range []struct{ title, text, other string }{
		{title: "Branch", text: "main", other: "a-much-longer-feature"},
		{title: "Author", text: "ann", other: "bob with"},
		{title: "Date", text: relativeDate.FindString(short), other: relativeDate.FindString(long)},
	} {
		want := column(header, pair.title)
		if got := column(short, pair.text); want < 0 || got != want {
			t.Errorf("%q с колонки %d, заголовок %q с %d:\n%s\n%s", pair.text, got, pair.title, want, header, short)
		}
		if got := column(long, pair.other); got != want {
			t.Errorf("%q с колонки %d, заголовок %q с %d:\n%s\n%s", pair.other, got, pair.title, want, header, long)
		}
	}
	if !strings.Contains(long, "bob with a very lon…") {
		t.Errorf("длинный автор не обрезан до 20 колонок: %q", long)
	}
}

// cellBackgrounds это фон каждой клетки строки как «r;g;b» из SGR 48;2,
// пустая строка без фона.
func cellBackgrounds(line string) []string {
	var cells []string
	background := ""
	for line != "" {
		if found := sgr.FindStringIndex(line); found != nil && found[0] == 0 {
			params := strings.Split(line[2:found[1]-1], ";")
			for index := 0; index < len(params); index++ {
				switch params[index] {
				case "", "0", "49":
					background = ""
				case "48", "38":
					if index+4 < len(params) && params[index+1] == "2" {
						if params[index] == "48" {
							background = strings.Join(params[index+2:index+5], ";")
						}
						index += 4
					}
				}
			}
			line = line[found[1]:]
			continue
		}

		_, size := utf8.DecodeRuneInString(line)
		cells = append(cells, background)
		line = line[size:]
	}
	return cells
}

// Строка списка от левого до правого края окна идёт с фоном окна: пробелы
// после короткой строки не чёрные. Так в окнах веток и worktree и в строке
// соединения графа в логе (#131).
func TestListRowsKeepWindowBackground(t *testing.T) {
	branches := func(t *testing.T) tea.Model { _, model := threeFileRepo(t); return openAsync(model, key('B')) }
	worktrees := func(t *testing.T) tea.Model { _, model := threeFileRepo(t); return openAsync(model, key('W')) }

	for _, tc := range []struct {
		title, row string
		open       func(t *testing.T) tea.Model
	}{
		{title: "Branches", row: "(current)", open: branches},
		{title: "Worktrees", row: "(current)", open: worktrees},
		{title: "Log", row: `|\`, open: mergeLog},
	} {
		model := tc.open(t)
		screen := screenOf(model)
		title := rowWith(t, screen, "close")
		start, end := column(title, tc.title), column(title, "close")+len("close")

		// строка ищется без кодов: в графе между символами стоят цвета
		row := slices.IndexFunc(screen, func(line string) bool { return strings.Contains(line, tc.row) })
		if row < 0 {
			t.Fatalf("окно %s: строки с %q нет:\n%s", tc.title, tc.row, strings.Join(screen, "\n"))
		}
		line := strings.Split(model.View().Content, "\n")[row]
		cells := cellBackgrounds(line)
		for x := start; x < end; x++ {
			if cells[x] != "20;20;20" {
				t.Fatalf("окно %s: клетка %d без фона окна (%q):\n%q", tc.title, x, cells[x], ansi.Strip(line))
			}
		}
	}
}

// В окне веток автор и дата у правого края, как в логе: дата кончается под
// «close», а отметка текущей стоит сразу за именем (#131).
func TestBranchDateAtRightEdge(t *testing.T) {
	_, model := branchesWith(t)
	screen := screenOf(model)

	end := column(rowWith(t, screen, "close"), "close") + len("close")
	row := rowWith(t, screen, "(current)")
	if date := relativeDate.FindString(row); column(row, date)+len(date) != end {
		t.Errorf("дата кончается в колонке %d, want %d: %q", column(row, date)+len(date), end, row)
	}
	if got, want := column(row, "(current)"), column(row, "main")+len("main")+1; got != want {
		t.Errorf("(current) с колонки %d, want %d сразу за именем: %q", got, want, row)
	}
}

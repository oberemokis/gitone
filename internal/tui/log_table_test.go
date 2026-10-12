package tui_test

import (
	"path/filepath"
	"strings"
	"testing"

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

// Заголовки столбцов стоят над началом своих столбцов, а автор и дата
// начинаются с одной колонки при разной длине тем (#109).
func TestLogColumnsLineUp(t *testing.T) {
	screen := tableLog(t, []string{"short", "a much longer subject line"}, []string{"ann", "bob"})

	header := rowWith(t, screen, "Message")
	short, long := rowWith(t, screen, "short"), rowWith(t, screen, "a much longer")

	for _, pair := range []struct{ title, text, other string }{
		{title: "Message", text: "short", other: "a much longer"},
		{title: "Author", text: "ann", other: "bob"},
		{title: "Date", text: "0 seconds", other: "0 seconds"},
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

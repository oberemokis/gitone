package tui_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// l открывает лог, enter показывает диф коммита, b возвращает рабочее дерево.
func TestLogShowsCommitDiffAndGoesBack(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)
	// в дереве правка, которой нет ни в одном коммите
	write(t, repo, "a.txt", "three\n")

	model := resize(open(t, options(repo, layout.Default())), 100, 20)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	model = settle(model, cmd, 4)
	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "commit") {
		t.Fatalf("лог не открылся:\n%s", body)
	}

	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 4)
	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "one") || strings.Contains(body, "three") {
		t.Fatalf("дифф коммита не показан:\n%s", body)
	}

	model, cmd = model.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	model = settle(model, cmd, 4)
	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "three") {
		t.Fatalf("рабочее дерево не вернулось:\n%s", body)
	}
}

// openLog открывает лог репозитория с count коммитами на экране width на height.
func openLog(t *testing.T, count, width, height int) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	for index := range count {
		write(t, repo, "a.txt", fmt.Sprintf("%d\n", index))
		commitAll(t, repo)
	}

	model := resize(open(t, options(repo, layout.Focus())), width, height)
	return repo, openAsync(model, key('l'))
}

// Окно лога во всю ширину с полями: по 12 колонок слева и справа, по 4 строки
// сверху и снизу (восьмая часть высоты тела 36), список заполняет высоту.
func TestLogWindowFillsScreenWithMargins(t *testing.T) {
	_, model := openLog(t, 40, 120, 40)
	screen := screenOf(model)

	title := slices.IndexFunc(screen, func(line string) bool { return strings.Contains(line, "Log") })
	// тело с строки bodyRow, поле 4 строки, отступ окна 1 строка
	if want := bodyRow + 4 + 1; title != want {
		t.Errorf("заголовок окна в строке %d, want %d:\n%s", title, want, strings.Join(screen, "\n"))
	}
	// поле 12 колонок и боковой отступ окна 2
	if column := strings.Index(screen[title], "Log"); column != 14 {
		t.Errorf("заголовок окна с колонки %d, want 14: %q", column, screen[title])
	}

	// высота окна 36-8: без отступов, заголовка, пустой строки и подсказки 23 строки
	if shown := strings.Count(strings.Join(screen, "\n"), " · "); shown != 23 {
		t.Errorf("строк лога %d, want 23", shown)
	}
}

// На узком экране поле не больше восьмой части ширины.
func TestLogWindowMarginOnNarrowScreen(t *testing.T) {
	_, model := openLog(t, 3, 80, 40)
	screen := screenOf(model)

	title := slices.IndexFunc(screen, func(line string) bool { return strings.Contains(line, "Log") })
	// поле 80/8 = 10 колонок и боковой отступ окна 2
	if column := strings.Index(screen[title], "Log"); column != 12 {
		t.Errorf("заголовок окна с колонки %d, want 12: %q", column, screen[title])
	}
}

// Хэш коммита окрашен accent темы: #9d7cd8 в ANSI это 38;2;157;124;216.
func TestLogHashUsesAccent(t *testing.T) {
	repo, model := openLog(t, 1, 120, 40)
	hash := strings.TrimSpace(runGit(t, repo, "rev-parse", "--short=7", "HEAD"))

	line := rawLine(t, model.View().Content, hash)
	if accent := strings.Index(line, "38;2;157;124;216"); accent < 0 || accent > strings.Index(line, hash) {
		t.Fatalf("хэш без цвета accent: %q", line)
	}
}

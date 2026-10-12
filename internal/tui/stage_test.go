package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/config"
	"gitone/internal/git"
	"gitone/internal/keymap"
	"gitone/internal/layout"
	"gitone/internal/tui"
	"gitone/internal/tui/theme"
)

// settle выполняет команду и её ответы, пока команды дают ответы. Ограничение
// глубины не даёт зациклиться на ожидании сигнала.
func settle(model tea.Model, cmd tea.Cmd, depth int) tea.Model {
	if depth == 0 {
		return model
	}
	for _, msg := range runAll(cmd) {
		var next tea.Cmd
		model, next = model.Update(msg)
		model = settle(model, next, depth-1)
	}
	return model
}

func openStage(t *testing.T, repo string) tea.Model {
	t.Helper()

	files, err := tui.Load(repo, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	opts := tui.Options{Dir: repo, Config: config.Default(), Keys: keymap.Default(), Theme: theme.Dark(), Layout: layout.Default()}
	model, _ := tui.New(opts, files).Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	return model
}

func press(model tea.Model, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	return model.Update(key)
}

// Пробел ставит текущий файл в индекс: в панели появляется отметка, в git он в
// индексе.
func TestSpaceStagesCurrentFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	model := openStage(t, repo)
	model, cmd := press(model, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	model = settle(model, cmd, 4)

	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "● a.txt") {
		t.Fatalf("отметки нет в панели:\n%s", body)
	}
	if staged, err := git.StagedPaths(repo); err != nil || len(staged) != 1 || staged[0] != "a.txt" {
		t.Fatalf("в индексе %q, ошибка %v", staged, err)
	}
}

// x спрашивает, и только y отменяет правку. Любая другая клавиша отменяет
// вопрос и не выполняет действие.
func TestDiscardNeedsYes(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	model := openStage(t, repo)
	model, _ = press(model, tea.KeyPressMsg{Code: 'x', Text: "x"})
	if footer := screenOf(model)[len(screenOf(model))-1]; !strings.Contains(footer, "отменить") {
		t.Fatalf("вопроса нет в подвале: %q", footer)
	}

	model, cmd := press(model, tea.KeyPressMsg{Code: 'n', Text: "n"})
	model = settle(model, cmd, 4)
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "two\n" {
		t.Fatalf("правка пропала без подтверждения: %q", got)
	}

	model, _ = press(model, tea.KeyPressMsg{Code: 'x', Text: "x"})
	model, cmd = press(model, tea.KeyPressMsg{Code: 'y', Text: "y"})
	model = settle(model, cmd, 4)
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "one\n" {
		t.Fatalf("правка не отменена после y: %q", got)
	}
	_ = model
}

func readText(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

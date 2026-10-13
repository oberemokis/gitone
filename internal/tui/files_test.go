package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
	"gitone/internal/tui"
)

func TestBracketsMoveBetweenFilesAndStopAtEdges(t *testing.T) {
	_, model := threeFileRepo(t)
	next := tea.KeyPressMsg{Code: ']', Text: "]"}
	previous := tea.KeyPressMsg{Code: '[', Text: "["}

	steps := []struct {
		name        string
		key         tea.KeyPressMsg
		wantFile    string
		wantCounter string
	}{
		{name: "[ на первом", key: previous, wantFile: "a.txt", wantCounter: "(1/3)"},
		{name: "]", key: next, wantFile: "b.txt", wantCounter: "(2/3)"},
		{name: "второй ]", key: next, wantFile: "c.txt", wantCounter: "(3/3)"},
		{name: "] на последнем", key: next, wantFile: "c.txt", wantCounter: "(3/3)"},
		{name: "[", key: previous, wantFile: "b.txt", wantCounter: "(2/3)"},
	}

	for _, step := range steps {
		model, _ = model.Update(step.key)
		screen := screenOf(model)

		if got := openedFile(screen); got != step.wantFile {
			t.Fatalf("%s: открыт %q, ожидался %q", step.name, got, step.wantFile)
		}
		if header := screen[headerRow]; !strings.Contains(header, step.wantCounter) {
			t.Fatalf("%s: в заголовке нет %q: %q", step.name, step.wantCounter, header)
		}
	}
}

func TestPickerFiltersBySubsequenceAndOpensChoice(t *testing.T) {
	repo := newRepo(t)
	paths := []string{"README.md", "internal/tui/app.go", "internal/tui/keys.go"}
	for _, name := range paths {
		write(t, repo, name, "old\n")
	}
	commitAll(t, repo)
	for _, name := range paths {
		write(t, repo, name, "new\n")
	}
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	model = typeText(model, "tuiapp")

	screen := strings.Join(screenOf(model), "\n")
	if !strings.Contains(screen, "internal/tui/app.go") || strings.Contains(screen, "keys.go") {
		t.Fatalf("после tuiapp в списке не только internal/tui/app.go:\n%s", screen)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	after := screenOf(model)

	if got := openedFile(after); got != "internal/tui/app.go" {
		t.Fatalf("после enter открыт %q", got)
	}
	if body := strings.Join(after, "\n"); strings.Contains(body, "esc close") || !strings.Contains(after[headerRow], "(2/3)") {
		t.Fatalf("окно не закрылось или счётчик не сменился:\n%s", body)
	}
}

func TestPickerTakesQuitAndHelpKeysAsText(t *testing.T) {
	_, model := threeFileRepo(t)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})

	model, cmd := model.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd != nil {
		t.Fatal("q в поиске вернул команду, ожидалась буква в поле")
	}
	model = typeText(model, "?")
	screen := strings.Join(screenOf(model), "\n")
	if !strings.Contains(screen, "q?") || strings.Contains(screen, "Hotkeys") {
		t.Fatalf("q и ? не набрались в поиск:\n%s", screen)
	}

	_, cmd = model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl c в окне поиска не выходит из приложения")
	}
	if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Fatal("ctrl c в окне поиска не выходит из приложения")
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if screen := strings.Join(screenOf(model), "\n"); strings.Contains(screen, "esc close") || strings.Contains(screen, "q?") {
		t.Fatalf("esc не закрыл окно:\n%s", screen)
	}
}

func TestSamePathTwiceIsDistinguishableInPicker(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "link", "content\n")
	commitAll(t, repo)
	if err := os.Remove(filepath.Join(repo, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(repo, "link")); err != nil {
		t.Fatal(err)
	}
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})

	var rows []string
	for row, line := range screenOf(model) {
		if strings.Contains(line, "link") && row != headerRow {
			rows = append(rows, strings.Join(strings.Fields(line), " "))
		}
	}
	if len(rows) != 2 || rows[0] == rows[1] {
		t.Fatalf("ожидались две разные строки link в списке, получили %q", rows)
	}
}

func TestWindowsLieOverDiffWithEdgesVisible(t *testing.T) {
	_, model := threeFileRepo(t)

	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})

	screen := screenOf(model)
	if !strings.Contains(screen[bodyRow], "old a.txt") || !strings.Contains(strings.Join(screen, "\n"), "esc close") {
		t.Fatalf("окно не лежит поверх диффа с видимыми краями:\n%s", strings.Join(screen, "\n"))
	}
}

func TestHelpShrinksOnNarrowScreen(t *testing.T) {
	model := resize(tui.New(options("", layout.Focus()), nil), 40, 40)
	model, _ = model.Update(tea.KeyPressMsg{Code: '?', Text: "?"})

	screen := screenOf(model)
	for index, line := range screen {
		if width := ansi.StringWidth(line); width > 40 {
			t.Fatalf("строка %d шириной %d вылезла за экран в 40 колонок: %q", index, width, line)
		}
	}
	if body := strings.Join(screen, "\n"); !strings.Contains(body, "Hotkeys") || !strings.Contains(body, "esc") {
		t.Fatalf("справка потеряла заголовок или esc:\n%s", body)
	}
}

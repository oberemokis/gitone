package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/config"
	"gitone/internal/keymap"
	"gitone/internal/tui"
	"gitone/internal/tui/theme"
)

// pickerOverReload открывает дифф трёх файлов со слежением, открывает окно
// поиска и набирает query. Потом a.txt откатывается, и сигнал слежения
// перечитывает дифф, пока окно открыто: в новом списке b.txt и c.txt.
func pickerOverReload(t *testing.T, query string) tea.Model {
	t.Helper()

	repo := newRepo(t)
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		write(t, repo, name, "old "+name+"\n")
	}
	commitAll(t, repo)
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		write(t, repo, name, "new "+name+"\n")
	}

	files, err := tui.Load(repo, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	changes := make(chan struct{}, 1)
	opts := tui.Options{Dir: repo, Config: config.Default(), Keys: keymap.Default(), Theme: theme.Dark(), Changes: changes}
	var model tea.Model = tui.New(opts, files)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	model = typeText(model, query)

	runGit(t, repo, "checkout", "--", "a.txt")
	changes <- struct{}{}
	return settle(model, model.Init(), 4)
}

// Выбор после перечитывания открывает выбранный по имени файл, а не файл с тем
// же номером в новом списке.
func TestPickerChoiceAfterReloadOpensChosenFile(t *testing.T) {
	model := pickerOverReload(t, "c")

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := openedFile(screenOf(model)); got != "c.txt" {
		t.Fatalf("открыт %q, ожидался c.txt:\n%s", got, strings.Join(screenOf(model), "\n"))
	}
}

// Выбранного файла после перечитывания нет: окно закрывается, открыт прежний.
func TestPickerChoiceOfGoneFileKeepsOpenFile(t *testing.T) {
	model := pickerOverReload(t, "a")

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	screen := screenOf(model)
	if got := openedFile(screen); got != "b.txt" {
		t.Fatalf("открыт %q, ожидался b.txt:\n%s", got, strings.Join(screen, "\n"))
	}
	if strings.Contains(strings.Join(screen, "\n"), "Search") {
		t.Fatalf("окно поиска не закрылось:\n%s", strings.Join(screen, "\n"))
	}
}

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// pickerOverReload открывает дифф трёх файлов со слежением, открывает окно
// поиска и набирает query. Потом a.txt откатывается, и сигнал слежения
// перечитывает дифф, пока окно открыто: в новом списке b.txt и c.txt.
func pickerOverReload(t *testing.T, query string) tea.Model {
	t.Helper()

	repo, _ := threeFileRepo(t)
	changes := make(chan struct{}, 1)
	opts := options(repo, layout.Focus())
	opts.Changes = changes
	model := resize(open(t, opts), 100, 20)

	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	model = typeText(model, query)

	runGit(t, repo, "checkout", "--", "a.txt")
	changes <- struct{}{}
	// сигнал последний: закрытый канал отпускает ожидание следующего
	close(changes)
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

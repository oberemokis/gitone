package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// pickerOverReload открывает дифф трёх файлов с таймером опроса, переходит на
// файл номер opened (0 это a.txt), открывает окно поиска и набирает query.
// Потом a.txt откатывается, и тик перечитывает дифф, пока окно открыто: в
// новом списке b.txt и c.txt.
func pickerOverReload(t *testing.T, query string, opened int) tea.Model {
	t.Helper()

	repo, _ := threeFileRepo(t)
	opts := options(repo, layout.Focus())
	opts.Ticks = oneTick()
	model := resize(open(t, opts), 100, 20)
	for range opened {
		model, _ = model.Update(key(']'))
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	model = typeText(model, query)

	runGit(t, repo, "checkout", "--", "a.txt")
	return settle(model, model.Init(), 4)
}

// Выбор после перечитывания открывает выбранный по имени файл, а не файл с тем
// же номером в новом списке.
func TestPickerChoiceAfterReloadOpensChosenFile(t *testing.T) {
	model := pickerOverReload(t, "c", 0)

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if got := openedFile(screenOf(model)); got != "c.txt" {
		t.Fatalf("открыт %q, ожидался c.txt:\n%s", got, strings.Join(screenOf(model), "\n"))
	}
}

// Выбранного файла после перечитывания нет: окно закрывается, открыт прежний.
// Открыт c.txt, а не первый файл нового списка: иначе «оставить открытый» и
// «открыть первый» давали бы один экран.
func TestPickerChoiceOfGoneFileKeepsOpenFile(t *testing.T) {
	model := pickerOverReload(t, "a", 2)

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	screen := screenOf(model)
	if got := openedFile(screen); got != "c.txt" {
		t.Fatalf("открыт %q, ожидался c.txt:\n%s", got, strings.Join(screen, "\n"))
	}
	if strings.Contains(strings.Join(screen, "\n"), "esc close") {
		t.Fatalf("окно поиска не закрылось:\n%s", strings.Join(screen, "\n"))
	}
}

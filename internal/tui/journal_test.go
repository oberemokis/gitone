package tui_test

import (
	"slices"
	"strings"
	"testing"

	"gitone/internal/layout"
)

// @ открывает журнал: правка из приложения видна командой git, отказ с кодом
// выхода (#247, случаи 1 и 2). Журнал общий для всех тестов пакета, поэтому
// своя запись узнаётся по имени файла, которого нет в других тестах (#290,
// случай 3).
func TestJournalWindow(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "journal-window.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "journal-window.txt", "two\n")
	model := resize(open(t, options(repo, layout.Focus())), 120, 20)

	model = pressAndSettle(model, key(' '))
	model = openAsync(model, key('@'))

	body := screenText(model)
	own := slices.ContainsFunc(strings.Split(body, "\n"), func(row string) bool {
		return strings.Contains(row, "git add") && strings.Contains(row, "journal-window.txt")
	})
	if !strings.Contains(body, "Git commands") || !own {
		t.Fatalf("в журнале нет своего git add:\n%s", body)
	}
	if strings.Contains(body, "git diff") {
		t.Fatalf("чтение диффа попало в журнал:\n%s", body)
	}
}

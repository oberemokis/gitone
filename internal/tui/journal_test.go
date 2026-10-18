package tui_test

import (
	"strings"
	"testing"

	"gitone/internal/layout"
)

// @ открывает журнал: правка из приложения видна командой git, отказ с кодом
// выхода (#247, случаи 1 и 2).
func TestJournalWindow(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	model := resize(open(t, options(repo, layout.Focus())), 120, 20)

	model = pressAndSettle(model, key(' '))
	model = openAsync(model, key('@'))

	body := screenText(model)
	if !strings.Contains(body, "Git commands") || !strings.Contains(body, "git add") || !strings.Contains(body, "a.txt") {
		t.Fatalf("в журнале нет git add:\n%s", body)
	}
	if strings.Contains(body, "git diff") {
		t.Fatalf("чтение диффа попало в журнал:\n%s", body)
	}
}

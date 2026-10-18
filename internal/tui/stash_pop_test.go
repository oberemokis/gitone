package tui_test

import (
	"strings"
	"testing"

	"gitone/internal/layout"
)

// stashedRepo это репозиторий с одной записью stash, правкой a.txt «two».
func stashedRepo(t *testing.T) string {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "stash", "push", "-q")
	return repo
}

// g в окне stash применяет запись и удаляет её (#235, случай 1).
func TestStashPopAppliesAndDrops(t *testing.T) {
	repo := stashedRepo(t)

	model := openAsync(resize(open(t, options(repo, layout.Focus())), 100, 20), key('Z'))
	pressAndSettle(model, key('g'))

	if got := readText(t, repo+"/a.txt"); got != "two\n" {
		t.Fatalf("после g файл %q, ожидалась правка", got)
	}
	if got := runGit(t, repo, "stash", "list"); got != "" {
		t.Fatalf("запись stash осталась: %q", got)
	}
}

// Конфликт оставляет запись, подвал говорит об этом (случай 2).
func TestStashPopConflictKeepsEntry(t *testing.T) {
	repo := stashedRepo(t)
	write(t, repo, "a.txt", "three\n")
	commitAll(t, repo)

	model := openAsync(resize(open(t, options(repo, layout.Focus())), 120, 20), key('Z'))
	model = pressAndSettle(model, key('g'))

	if footer := footerOf(model); !strings.Contains(footer, "the stash entry is kept") {
		t.Fatalf("подвал %q, ожидалось, что запись осталась", footer)
	}
	if got := runGit(t, repo, "stash", "list"); got == "" {
		t.Fatal("запись stash удалена при конфликте")
	}
}

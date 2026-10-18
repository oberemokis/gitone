package tui_test

import (
	"strings"
	"testing"

	"gitone/internal/layout"
)

// h открывает лог только с коммитами открытого файла, вместе с теми, что были
// до переименования (#241, случаи 1 и 2).
func TestFileHistory(t *testing.T) {
	repo := newRepo(t)
	commit := func(name, subject string) {
		write(t, repo, name, subject+"\n")
		runGit(t, repo, "add", "-A")
		runGit(t, repo, "commit", "-q", "-m", subject)
	}
	commit("old.txt", "create-old")
	commit("other.txt", "touch-other")
	runGit(t, repo, "mv", "old.txt", "new.txt")
	runGit(t, repo, "commit", "-q", "-m", "rename")
	write(t, repo, "new.txt", "edited\n")

	model := resize(open(t, options(repo, layout.Focus())), 120, 30)
	model = openAsync(model, key('h'))

	body := screenText(model)
	if !strings.Contains(body, "path new.txt") || !strings.Contains(body, "create-old") || strings.Contains(body, "touch-other") {
		t.Fatalf("история файла неполная или лишняя:\n%s", body)
	}
}

// У нового неотслеженного файла истории нет: подвал говорит об этом, окно не
// открывается (случай 3).
func TestFileHistoryOfNewFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "fresh.txt", "new\n")

	model := resize(open(t, options(repo, layout.Focus())), 120, 30)
	if got := openedFile(screenOf(model)); got != "fresh.txt" {
		t.Fatalf("открыт %q, ожидался fresh.txt", got)
	}
	model = openAsync(model, key('h'))

	if footer := footerOf(model); !strings.Contains(footer, "fresh.txt has no history yet") {
		t.Fatalf("подвал %q, ожидалась заметка", footer)
	}
	if body := screenText(model); strings.Contains(body, "Log") {
		t.Fatalf("окно лога открылось:\n%s", body)
	}
}

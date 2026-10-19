package git_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/i18n"
)

// Неотслеженный файл попадает в новый корневой .gitignore точным путём и
// пропадает из неотслеженных; повтор файл не меняет (#214, случаи 1, 2 и 4).
func TestIgnoreAddsExactPath(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "logs/[1] *.log", "x\n")
	write(t, repo, "logs/2.log", "y\n")

	if err := open(t, repo).Ignore("logs/[1] *.log"); err != nil {
		t.Fatalf("Ignore: %v", err)
	}
	if err := open(t, repo).Ignore("logs/[1] *.log"); err != nil {
		t.Fatalf("повторный Ignore: %v", err)
	}

	if got := readFile(t, filepath.Join(repo, ".gitignore")); got != "/logs/\\[1] \\*.log\n" {
		t.Fatalf(".gitignore = %q", got)
	}
	untracked := runGit(t, repo, "ls-files", "--others", "--exclude-standard")
	if strings.Contains(untracked, "[1]") || !strings.Contains(untracked, "logs/2.log") {
		t.Fatalf("неотслеженные %q: игнорироваться должен ровно один файл", untracked)
	}
}

// Строка дописывается с новой строки, даже если .gitignore не кончается
// переводом строки.
func TestIgnoreAppendsOnNewLine(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, ".gitignore", "*.tmp")
	commitAll(t, repo)
	write(t, repo, "new.txt", "x\n")

	if err := open(t, repo).Ignore("new.txt"); err != nil {
		t.Fatalf("Ignore: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, ".gitignore")); got != "*.tmp\n/new.txt\n" {
		t.Fatalf(".gitignore = %q", got)
	}
}

// Путь с «:» в начале не читается магией pathspec: правило «foo» не прячет
// файл «:foo», и он попадает в .gitignore (#301, случай 1).
func TestIgnoreLeadingColon(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, ".gitignore", "foo\n")
	commitAll(t, repo)
	write(t, repo, ":foo", "x\n")

	if err := open(t, repo).Ignore(":foo"); err != nil {
		t.Fatalf("Ignore: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, ".gitignore")); got != "foo\n/:foo\n" {
		t.Fatalf(".gitignore = %q", got)
	}
}

// Отслеженный файл git не игнорирует: Ignore отказывает и .gitignore не
// создаёт (случай 3).
func TestIgnoreRefusesTrackedFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	var refusal *i18n.Error
	if err := open(t, repo).Ignore("a.txt"); !errors.As(err, &refusal) || refusal.Key != "git.ignoreTracked" {
		t.Fatalf("Ignore = %v, ожидался отказ git.ignoreTracked", err)
	}
	if untracked := runGit(t, repo, "status", "--porcelain"); untracked != "" {
		t.Fatalf("репозиторий изменился: %q", untracked)
	}
}

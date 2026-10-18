package git_test

import (
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/git"
)

// mergeConflict это слияние topic в main, остановленное на конфликте в a.txt
// и b.bin; c.txt слился чисто.
func mergeConflict(t *testing.T) string {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "base\n")
	write(t, repo, "b.bin", "base\x00\n")
	write(t, repo, "c.txt", "c\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "topic")
	write(t, repo, "a.txt", "topic\n")
	write(t, repo, "b.bin", "topic\x00\n")
	write(t, repo, "c.txt", "c topic\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	write(t, repo, "a.txt", "main\n")
	write(t, repo, "b.bin", "main\x00\n")
	commitAll(t, repo)
	stopsOnConflict(t, repo, "merge", "topic")
	return repo
}

// Конфликтные пути видны списком (#242, случай 1); их сторона для всего
// файла ставит файл в индекс решённым, бинарный тоже (случаи 3 и 5).
func TestConflictedPathsAndResolveFile(t *testing.T) {
	repo := mergeConflict(t)

	paths, err := git.ConflictedPaths(repo)
	if err != nil || strings.Join(paths, ",") != "a.txt,b.bin" {
		t.Fatalf("ConflictedPaths = %q, %v", paths, err)
	}

	if err := git.ResolveFile(repo, "a.txt", true); err != nil {
		t.Fatalf("ResolveFile theirs: %v", err)
	}
	if err := git.ResolveFile(repo, "b.bin", false); err != nil {
		t.Fatalf("ResolveFile ours: %v", err)
	}

	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "topic\n" {
		t.Fatalf("a.txt = %q, ожидалась их сторона", got)
	}
	if got := readFile(t, filepath.Join(repo, "b.bin")); got != "main\x00\n" {
		t.Fatalf("b.bin = %q, ожидалась наша сторона", got)
	}
	if paths, _ := git.ConflictedPaths(repo); len(paths) != 0 {
		t.Fatalf("конфликты остались: %q", paths)
	}
}

// Текст файла читается и пишется по пути от корня.
func TestWorktreeFile(t *testing.T) {
	repo := mergeConflict(t)

	text, err := git.WorktreeFile(repo, "a.txt")
	if err != nil || !strings.HasPrefix(text, "<<<<<<< HEAD\nmain\n") {
		t.Fatalf("WorktreeFile = %q, %v", text, err)
	}
	if err := git.WriteWorktreeFile(repo, "a.txt", "resolved\n"); err != nil {
		t.Fatalf("WriteWorktreeFile: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "resolved\n" {
		t.Fatalf("a.txt = %q", got)
	}
}

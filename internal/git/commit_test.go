package git_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/git"
)

func lastSubject(t *testing.T, repo string) string {
	t.Helper()
	return strings.TrimSpace(runGit(t, repo, "log", "-1", "--format=%s"))
}

func TestCommitRecordsSubject(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "add", "a.txt")

	if err := git.Commit(repo, "поправить текст"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := lastSubject(t, repo); got != "поправить текст" {
		t.Fatalf("тема коммита %q", got)
	}
}

func TestCommitWithNothingStagedFails(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	err := git.Commit(repo, "пусто")
	if err == nil || !strings.Contains(err.Error(), "nothing to commit") {
		t.Fatalf("ошибка %v, ожидалось nothing to commit", err)
	}
}

// Упавший хук не даёт коммита: хуки не обходятся, а их отказ виден.
func TestFailingHookKeepsHead(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	before := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

	hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho отказ хука >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "add", "a.txt")

	if err := git.Commit(repo, "с хуком"); err == nil {
		t.Fatal("коммит прошёл, хотя хук упал")
	}
	if after := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD")); after != before {
		t.Fatal("HEAD сдвинулся при упавшем хуке")
	}
}

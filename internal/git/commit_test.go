package git_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/git"
	"gitone/internal/i18n"
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

// Пустой индекс: отказ из словаря, а не «On branch main» из вывода git commit.
func TestCommitWithNothingStagedFails(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	err := git.Commit(repo, "пусто")

	var refusal *i18n.Error
	if !errors.As(err, &refusal) || refusal.Key != "git.nothingToCommit" {
		t.Fatalf("ошибка %v, ожидался отказ git.nothingToCommit", err)
	}
	if err.Error() != "commit: nothing to commit" {
		t.Fatalf("ошибка %q, ожидалось «commit: nothing to commit»", err)
	}
}

// Первый коммит репозитория: HEAD ещё нет, а в индексе есть файл.
func TestCommitFirstCommit(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "a.txt")

	if err := git.Commit(repo, "первый"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := lastSubject(t, repo); got != "первый" {
		t.Fatalf("тема коммита %q", got)
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

package git_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// conflictBranches это main и topic, которые правят a.txt по-разному, и
// topic с двумя коммитами для rebase.
func conflictBranches(t *testing.T) string {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "base\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "topic")
	write(t, repo, "a.txt", "topic\n")
	commitAll(t, repo)
	write(t, repo, "b.txt", "topic b\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	write(t, repo, "a.txt", "main\n")
	commitAll(t, repo)
	return repo
}

func operationOf(t *testing.T, repo string) string {
	t.Helper()

	location, err := open(t, repo).Locate()
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	return location.Operation
}

// Abort слияния возвращает HEAD и файлы к состоянию до merge (#243, случай 1).
func TestAbortMerge(t *testing.T) {
	repo := conflictBranches(t)
	before := revision(t, repo, "HEAD")
	stopsOnConflict(t, repo, "merge", "topic")

	if got := operationOf(t, repo); got != "merge" {
		t.Fatalf("Operation = %q, ожидался merge", got)
	}
	if err := open(t, repo).Abort(); err != nil {
		t.Fatalf("Abort: %v", err)
	}
	if got := revision(t, repo, "HEAD"); got != before {
		t.Fatalf("HEAD %s, ожидался %s", got, before)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "main\n" {
		t.Fatalf("a.txt = %q после abort", got)
	}
	if got := operationOf(t, repo); got != "" {
		t.Fatalf("Operation = %q после abort", got)
	}
}

// Continue с нерешённым конфликтом отвергает git своей причиной (случай 3),
// после решения rebase применяет следующий коммит и заканчивается (случай 2).
func TestContinueRebase(t *testing.T) {
	repo := conflictBranches(t)
	runGit(t, repo, "switch", "-q", "topic")
	stopsOnConflict(t, repo, "rebase", "main")

	if got := operationOf(t, repo); got != "rebase" {
		t.Fatalf("Operation = %q, ожидался rebase", got)
	}
	if err := open(t, repo).Continue(); err == nil || !strings.HasPrefix(err.Error(), "rebase: ") {
		t.Fatalf("Continue с конфликтом = %v, ожидался отказ git", err)
	}

	write(t, repo, "a.txt", "resolved\n")
	runGit(t, repo, "add", "a.txt")
	if err := open(t, repo).Continue(); err != nil {
		t.Fatalf("Continue: %v", err)
	}
	if got := operationOf(t, repo); got != "" {
		t.Fatalf("Operation = %q после continue", got)
	}
	if got := strings.TrimSpace(runGit(t, repo, "show", "HEAD:b.txt")); got != "topic b" {
		t.Fatalf("второй коммит topic не применён: b.txt = %q", got)
	}
}

// Continue после решения первого конфликта дошёл до второго: ошибка говорит,
// что rebase снова остановился (#300, случай 4).
func TestContinueStopsOnNextConflict(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "base\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "topic")
	for _, content := range []string{"t1\n", "t2\n"} {
		write(t, repo, "a.txt", content)
		commitAll(t, repo)
	}
	runGit(t, repo, "switch", "-q", "main")
	write(t, repo, "a.txt", "main\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "topic")
	stopsOnConflict(t, repo, "rebase", "main")

	write(t, repo, "a.txt", "resolved\n")
	runGit(t, repo, "add", "a.txt")
	refusedWith(t, open(t, repo).Continue(), "git.stoppedOnConflict")
}

// У bisect нет continue и abort: оба отказывают ошибкой из словаря (#300,
// случай 2).
func TestContinueRefusesBisect(t *testing.T) {
	repo := eightCommits(t)
	if _, err := open(t, repo).BisectMark("HEAD", false); err != nil {
		t.Fatalf("BisectMark: %v", err)
	}

	refusedWith(t, open(t, repo).Continue(), "git.bisectNoContinue")
	refusedWith(t, open(t, repo).Abort(), "git.bisectNoContinue")
}

// Упавший хук при продолжении слияния виден своим текстом, а не остановкой
// на конфликте (#300, случай 3).
func TestContinueShowsHookFailure(t *testing.T) {
	repo := conflictBranches(t)
	stopsOnConflict(t, repo, "merge", "topic")
	write(t, repo, "a.txt", "resolved\n")
	runGit(t, repo, "add", "a.txt")
	write(t, repo, ".git/hooks/pre-commit", "#!/bin/sh\necho hook says no >&2\nexit 1\n")
	if err := os.Chmod(filepath.Join(repo, ".git/hooks/pre-commit"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := open(t, repo).Continue(); err == nil || !strings.Contains(err.Error(), "hook says no") {
		t.Fatalf("Continue = %v, ожидался текст хука", err)
	}
}

// Остановленный git am это своя операция: не rebase, и abort его прерывает
// (#300, случай 1).
func TestAmOperation(t *testing.T) {
	repo := conflictBranches(t)
	patch := runGit(t, repo, "format-patch", "-1", "--stdout", "topic~1")
	write(t, repo, "topic.patch", patch)
	stopsOnConflict(t, repo, "am", "topic.patch")

	if got := operationOf(t, repo); got != "am" {
		t.Fatalf("Operation = %q, ожидался am", got)
	}
	if err := open(t, repo).Abort(); err != nil {
		t.Fatalf("Abort: %v", err)
	}
	if got := operationOf(t, repo); got != "" {
		t.Fatalf("Operation = %q после abort", got)
	}
}

// Abort rebase возвращает ветку на исходный коммит (случай 4); без операции
// отказ из словаря.
func TestAbortRebase(t *testing.T) {
	repo := conflictBranches(t)
	runGit(t, repo, "switch", "-q", "topic")
	before := revision(t, repo, "HEAD")
	stopsOnConflict(t, repo, "rebase", "main")

	if err := open(t, repo).Abort(); err != nil {
		t.Fatalf("Abort: %v", err)
	}
	if got := revision(t, repo, "topic"); got != before {
		t.Fatalf("topic на %s, ожидался %s", got, before)
	}
	refusedWith(t, open(t, repo).Abort(), "git.noOperation")
}

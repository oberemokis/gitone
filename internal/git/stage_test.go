package git_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gitone/internal/git"
)

// Файл в индексе попадает в StagedPaths, а Unstage убирает его оттуда.
func TestStageAndUnstage(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	if err := git.Stage(repo, []string{"a.txt"}); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if staged := mustStaged(t, repo); !slices.Equal(staged, []string{"a.txt"}) {
		t.Fatalf("в индексе %q, ожидался a.txt", staged)
	}

	if err := git.Unstage(repo, []string{"a.txt"}); err != nil {
		t.Fatalf("Unstage: %v", err)
	}
	if staged := mustStaged(t, repo); len(staged) != 0 {
		t.Fatalf("в индексе %q, ожидалось пусто", staged)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "two\n" {
		t.Fatalf("правка в дереве пропала: %q", got)
	}
}

// Discard возвращает файл к HEAD, а правка в дереве уходит.
func TestDiscardRestoresTrackedFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	if err := git.Discard(repo, []string{"a.txt"}); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "one\n" {
		t.Fatalf("файл %q, ожидался возврат к HEAD", got)
	}
}

// Неотслеженный файл удаляется с диска.
func TestDiscardRemovesUntrackedFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "keep.txt", "k\n")
	commitAll(t, repo)
	write(t, repo, "new.txt", "fresh\n")

	if err := git.Discard(repo, []string{"new.txt"}); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("неотслеженный файл остался: %v", err)
	}
}

// Файл, добавленный в индекс и не бывший в HEAD, уходит и из индекса, и с диска.
func TestDiscardDropsStagedNewFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "keep.txt", "k\n")
	commitAll(t, repo)
	write(t, repo, "new.txt", "fresh\n")
	runGit(t, repo, "add", "new.txt")

	if err := git.Discard(repo, []string{"new.txt"}); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if staged := mustStaged(t, repo); len(staged) != 0 {
		t.Fatalf("в индексе %q, ожидалось пусто", staged)
	}
	if _, err := os.Stat(filepath.Join(repo, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("файл остался на диске: %v", err)
	}
}

func mustStaged(t *testing.T, repo string) []string {
	t.Helper()

	staged, err := git.StagedPaths(repo)
	if err != nil {
		t.Fatalf("StagedPaths: %v", err)
	}
	return staged
}

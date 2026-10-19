package git_test

import (
	"errors"
	"os/exec"
	"path/filepath"
	"testing"

	"gitone/internal/diff"
)

// Аргументы из подкаталога считаются от него, как в git: gitone file.txt в
// подкаталоге sub показывает sub/file.txt, а не file.txt в корне.
func TestArgumentsFromSubdirectoryAreRelative(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "sub/file.txt", "one\n")
	write(t, repo, "file.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "sub/file.txt", "two\n")
	write(t, repo, "file.txt", "two\n")

	files := load(t, filepath.Join(repo, "sub"), []string{"file.txt"})

	wantStatuses(t, files, map[string]diff.Status{"sub/file.txt": diff.StatusModified})
}

// Отказ git несёт исходную ошибку процесса: код выхода находится через
// errors.As, а не теряется в тексте.
func TestGitErrorKeepsExitCode(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	_, err := open(t, repo).Diff([]string{"main..nope"})

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Diff(неизвестная ревизия) = %v, want *exec.ExitError в цепочке", err)
	}
	if exitErr.ExitCode() != 128 {
		t.Errorf("Diff(неизвестная ревизия) код выхода = %d, want 128", exitErr.ExitCode())
	}
}

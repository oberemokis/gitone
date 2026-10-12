package git_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitone/internal/diff"
)

// Правка с тем же размером, записанная в ту же тику времени, что и индекс, видна
// в диффе. git считает такую запись «racy» и сверяет её содержимое, но только
// пока время записи индекса не новее записи. Времена выставляются явно, поэтому
// тест не зависит от точности часов файловой системы.
func TestSameSizeEditInSameTickIsShown(t *testing.T) {
	repo := newRepo(t)
	runGit(t, repo, "config", "core.trustctime", "false")
	tick := time.Unix(1_700_000_000, 0)

	write(t, repo, "a.txt", "one\n")
	if err := os.Chtimes(filepath.Join(repo, "a.txt"), tick, tick); err != nil {
		t.Fatal(err)
	}
	commitAll(t, repo)
	if err := os.Chtimes(filepath.Join(repo, ".git", "index"), tick, tick); err != nil {
		t.Fatal(err)
	}

	write(t, repo, "a.txt", "two\n")
	if err := os.Chtimes(filepath.Join(repo, "a.txt"), tick, tick); err != nil {
		t.Fatal(err)
	}

	files := load(t, repo, nil)
	wantStatuses(t, files, map[string]diff.Status{"a.txt": diff.StatusModified})
}

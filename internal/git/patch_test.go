package git_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// Патч коммита сохраняется в корень файлом 0001-<тема>.patch и возвращается
// текстом format-patch, у первого коммита тоже (#224, случаи 2 и 3).
func TestPatch(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "first change")

	name, patch, err := open(t, repo).Patch(revision(t, repo, "HEAD"))
	if err != nil {
		t.Fatalf("Patch: %v", err)
	}

	if name != "0001-first-change.patch" {
		t.Fatalf("файл %q, ожидался 0001-first-change.patch", name)
	}
	if !strings.Contains(patch, "Subject: [PATCH] first change") || !strings.Contains(patch, "+one") {
		t.Fatalf("патч без темы или правки:\n%s", patch)
	}
	if saved := readFile(t, filepath.Join(repo, name)); saved != patch {
		t.Fatal("файл патча расходится с текстом")
	}
}

// Merge-коммит отвергается до format-patch, файла патча нет (#292, случай 1).
func TestPatchRefusesMerge(t *testing.T) {
	repo := divergedRepo(t)
	runGit(t, repo, "merge", "-q", "--no-edit", "feature")

	_, _, err := open(t, repo).Patch(revision(t, repo, "HEAD"))
	refusedWith(t, err, "git.patchMerge")
	if patches, _ := filepath.Glob(filepath.Join(repo, "*.patch")); len(patches) != 0 {
		t.Fatalf("файлы патча остались: %q", patches)
	}
}

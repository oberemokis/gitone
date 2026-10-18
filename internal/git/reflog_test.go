package git_test

import (
	"testing"

	"gitone/internal/git"
)

// Reflog отдаёт записи от новых к старым с коммитом, действием и описанием
// (#240, случай 1); у первого коммита помечен корень.
func TestReflog(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	first := revision(t, repo, "HEAD")
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)
	runGit(t, repo, "checkout", "-q", first)

	entries, err := git.Reflog(repo)
	if err != nil || len(entries) != 3 {
		t.Fatalf("Reflog = %+v, %v", entries, err)
	}
	if got := entries[0]; got.Hash != first || got.Action != "checkout" || got.Subject != "moving from main to "+first {
		t.Fatalf("первая запись %+v, ожидался checkout", got)
	}
	if got := entries[2]; got.Action != "commit (initial)" || got.Subject != "commit" || !got.Root || got.When == "" {
		t.Fatalf("последняя запись %+v, ожидался первый коммит", got)
	}
}

// Репозиторий без коммитов даёт пустой reflog, а не ошибку.
func TestReflogEmpty(t *testing.T) {
	if entries, err := git.Reflog(newRepo(t)); err != nil || len(entries) != 0 {
		t.Fatalf("Reflog = %+v, %v", entries, err)
	}
}

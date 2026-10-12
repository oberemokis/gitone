package git_test

import (
	"testing"

	"gitone/internal/git"
)

// Коммиты идут от новых к старым, корневой помечен, у остальных родитель есть.
func TestLogListsNewestFirst(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)

	commits, err := git.Log(repo, 200)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("коммитов %d, ожидалось 2", len(commits))
	}
	if commits[0].Subject != "commit" || commits[0].Root {
		t.Fatalf("первый коммит %+v", commits[0])
	}
	if !commits[1].Root {
		t.Fatalf("корневой коммит не помечен: %+v", commits[1])
	}
}

func TestLogOfEmptyRepositoryIsEmpty(t *testing.T) {
	repo := newRepo(t)

	commits, err := git.Log(repo, 200)
	if err != nil || len(commits) != 0 {
		t.Fatalf("коммиты %v, ошибка %v; ожидался пустой список без ошибки", commits, err)
	}
}

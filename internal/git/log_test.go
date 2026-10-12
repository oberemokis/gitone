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

// Слияние ветки рисуется, как в git log --graph: у коммитов свой рисунок,
// между ними строки соединения без коммита.
func TestLogDrawsGraphOfMerge(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "side")
	write(t, repo, "b.txt", "side\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	write(t, repo, "c.txt", "main\n")
	commitAll(t, repo)
	runGit(t, repo, "merge", "-q", "--no-ff", "-m", "merge", "side")

	entries, err := git.Log(repo, 200)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	want := []string{"*", `|\`, "| *", "* |", "|/", "*"}
	commits := []bool{true, false, true, true, false, true}
	if len(entries) != len(want) {
		t.Fatalf("строк %d, ожидалось %d: %+v", len(entries), len(want), entries)
	}
	for index, entry := range entries {
		if entry.Graph != want[index] || (entry.Hash != "") != commits[index] {
			t.Fatalf("строка %d: граф %q, хэш %q; ожидался граф %q, коммит %v", index, entry.Graph, entry.Hash, want[index], commits[index])
		}
	}
	if entries[0].Subject != "merge" || !entries[5].Root {
		t.Fatalf("слияние %+v, корень %+v", entries[0], entries[5])
	}
}

func TestLogOfEmptyRepositoryIsEmpty(t *testing.T) {
	repo := newRepo(t)

	commits, err := git.Log(repo, 200)
	if err != nil || len(commits) != 0 {
		t.Fatalf("коммиты %v, ошибка %v; ожидался пустой список без ошибки", commits, err)
	}
}

package git_test

import (
	"slices"
	"strings"
	"testing"

	"gitone/internal/git"
)

func branchNames(branches []git.Branch) []string {
	var names []string
	for _, branch := range branches {
		names = append(names, branch.Name)
	}
	return names
}

func currentBranch(branches []git.Branch) string {
	for _, branch := range branches {
		if branch.Current {
			return branch.Name
		}
	}
	return ""
}

func TestBranchesListsLocalWithCurrent(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "branch", "feature")

	branches, err := git.Branches(repo)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if want := []string{"feature", "main"}; !slices.Equal(branchNames(branches), want) {
		t.Fatalf("ветки %q, ожидались %q", branchNames(branches), want)
	}
	if got := currentBranch(branches); got != "main" {
		t.Fatalf("текущая %q, ожидалась main", got)
	}
}

// У ветки автор и время её последнего коммита: git не хранит, кто создал
// ветку (#124).
func TestBranchesCarryLastCommit(t *testing.T) {
	repo := newRepo(t)
	t.Setenv("GIT_AUTHOR_NAME", "ann")
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "feature")
	t.Setenv("GIT_AUTHOR_NAME", "bob")
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)

	branches, err := git.Branches(repo)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	authors := map[string]string{"feature": "bob", "main": "ann"}
	for _, branch := range branches {
		if branch.Author != authors[branch.Name] || !strings.HasSuffix(branch.When, " ago") {
			t.Errorf("ветка %s: автор %q, время %q; ожидался автор %q и «… ago»", branch.Name, branch.Author, branch.When, authors[branch.Name])
		}
	}
}

func TestSwitchChangesCurrentBranch(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "branch", "feature")

	if err := git.Switch(repo, "feature"); err != nil {
		t.Fatalf("Switch: %v", err)
	}
	branches, _ := git.Branches(repo)
	if got := currentBranch(branches); got != "feature" {
		t.Fatalf("текущая %q, ожидалась feature", got)
	}
}

// Правка, которой мешает переключению, остаётся: git отказывает, ветка прежняя.
func TestSwitchRefusedKeepsBranch(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "checkout", "-q", "-b", "feature")
	write(t, repo, "a.txt", "feature\n")
	commitAll(t, repo)
	runGit(t, repo, "checkout", "-q", "main")
	write(t, repo, "a.txt", "local\n")

	err := git.Switch(repo, "feature")
	if err == nil || !strings.Contains(err.Error(), "local changes") {
		t.Fatalf("ошибка %v, ожидался отказ из-за локальных правок", err)
	}
	branches, _ := git.Branches(repo)
	if got := currentBranch(branches); got != "main" {
		t.Fatalf("текущая %q, ожидалась main", got)
	}
}

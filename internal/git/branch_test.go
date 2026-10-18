package git_test

import (
	"slices"
	"strings"
	"testing"

	"gitone/internal/git"
)

// Текущая ветка переименовывается и остаётся текущей, связь с upstream
// сохраняется, занятое имя git отвергает (#228).
func TestRenameBranch(t *testing.T) {
	_, local := remotePair(t)
	runGit(t, local, "branch", "taken")

	if err := git.RenameBranch(local, "main", "trunk"); err != nil {
		t.Fatalf("RenameBranch: %v", err)
	}
	if got := strings.TrimSpace(runGit(t, local, "branch", "--show-current")); got != "trunk" {
		t.Fatalf("текущая %q, ожидалась trunk", got)
	}
	if got := strings.TrimSpace(runGit(t, local, "rev-parse", "--abbrev-ref", "trunk@{upstream}")); got != "origin/main" {
		t.Fatalf("upstream %q, ожидался origin/main", got)
	}

	err := git.RenameBranch(local, "trunk", "taken")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("RenameBranch в занятое имя = %v, ожидался отказ «already exists»", err)
	}
}

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

// Тег с именем ветки делает короткое имя неоднозначным: git отдаёт heads/v1.
// В списке имя целое, и Switch по нему работает.
func TestBranchesKeepNameWhenTagSharesIt(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "branch", "v1")
	runGit(t, repo, "tag", "v1")

	branches, err := git.Branches(repo)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if want := []string{"main", "v1"}; !slices.Equal(branchNames(branches), want) {
		t.Fatalf("ветки %q, ожидались %q", branchNames(branches), want)
	}
	if err := git.Switch(repo, branches[1].Name); err != nil {
		t.Fatalf("Switch(%q): %v", branches[1].Name, err)
	}
	after, _ := git.Branches(repo)
	if got := currentBranch(after); got != "v1" {
		t.Fatalf("текущая %q, ожидалась v1", got)
	}
}

// Локальная ветка origin/x рядом с удалённой origin/x: короткое имя
// неоднозначно, и git снова добавляет heads/.
func TestBranchesKeepNameOfBranchLikeRemote(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "branch", "origin/x")
	runGit(t, repo, "update-ref", "refs/remotes/origin/x", "HEAD")

	branches, err := git.Branches(repo)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if want := []string{"main", "origin/x"}; !slices.Equal(branchNames(branches), want) {
		t.Fatalf("ветки %q, ожидались %q", branchNames(branches), want)
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

// Разделитель полей может оказаться в имени автора: ветка остаётся в списке,
// автор целый.
func TestBranchesKeepBranchWithSeparatorInAuthor(t *testing.T) {
	repo := newRepo(t)
	t.Setenv("GIT_AUTHOR_NAME", "ann\x1fbob")
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	branches, err := git.Branches(repo)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}

	if len(branches) != 1 {
		t.Fatalf("веток %d, ожидалась 1: %+v", len(branches), branches)
	}
	if branch := branches[0]; branch.Name != "main" || !branch.Current || branch.Author != "ann\x1fbob" || !strings.HasSuffix(branch.When, " ago") {
		t.Fatalf("ветка %+v, ожидалась текущая main автора «ann\\x1fbob»", branch)
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

// Новая ветка встаёт на базу и становится текущей (#227).
func TestCreateBranchSwitchesToIt(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	base := revision(t, repo, "HEAD")
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)

	if err := git.CreateBranch(repo, "feature", base); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if got := strings.TrimSpace(runGit(t, repo, "branch", "--show-current")); got != "feature" {
		t.Fatalf("текущая %q, ожидалась feature", got)
	}
	if got := revision(t, repo, "HEAD"); got != base {
		t.Fatalf("HEAD %s, ожидалась база %s", got, base)
	}
}

// Недопустимое и занятое имя git отвергает своим текстом, ветка прежняя.
func TestCreateBranchRefusesBadOrTakenName(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "branch", "taken")

	for name, reason := range map[string]string{"a b": "not a valid branch name", "taken": "already exists"} {
		err := git.CreateBranch(repo, name, "main")
		if err == nil || !strings.Contains(err.Error(), reason) {
			t.Errorf("CreateBranch(%q) = %v, ожидался отказ «%s»", name, err, reason)
		}
	}
	if got := strings.TrimSpace(runGit(t, repo, "branch", "--show-current")); got != "main" {
		t.Fatalf("текущая %q, ожидалась main", got)
	}
}

// Слитая ветка удаляется, неслитую и текущую git не удаляет (#227).
func TestDeleteBranch(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "branch", "merged")
	runGit(t, repo, "switch", "-q", "-c", "unmerged")
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")

	if err := git.DeleteBranch(repo, "merged"); err != nil {
		t.Fatalf("DeleteBranch(merged): %v", err)
	}
	if err := git.DeleteBranch(repo, "unmerged"); err == nil || !strings.Contains(err.Error(), "not fully merged") {
		t.Fatalf("DeleteBranch(unmerged) = %v, ожидался отказ «not fully merged»", err)
	}
	if err := git.DeleteBranch(repo, "main"); err == nil {
		t.Fatal("DeleteBranch(main) удалил текущую ветку")
	}

	branches, _ := git.Branches(repo)
	if want := []string{"main", "unmerged"}; !slices.Equal(branchNames(branches), want) {
		t.Fatalf("ветки %q, ожидались %q", branchNames(branches), want)
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

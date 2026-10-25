package git_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gitone/internal/git"
	"gitone/internal/i18n"
)

// Ветка без upstream получает выбранную удалённую, уже привязанная её
// меняет, отсутствующую удалённую git отвергает (#230, случаи 1-3). Список
// удалённых веток без origin/HEAD.
func TestSetUpstream(t *testing.T) {
	_, local := remotePair(t)
	runGit(t, local, "push", "-q", "origin", "main:other")
	runGit(t, local, "remote", "set-head", "origin", "main")
	runGit(t, local, "switch", "-q", "-c", "topic")
	upstream := func() string {
		return strings.TrimSpace(runGit(t, local, "rev-parse", "--abbrev-ref", "topic@{upstream}"))
	}

	remotes, err := open(t, local).RemoteBranches()
	if want := []string{"origin/main", "origin/other"}; err != nil || !slices.Equal(remotes, want) {
		t.Fatalf("RemoteBranches = %q, %v; ожидались %q", remotes, err, want)
	}

	if err := open(t, local).SetUpstream("topic", "origin/other"); err != nil || upstream() != "origin/other" {
		t.Fatalf("SetUpstream = %v, upstream %q", err, upstream())
	}
	if err := open(t, local).SetUpstream("topic", "origin/main"); err != nil || upstream() != "origin/main" {
		t.Fatalf("замена upstream = %v, upstream %q", err, upstream())
	}
	if err := open(t, local).SetUpstream("topic", "origin/missing"); err == nil || !strings.HasPrefix(err.Error(), "branch: ") {
		t.Fatalf("SetUpstream к отсутствующей = %v, ожидался отказ git", err)
	}
}

// divergedRepo это main и feature, разошедшиеся от общего коммита: у каждой
// свой файл. Текущая main.
func divergedRepo(t *testing.T) string {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "feature")
	write(t, repo, "f.txt", "feature\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	write(t, repo, "m.txt", "main\n")
	commitAll(t, repo)
	return repo
}

// Отставшая ветка сливается перемоткой, без merge-коммита (#229, случай 1).
func TestMergeFastForwards(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "feature")
	write(t, repo, "f.txt", "feature\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")

	if err := open(t, repo).Merge("feature"); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got, want := revision(t, repo, "main"), revision(t, repo, "feature"); got != want {
		t.Fatalf("main %s, ожидалась перемотка на feature %s", got, want)
	}
}

// Разошедшиеся ветки дают merge-коммит с сообщением по умолчанию (случай 2).
func TestMergeCreatesMergeCommit(t *testing.T) {
	repo := divergedRepo(t)

	if err := open(t, repo).Merge("feature"); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if parents := strings.Fields(runGit(t, repo, "log", "-1", "--format=%P")); len(parents) != 2 {
		t.Fatalf("у HEAD родителей %d, ожидался merge-коммит", len(parents))
	}
	if got := lastSubject(t, repo); got != "Merge branch 'feature'" {
		t.Fatalf("сообщение %q, ожидалось по умолчанию", got)
	}
}

// Rebase ставит коммиты main поверх feature, история линейна (случай 3).
func TestRebaseMakesLinearHistory(t *testing.T) {
	repo := divergedRepo(t)

	if err := open(t, repo).Rebase("feature"); err != nil {
		t.Fatalf("Rebase: %v", err)
	}
	if got, want := revision(t, repo, "HEAD~1"), revision(t, repo, "feature"); got != want {
		t.Fatalf("родитель HEAD %s, ожидалась вершина feature %s", got, want)
	}
	if merges := strings.TrimSpace(runGit(t, repo, "rev-list", "--merges", "HEAD")); merges != "" {
		t.Fatalf("в истории merge-коммиты: %s", merges)
	}
}

// Тег с именем ветки на другом коммите не подменяет ветку: merge сливает её с
// обычным сообщением, rebase идёт поверх неё (#291, случаи 1 и 2).
func TestMergeAndRebaseTakeBranchOverTag(t *testing.T) {
	repo := divergedRepo(t)
	runGit(t, repo, "tag", "feature", "main~1")

	if err := open(t, repo).Merge("feature"); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if parents := strings.Fields(runGit(t, repo, "log", "-1", "--format=%P")); len(parents) != 2 || parents[1] != revision(t, repo, "refs/heads/feature") {
		t.Fatalf("родители HEAD %q, ожидалась вершина ветки feature", parents)
	}
	if got := lastSubject(t, repo); got != "Merge branch 'feature'" {
		t.Fatalf("сообщение %q, ожидалось по умолчанию", got)
	}

	runGit(t, repo, "reset", "-q", "--hard", "HEAD~1")
	if err := open(t, repo).Rebase("feature"); err != nil {
		t.Fatalf("Rebase: %v", err)
	}
	if got, want := revision(t, repo, "HEAD~1"), revision(t, repo, "refs/heads/feature"); got != want {
		t.Fatalf("родитель HEAD %s, ожидалась вершина ветки feature %s", got, want)
	}
}

// Отложенные autostash правки, которые не встали обратно, дают ошибку, а не
// итог «слито»: git выходит с 0, правки лежат в stash (#388, случай 1).
func TestMergeAndRebaseAutostashConflict(t *testing.T) {
	cases := []struct {
		operation string
		run       func(git.Repo) error
	}{
		{"merge", func(repo git.Repo) error { return repo.Merge("other") }},
		{"rebase", func(repo git.Repo) error { return repo.Rebase("other") }},
	}

	for _, tc := range cases {
		t.Run(tc.operation, func(t *testing.T) {
			repo := newRepo(t)
			write(t, repo, "a.txt", "one\n")
			commitAll(t, repo)
			runGit(t, repo, "switch", "-q", "-c", "other")
			write(t, repo, "a.txt", "other\n")
			commitAll(t, repo)
			runGit(t, repo, "switch", "-q", "main")
			runGit(t, repo, "config", tc.operation+".autoStash", "true")
			write(t, repo, "a.txt", "dirty\n")

			refusedWith(t, tc.run(open(t, repo)), "git.autostashConflict")
			if stash := runGit(t, repo, "stash", "list"); stash == "" {
				t.Fatal("отложенных правок нет в stash")
			}
		})
	}
}

// Правка, которую затронет операция, останавливает её до начала одной строкой
// git (случай 4), а конфликт останавливает её посредине: ошибка говорит, что
// операция не закончена (случай 5).
func TestMergeAndRebaseRefusals(t *testing.T) {
	repo := divergedRepo(t)
	write(t, repo, "a.txt", "dirty\n")
	err := open(t, repo).Rebase("feature")
	if err == nil || !strings.HasPrefix(err.Error(), "rebase: error:") || strings.ContainsAny(err.Error(), "\r\n") {
		t.Fatalf("Rebase на грязном дереве = %q, ожидалась одна строка error:", err)
	}
	runGit(t, repo, "checkout", "--", "a.txt")

	runGit(t, repo, "switch", "-q", "feature")
	write(t, repo, "m.txt", "feature side\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")

	var stopped *i18n.Error
	if err := open(t, repo).Merge("feature"); !errors.As(err, &stopped) || stopped.Key != "git.stoppedOnConflict" {
		t.Fatalf("Merge с конфликтом = %v, ожидалась git.stoppedOnConflict", err)
	}
	if _, statErr := os.Stat(filepath.Join(repo, ".git", "MERGE_HEAD")); statErr != nil {
		t.Fatalf("слияние не осталось незавершённым: %v", statErr)
	}

	runGit(t, repo, "merge", "--abort")
	if err := open(t, repo).Rebase("feature"); !errors.As(err, &stopped) || stopped.Key != "git.stoppedOnConflict" {
		t.Fatalf("Rebase с конфликтом = %v, ожидалась git.stoppedOnConflict", err)
	}
}

// Текущая ветка переименовывается и остаётся текущей, связь с upstream
// сохраняется, занятое имя git отвергает (#228).
func TestRenameBranch(t *testing.T) {
	_, local := remotePair(t)
	runGit(t, local, "branch", "taken")

	if err := open(t, local).RenameBranch("main", "trunk"); err != nil {
		t.Fatalf("RenameBranch: %v", err)
	}
	if got := strings.TrimSpace(runGit(t, local, "branch", "--show-current")); got != "trunk" {
		t.Fatalf("текущая %q, ожидалась trunk", got)
	}
	if got := strings.TrimSpace(runGit(t, local, "rev-parse", "--abbrev-ref", "trunk@{upstream}")); got != "origin/main" {
		t.Fatalf("upstream %q, ожидался origin/main", got)
	}

	err := open(t, local).RenameBranch("trunk", "taken")
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

	branches, err := open(t, repo).Branches()
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

// Checkout коммита отсоединяет HEAD на нём, правка, которой переход мешает,
// его останавливает (#223, случаи 2 и 3).
func TestCheckoutCommit(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	first := revision(t, repo, "HEAD")
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)

	write(t, repo, "a.txt", "dirty\n")
	if err := open(t, repo).CheckoutCommit(first); err == nil || !strings.HasPrefix(err.Error(), "switch: error:") {
		t.Fatalf("CheckoutCommit на грязном дереве = %v, ожидался отказ git", err)
	}
	runGit(t, repo, "checkout", "--", "a.txt")

	if err := open(t, repo).CheckoutCommit(first); err != nil {
		t.Fatalf("CheckoutCommit: %v", err)
	}
	if got := revision(t, repo, "HEAD"); got != first {
		t.Fatalf("HEAD %s, ожидался %s", got, first)
	}
	if branch := strings.TrimSpace(runGit(t, repo, "branch", "--show-current")); branch != "" {
		t.Fatalf("HEAD на ветке %q, ожидался отсоединённый", branch)
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

	branches, err := open(t, repo).Branches()
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	if want := []string{"main", "v1"}; !slices.Equal(branchNames(branches), want) {
		t.Fatalf("ветки %q, ожидались %q", branchNames(branches), want)
	}
	if err := open(t, repo).Switch(branches[1].Name); err != nil {
		t.Fatalf("Switch(%q): %v", branches[1].Name, err)
	}
	after, _ := open(t, repo).Branches()
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

	branches, err := open(t, repo).Branches()
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

	branches, err := open(t, repo).Branches()
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	authors := map[string]string{"feature": "bob", "main": "ann"}
	if len(branches) != len(authors) {
		t.Fatalf("веток %d, ожидалось %d: пустой список прошёл бы цикл молча", len(branches), len(authors))
	}
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

	branches, err := open(t, repo).Branches()
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

	if err := open(t, repo).Switch("feature"); err != nil {
		t.Fatalf("Switch: %v", err)
	}
	branches, _ := open(t, repo).Branches()
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

	if err := open(t, repo).CreateBranch("feature", base); err != nil {
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
		err := open(t, repo).CreateBranch(name, "main")
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

	if err := open(t, repo).DeleteBranch("merged"); err != nil {
		t.Fatalf("DeleteBranch(merged): %v", err)
	}
	if err := open(t, repo).DeleteBranch("unmerged"); err == nil || !strings.Contains(err.Error(), "not fully merged") {
		t.Fatalf("DeleteBranch(unmerged) = %v, ожидался отказ «not fully merged»", err)
	}
	if err := open(t, repo).DeleteBranch("main"); err == nil {
		t.Fatal("DeleteBranch(main) удалил текущую ветку")
	}

	branches, _ := open(t, repo).Branches()
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

	err := open(t, repo).Switch("feature")
	if err == nil || !strings.Contains(err.Error(), "local changes") {
		t.Fatalf("ошибка %v, ожидался отказ из-за локальных правок", err)
	}
	branches, _ := open(t, repo).Branches()
	if got := currentBranch(branches); got != "main" {
		t.Fatalf("текущая %q, ожидалась main", got)
	}
}

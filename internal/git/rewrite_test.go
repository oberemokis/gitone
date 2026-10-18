package git_test

import (
	"strings"
	"testing"

	"gitone/internal/git"
)

// historyRepo это ветка из коммитов one, two, three, four: каждый пишет свой
// файл. Возвращает хэши от старых к новым.
func historyRepo(t *testing.T) (string, []string) {
	t.Helper()

	repo := newRepo(t)
	var hashes []string
	for _, name := range []string{"one", "two", "three", "four"} {
		write(t, repo, name+".txt", name+"\n")
		runGit(t, repo, "add", "-A")
		runGit(t, repo, "commit", "-q", "-m", name)
		hashes = append(hashes, revision(t, repo, "HEAD"))
	}
	return repo, hashes
}

// history это темы коммитов ветки от новых к старым через пробел.
func history(t *testing.T, repo string) string {
	t.Helper()
	return strings.Join(strings.Fields(runGit(t, repo, "log", "--format=%s")), " ")
}

// Squash, fixup, drop, reword и перенос меняют историю, как в lazygit, а
// незакоммиченная правка переживает rebase (#219, случаи 1-4).
func TestRewriteHistory(t *testing.T) {
	cases := []struct {
		name    string
		at      int
		action  git.Rewrite
		message string
		want    string
	}{
		// тема слитого коммита от старшего, «three» уходит в описание
		{name: "squash", at: 2, action: git.RewriteSquash, want: "four two one"},
		{name: "fixup", at: 2, action: git.RewriteFixup, want: "four two one"},
		{name: "drop", at: 1, action: git.RewriteDrop, want: "four three one"},
		{name: "reword", at: 1, action: git.RewriteReword, message: "second", want: "four three second one"},
		{name: "вверх", at: 1, action: git.RewriteMoveUp, want: "four two three one"},
		{name: "вниз", at: 2, action: git.RewriteMoveDown, want: "four two three one"},
		{name: "reword корня", at: 0, action: git.RewriteReword, message: "first", want: "four three two first"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, hashes := historyRepo(t)
			write(t, repo, "four.txt", "dirty\n")

			published, err := git.RewriteHistory(repo, hashes[tc.at], tc.action, tc.message)
			if err != nil || published {
				t.Fatalf("RewriteHistory = %v, %v", published, err)
			}
			if got := history(t, repo); got != tc.want {
				t.Fatalf("история %q, ожидалась %q", got, tc.want)
			}
			if got := readFile(t, repo+"/four.txt"); got != "dirty\n" {
				t.Fatalf("незакоммиченная правка потеряна: %q", got)
			}
		})
	}
}

// У squash склеиваются оба сообщения (случай 1), у корня squash нечего
// сливать, у HEAD некуда поднять.
func TestRewriteHistoryDetails(t *testing.T) {
	repo, hashes := historyRepo(t)

	if _, err := git.RewriteHistory(repo, hashes[2], git.RewriteSquash, ""); err != nil {
		t.Fatalf("squash: %v", err)
	}
	if got := strings.TrimSpace(runGit(t, repo, "log", "-1", "--skip=1", "--format=%B")); got != "two\n\nthree" {
		t.Fatalf("сообщение после squash %q, ожидались оба", got)
	}

	refusedWith(t, func() error { _, err := git.RewriteHistory(repo, hashes[0], git.RewriteSquash, ""); return err }(), "git.rewriteNoParent")
	head := revision(t, repo, "HEAD")
	refusedWith(t, func() error { _, err := git.RewriteHistory(repo, head, git.RewriteMoveUp, ""); return err }(), "git.rewriteNoChild")
}

// Конфликт при переносе останавливает rebase (случай 5).
func TestRewriteHistoryConflict(t *testing.T) {
	repo := newRepo(t)
	for _, content := range []string{"one\n", "two\n"} {
		write(t, repo, "a.txt", content)
		commitAll(t, repo)
	}
	write(t, repo, "a.txt", "three\n")
	commitAll(t, repo)
	middle := revision(t, repo, "HEAD~1")

	_, err := git.RewriteHistory(repo, middle, git.RewriteMoveUp, "")
	refusedWith(t, err, "git.stoppedOnConflict")
}

// Fixup из индекса создаёт коммит «fixup! <тема>», autosquash сливает его в
// цель; пустой индекс и коммит не из ветки отвергаются (#225, случаи 1-3).
func TestFixupAndAutosquash(t *testing.T) {
	repo, hashes := historyRepo(t)
	runGit(t, repo, "switch", "-q", "-c", "side", hashes[0])
	write(t, repo, "side.txt", "side\n")
	commitAll(t, repo)
	side := revision(t, repo, "HEAD")
	runGit(t, repo, "switch", "-q", "main")

	refusedWith(t, git.Fixup(repo, hashes[1]), "git.nothingToCommit")
	write(t, repo, "two.txt", "two fixed\n")
	runGit(t, repo, "add", "two.txt")
	refusedWith(t, git.Fixup(repo, side), "git.rewriteNotInBranch")

	if err := git.Fixup(repo, hashes[1]); err != nil {
		t.Fatalf("Fixup: %v", err)
	}
	if got := history(t, repo); got != "fixup! two four three two one" {
		t.Fatalf("история %q, ожидался fixup-коммит сверху", got)
	}

	if _, err := git.Autosquash(repo, hashes[1]); err != nil {
		t.Fatalf("Autosquash: %v", err)
	}
	if got := history(t, repo); got != "four three two one" {
		t.Fatalf("история %q, ожидался fixup внутри two", got)
	}
	if got := readFile(t, repo+"/two.txt"); got != "two fixed\n" {
		t.Fatalf("two.txt = %q, правка fixup потеряна", got)
	}
}

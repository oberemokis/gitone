package git_test

import (
	"path/filepath"
	"testing"

	"gitone/internal/git"
)

// Stash уносит правку и возвращает файл к HEAD. Apply возвращает правку и
// оставляет stash, Drop удаляет его.
func TestStashPushApplyDrop(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	if err := git.StashPush(repo); err != nil {
		t.Fatalf("StashPush: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "one\n" {
		t.Fatalf("после stash файл %q, ожидался возврат к HEAD", got)
	}
	stashes, err := git.Stashes(repo)
	if err != nil || len(stashes) != 1 {
		t.Fatalf("stash %v, ошибка %v; ожидался один", stashes, err)
	}

	if err := git.StashApply(repo, stashes[0].Ref); err != nil {
		t.Fatalf("StashApply: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "two\n" {
		t.Fatalf("после apply файл %q, ожидалась правка", got)
	}
	if again, _ := git.Stashes(repo); len(again) != 1 {
		t.Fatalf("apply удалил stash: %v", again)
	}

	if err := git.StashDrop(repo, stashes[0].Ref); err != nil {
		t.Fatalf("StashDrop: %v", err)
	}
	if left, _ := git.Stashes(repo); len(left) != 0 {
		t.Fatalf("drop оставил stash: %v", left)
	}
}

func TestStashPushWithNothingFails(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	if err := git.StashPush(repo); err == nil {
		t.Fatal("stash без правок прошёл")
	}
}

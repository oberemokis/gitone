package git_test

import (
	"errors"
	"path/filepath"
	"testing"

	"gitone/internal/i18n"
)

// refusedWith проверяет, что ошибка это отказ из словаря с ключом key.
func refusedWith(t *testing.T, err error, key string) {
	t.Helper()

	var refusal *i18n.Error
	if !errors.As(err, &refusal) || refusal.Key != key {
		t.Fatalf("ошибка %v, ожидался отказ %s", err, key)
	}
}

// Revert создаёт коммит с обратным диффом и сообщением по умолчанию, повторный
// revert отказывает и коммит не создаёт (#221, случаи 1 и 4).
func TestRevert(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)
	target := revision(t, repo, "HEAD")

	if err := open(t, repo).Revert(target); err != nil {
		t.Fatalf("Revert: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "one\n" {
		t.Fatalf("a.txt = %q, ожидался обратный дифф", got)
	}
	if got := lastSubject(t, repo); got != `Revert "commit"` {
		t.Fatalf("сообщение %q, ожидалось по умолчанию", got)
	}

	head := revision(t, repo, "HEAD")
	refusedWith(t, open(t, repo).Revert(target), "git.alreadyReverted")
	if got := revision(t, repo, "HEAD"); got != head {
		t.Fatal("повторный revert создал коммит")
	}
}

// Коммит из другой ветки переносится с тем же сообщением, повтор отказывает и
// коммит не создаёт, конфликт останавливает cherry-pick (#222, случаи 1, 3, 4).
func TestCherryPick(t *testing.T) {
	repo := divergedRepo(t)
	feature := revision(t, repo, "feature")

	if err := open(t, repo).CherryPick(feature); err != nil {
		t.Fatalf("CherryPick: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, "f.txt")); got != "feature\n" {
		t.Fatalf("f.txt = %q, ожидалась правка feature", got)
	}
	if got := lastSubject(t, repo); got != "commit" {
		t.Fatalf("сообщение %q, ожидалось сообщение коммита", got)
	}

	head := revision(t, repo, "HEAD")
	refusedWith(t, open(t, repo).CherryPick(feature), "git.alreadyPicked")
	if got := revision(t, repo, "HEAD"); got != head {
		t.Fatal("повторный cherry-pick создал коммит")
	}

	runGit(t, repo, "switch", "-q", "feature")
	write(t, repo, "m.txt", "feature side\n")
	commitAll(t, repo)
	conflicting := revision(t, repo, "HEAD")
	runGit(t, repo, "switch", "-q", "main")
	refusedWith(t, open(t, repo).CherryPick(conflicting), "git.stoppedOnConflict")
}

// Merge-коммит не отменяется: номер родителя не спрашиваем (случай 2).
// Конфликт останавливает revert незавершённым (случай 3).
func TestRevertRefusals(t *testing.T) {
	repo := divergedRepo(t)
	runGit(t, repo, "merge", "-q", "--no-edit", "feature")
	refusedWith(t, open(t, repo).Revert(revision(t, repo, "HEAD")), "git.revertMerge")

	write(t, repo, "f.txt", "changed later\n")
	commitAll(t, repo)
	err := open(t, repo).Revert(revision(t, repo, "feature"))
	refusedWith(t, err, "git.stoppedOnConflict")
	if got := operationOf(t, repo); got != "revert" {
		t.Fatalf("Operation = %q, revert не остался незавершённым", got)
	}
}

// Merge-коммит не переносится: номер родителя не спрашиваем. Пока слияние
// стоит на конфликте, cherry-pick отказывает словами git: он не начинался, и
// конфликт не его.
func TestCherryPickRefusals(t *testing.T) {
	repo := divergedRepo(t)
	runGit(t, repo, "merge", "-q", "--no-edit", "feature")
	refusedWith(t, open(t, repo).CherryPick(revision(t, repo, "HEAD")), "git.pickMerge")

	repo = conflictBranches(t)
	stopsOnConflict(t, repo, "merge", "topic")
	err := open(t, repo).CherryPick(revision(t, repo, "topic"))
	if err == nil || errorKey(err) == "git.stoppedOnConflict" {
		t.Fatalf("ошибка %v, ожидался отказ git при незавершённом слиянии", err)
	}
}

package git_test

import (
	"errors"
	"path/filepath"
	"testing"

	"gitone/internal/git"
	"gitone/internal/i18n"
)

func undo(t *testing.T, repo string) git.Step {
	t.Helper()

	step, err := open(t, repo).UndoStep()
	if err != nil {
		t.Fatalf("UndoStep: %v", err)
	}
	if err := open(t, repo).ApplyStep(step); err != nil {
		t.Fatalf("ApplyStep %+v: %v", step, err)
	}
	return step
}

func redo(t *testing.T, repo string) git.Step {
	t.Helper()

	step, err := open(t, repo).RedoStep()
	if err != nil {
		t.Fatalf("RedoStep: %v", err)
	}
	if err := open(t, repo).ApplyStep(step); err != nil {
		t.Fatalf("ApplyStep %+v: %v", step, err)
	}
	return step
}

func errorKey(err error) string {
	var keyed *i18n.Error
	if errors.As(err, &keyed) {
		return keyed.Key
	}
	return ""
}

// Откат коммита ставит HEAD на предыдущий, изменения коммита остаются в
// индексе; повтор возвращает коммит (#239, случаи 1 и 3).
func TestUndoCommitAndRedo(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	first := revision(t, repo, "HEAD")
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)
	second := revision(t, repo, "HEAD")

	if step := undo(t, repo); step.Action != "commit" || step.Target != first {
		t.Fatalf("откат %+v, ожидался коммит к %s", step, first)
	}
	if got := revision(t, repo, "HEAD"); got != first {
		t.Fatalf("HEAD %s после отката, ожидался %s", got, first)
	}
	if got := runGit(t, repo, "diff", "--cached", "--name-only"); got != "a.txt\n" {
		t.Fatalf("индекс %q, ожидались изменения коммита", got)
	}

	redo(t, repo)
	if got := revision(t, repo, "HEAD"); got != second {
		t.Fatalf("HEAD %s после повтора, ожидался %s", got, second)
	}

	// откат после повтора снова откатывает тот же коммит
	undo(t, repo)
	if got := revision(t, repo, "HEAD"); got != first {
		t.Fatalf("HEAD %s после второго отката, ожидался %s", got, first)
	}
}

// Два отката подряд идут дальше в прошлое, а не возвращают первый.
func TestUndoTwice(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	first := revision(t, repo, "HEAD")
	for _, content := range []string{"two\n", "three\n"} {
		write(t, repo, "a.txt", content)
		commitAll(t, repo)
	}

	undo(t, repo)
	undo(t, repo)
	if got := revision(t, repo, "HEAD"); got != first {
		t.Fatalf("HEAD %s после двух откатов, ожидался %s", got, first)
	}
}

// Откат перехода возвращает ветку, с которой был переход (случай 2).
func TestUndoCheckout(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "feature")

	if step := undo(t, repo); !step.Checkout || step.Target != "main" {
		t.Fatalf("откат %+v, ожидался переход на main", step)
	}
	if got := runGit(t, repo, "branch", "--show-current"); got != "main\n" {
		t.Fatalf("ветка %q, ожидалась main", got)
	}

	redo(t, repo)
	if got := runGit(t, repo, "branch", "--show-current"); got != "feature\n" {
		t.Fatalf("ветка %q после повтора, ожидалась feature", got)
	}
}

// Новое движение HEAD после отката сбрасывает повтор (случай 4).
func TestNewMoveDropsRedo(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)

	undo(t, repo)
	runGit(t, repo, "commit", "-q", "-m", "again")

	if _, err := open(t, repo).RedoStep(); errorKey(err) != "git.nothingToRedo" {
		t.Fatalf("RedoStep = %v, ожидался отказ", err)
	}
}

// Последним был stash: HEAD не двигался, откат отказывает, а не отменяет
// коммит до stash (случай 5). После pop откат снова работает.
func TestUndoAfterStash(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "dirty\n")
	runGit(t, repo, "stash", "push", "-q")

	if _, err := open(t, repo).UndoStep(); errorKey(err) != "git.undoStash" {
		t.Fatalf("UndoStep = %v, ожидался отказ из-за stash", err)
	}

	runGit(t, repo, "stash", "pop", "-q")
	if step, err := open(t, repo).UndoStep(); err != nil || step.Action != "commit" {
		t.Fatalf("UndoStep после pop = %+v, %v", step, err)
	}
}

// Без движений HEAD откатывать нечего, повторять тоже.
func TestNothingToUndo(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	if _, err := open(t, repo).UndoStep(); errorKey(err) != "git.nothingToUndo" {
		t.Fatalf("UndoStep = %v, ожидался отказ", err)
	}
	if _, err := open(t, repo).RedoStep(); errorKey(err) != "git.nothingToRedo" {
		t.Fatalf("RedoStep = %v, ожидался отказ", err)
	}
}

// Rebase откатывается одним шагом к коммиту до его начала.
func TestUndoRebase(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "feature")
	write(t, repo, "b.txt", "feature\n")
	commitAll(t, repo)
	before := revision(t, repo, "HEAD")
	runGit(t, repo, "switch", "-q", "main")
	write(t, repo, "c.txt", "main\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "feature")
	runGit(t, repo, "rebase", "-q", "main")

	undo(t, repo)
	if got := revision(t, repo, "HEAD"); got != before {
		t.Fatalf("HEAD %s после отката rebase, ожидался %s", got, before)
	}
}

// Pull с rebase откатывается одним шагом к коммиту до pull, и с --rebase, и
// с pull.rebase=true; pull перемоткой как раньше (#296, случаи 1–3).
func TestUndoPullRebase(t *testing.T) {
	cases := []struct {
		name string
		pull []string
	}{
		{"--rebase", []string{"pull", "-q", "--rebase"}},
		{"pull.rebase=true", []string{"-c", "pull.rebase=true", "pull", "-q"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			origin, local := remotePair(t)
			for _, name := range []string{"b.txt", "c.txt"} {
				write(t, local, name, name+"\n")
				commitAll(t, local)
			}
			before := revision(t, local, "HEAD")
			publish(t, origin, "d.txt", "peer\n")
			runGit(t, local, tc.pull...)

			undo(t, local)
			if got := revision(t, local, "HEAD"); got != before {
				t.Fatalf("HEAD %s после отката pull, ожидался %s", got, before)
			}
		})
	}

	origin, local := remotePair(t)
	before := revision(t, local, "HEAD")
	publish(t, origin, "d.txt", "peer\n")
	runGit(t, local, "pull", "-q", "--ff-only")
	undo(t, local)
	if got := revision(t, local, "HEAD"); got != before {
		t.Fatalf("HEAD %s после отката перемотки, ожидался %s", got, before)
	}
}

// Посреди операции откат и повтор отказывают: reset --soft снимал
// CHERRY_PICK_HEAD, и коммит пропадал из ветки, а посреди pull --rebase
// двигал HEAD внутри rebase (#385, случаи 1 и 2).
func TestUndoRefusesDuringOperation(t *testing.T) {
	t.Run("cherry-pick", func(t *testing.T) {
		repo := newRepo(t)
		write(t, repo, "a.txt", "one\n")
		commitAll(t, repo)
		runGit(t, repo, "switch", "-q", "-c", "other")
		write(t, repo, "a.txt", "other\n")
		commitAll(t, repo)
		runGit(t, repo, "switch", "-q", "main")
		write(t, repo, "a.txt", "main\n")
		commitAll(t, repo)
		stopsOnConflict(t, repo, "cherry-pick", "other")
		write(t, repo, "a.txt", "resolved\n")
		runGit(t, repo, "add", "a.txt")

		_, err := open(t, repo).UndoStep()
		refusedWith(t, err, "git.undoUnfinished")
		_, err = open(t, repo).RedoStep()
		refusedWith(t, err, "git.undoUnfinished")
		runGit(t, repo, "rev-parse", "--verify", "-q", "CHERRY_PICK_HEAD")
	})

	// первый коммит встаёт, второй конфликтует: rebase успел записать
	// «pull --rebase (pick)», и откат принимал её за шаг
	t.Run("pull --rebase", func(t *testing.T) {
		origin, local := remotePair(t)
		write(t, local, "c.txt", "local\n")
		commitAll(t, local)
		write(t, local, "b.txt", "local\n")
		commitAll(t, local)
		publish(t, origin, "b.txt", "peer\n")
		stopsOnConflict(t, local, "pull", "-q", "--rebase")

		_, err := open(t, local).UndoStep()
		refusedWith(t, err, "git.undoUnfinished")
	})
}

// switch -C и checkout -B переставляют текущую ветку записью «branch: Reset
// to», а следом пишут «checkout: moving from main to main». Откат возвращает
// ветку туда, где она стояла (случай 3).
func TestUndoBranchReset(t *testing.T) {
	for _, command := range [][]string{{"switch", "-q", "-C", "main", "other"}, {"checkout", "-q", "-B", "main", "other"}} {
		t.Run(command[0], func(t *testing.T) {
			repo := newRepo(t)
			write(t, repo, "a.txt", "one\n")
			commitAll(t, repo)
			runGit(t, repo, "branch", "other")
			write(t, repo, "a.txt", "two\n")
			commitAll(t, repo)
			second := revision(t, repo, "HEAD")
			runGit(t, repo, command...)

			undo(t, repo)
			if got := revision(t, repo, "main"); got != second {
				t.Fatalf("main %s после отката, ожидался %s", got, second)
			}
			if got := runGit(t, repo, "branch", "--show-current"); got != "main\n" {
				t.Fatalf("после отката ветка %q, ожидалась main", got)
			}
		})
	}
}

// Коммит из git am откатывается один, а повтор его возвращает (случай 4).
func TestUndoAm(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "p.txt", "patch\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "patch")
	patch := filepath.Join(t.TempDir(), "p.patch")
	runGit(t, repo, "format-patch", "-q", "-1", "-o", filepath.Dir(patch))
	runGit(t, repo, "reset", "-q", "--hard", "HEAD~1")
	write(t, repo, "m.txt", "mine\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "mine")
	mine := revision(t, repo, "HEAD")
	runGit(t, repo, "am", "-q", filepath.Join(filepath.Dir(patch), "0001-patch.patch"))
	applied := revision(t, repo, "HEAD")

	if step := undo(t, repo); step.Target != mine {
		t.Fatalf("откат %+v, ожидался к коммиту mine %s", step, mine)
	}
	redo(t, repo)
	if got := revision(t, repo, "HEAD"); got != applied {
		t.Fatalf("HEAD %s после повтора, ожидался коммит патча %s", got, applied)
	}
}

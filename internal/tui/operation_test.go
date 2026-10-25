package tui_test

import (
	"os/exec"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// mergeStopped это слияние topic в main, остановленное на конфликте в a.txt.
func mergeStopped(t *testing.T) (repo, head string, model tea.Model) {
	t.Helper()

	repo = newRepo(t)
	write(t, repo, "a.txt", "base\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "topic")
	write(t, repo, "a.txt", "topic\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	write(t, repo, "a.txt", "main\n")
	commitAll(t, repo)
	head = runGit(t, repo, "rev-parse", "HEAD")
	merge := exec.Command("git", "merge", "topic")
	merge.Dir = repo
	if out, err := merge.CombinedOutput(); err == nil {
		t.Fatalf("слияние прошло без конфликта:\n%s", out)
	}

	return repo, head, resize(open(t, options(repo, layout.Focus())), 100, 20)
}

// Незавершённое слияние видно в подвале; M, «Abort» и y возвращают HEAD и
// файлы к состоянию до merge (#243, случай 1).
func TestAbortMergeFromMenu(t *testing.T) {
	repo, head, model := mergeStopped(t)

	if footer := footerOf(model); !strings.Contains(footer, "M merging") {
		t.Fatalf("подвал %q, ожидалась операция", footer)
	}

	model, _ = model.Update(key('M'))
	model, _ = model.Update(key('j'))
	model = enterKey(model)
	if footer := footerOf(model); !strings.Contains(footer, "Abort and return to the state before merging? y yes") {
		t.Fatalf("подвал %q, ожидался вопрос", footer)
	}
	model = answerYes(model)

	if got := runGit(t, repo, "rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD %s, ожидался %s", got, head)
	}
	if got := readText(t, repo+"/a.txt"); got != "main\n" {
		t.Fatalf("a.txt = %q после abort", got)
	}
	if footer := footerOf(model); strings.Contains(footer, "merging") {
		t.Fatalf("подвал %q, операция не пропала", footer)
	}
}

// Continue при нерешённом конфликте: отказ git в подвале (случай 3); после
// решения слияние заканчивается коммитом (случай 2 для merge).
func TestContinueMergeFromMenu(t *testing.T) {
	repo, _, model := mergeStopped(t)

	model, _ = model.Update(key('M'))
	model = enterKey(model)
	if footer := footerOf(model); !strings.Contains(footer, "merge: ") {
		t.Fatalf("подвал %q, ожидался отказ git", footer)
	}

	write(t, repo, "a.txt", "merged\n")
	runGit(t, repo, "add", "a.txt")
	model = pressAndSettle(model, key('r'))
	model, _ = model.Update(key('M'))
	model = enterKey(model)

	if parents := runGit(t, repo, "rev-list", "--parents", "-1", "HEAD"); len(strings.Fields(parents)) != 3 {
		t.Fatalf("HEAD не коммит слияния: %q", parents)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Continued") {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}
}

// Без операции M говорит, что продолжать нечего.
func TestOperationMenuWithoutOperation(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	model, _ = model.Update(tea.KeyPressMsg{Code: 'M', Text: "M"})
	// am тоже операция меню (#399)
	if footer := footerOf(model); !strings.Contains(footer, "no merge, rebase, cherry-pick, revert or am is in progress") {
		t.Fatalf("подвал %q, ожидался отказ", footer)
	}
}

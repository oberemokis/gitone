package tui_test

import (
	"strings"
	"testing"

	"gitone/internal/layout"
)

// M на ветке, которая обогнала текущую, перематывает текущую на неё, окно
// закрывается, подвал говорит итог (#229, случай 1).
func TestMergeSelectedBranch(t *testing.T) {
	repo, model := branchRepo(t)
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))

	model = pressAndSettle(model, key('M'))

	if got, want := runGit(t, repo, "rev-parse", "main"), runGit(t, repo, "rev-parse", "unmerged"); got != want {
		t.Fatalf("main %s, ожидалась перемотка на unmerged %s", got, want)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Merged unmerged") {
		t.Fatalf("подвал %q, ожидался итог слияния", footer)
	}
	if body := screenText(model); strings.Contains(body, "Branches") {
		t.Fatalf("окно веток не закрылось:\n%s", body)
	}
}

// r перестраивает текущую ветку поверх выбранной (случай 3), а конфликт
// останавливает операцию со строкой CONFLICT в подвале (случай 5).
func TestRebaseOntoSelectedBranch(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "feature")
	write(t, repo, "f.txt", "feature\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	write(t, repo, "m.txt", "main\n")
	commitAll(t, repo)

	model := openAsync(resize(open(t, options(repo, layout.Focus())), 120, 24), key('B'))
	model, _ = model.Update(key('k'))
	model = pressAndSettle(model, key('r'))

	if got, want := runGit(t, repo, "rev-parse", "HEAD~1"), runGit(t, repo, "rev-parse", "feature"); got != want {
		t.Fatalf("родитель HEAD %s, ожидалась вершина feature %s", got, want)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Rebased onto feature") {
		t.Fatalf("подвал %q, ожидался итог rebase", footer)
	}

	runGit(t, repo, "switch", "-q", "feature")
	write(t, repo, "m.txt", "feature side\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	model = openAsync(model, key('B'))
	model, _ = model.Update(key('k'))
	model = pressAndSettle(model, key('r'))

	if footer := footerOf(model); !strings.Contains(footer, "rebase: rebase stopped on a conflict") {
		t.Fatalf("подвал %q, ожидалась остановка на конфликте", footer)
	}
}

// Слить ветку саму в себя нельзя: подвал просит выбрать другую, окно открыто.
func TestMergeCurrentBranchRefused(t *testing.T) {
	_, model := branchRepo(t)

	model = pressIdle(t, model, key('M'))

	if footer := footerOf(model); !strings.Contains(footer, "this is the current branch") {
		t.Fatalf("подвал %q, ожидался отказ", footer)
	}
	if body := screenText(model); !strings.Contains(body, "Branches") {
		t.Fatalf("окно веток закрылось:\n%s", body)
	}
}

package tui_test

import (
	"strings"
	"testing"

	"gitone/internal/layout"
)

// u спрашивает, какую запись reflog откатить и куда встанет HEAD; y откатывает
// коммит, U после y возвращает его (#239, случаи 1 и 3).
func TestUndoRedoKeys(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	first := runGit(t, repo, "rev-parse", "HEAD")
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)
	second := runGit(t, repo, "rev-parse", "HEAD")
	model := resize(open(t, options(repo, layout.Focus())), 120, 20)

	model = pressAndSettle(model, key('u'))
	if footer := footerOf(model); !strings.Contains(footer, "Undo commit: commit? HEAD goes back to "+first[:7]) {
		t.Fatalf("подвал %q, ожидался вопрос об откате", footer)
	}
	model, _ = model.Update(key('n'))
	if got := runGit(t, repo, "rev-parse", "HEAD"); got != second {
		t.Fatal("откат без y")
	}

	model = answerYes(pressAndSettle(model, key('u')))
	if got := runGit(t, repo, "rev-parse", "HEAD"); got != first {
		t.Fatalf("HEAD %s, ожидался %s", got, first)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Undone, HEAD is at "+first[:7]) {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}

	model = answerYes(pressAndSettle(model, key('U')))
	if got := runGit(t, repo, "rev-parse", "HEAD"); got != second {
		t.Fatalf("HEAD %s после повтора, ожидался %s", got, second)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Redone, HEAD is at "+second[:7]) {
		t.Fatalf("подвал %q, ожидался итог повтора", footer)
	}
}

// Отказ git, например после stash, идёт причиной в подвал (случай 5).
func TestUndoAfterStashRefused(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "dirty\n")
	runGit(t, repo, "stash", "push", "-q")
	model := resize(open(t, options(repo, layout.Focus())), 120, 20)

	model = pressAndSettle(model, key('u'))
	if footer := footerOf(model); !strings.Contains(footer, "the last change is a stash") {
		t.Fatalf("подвал %q, ожидался отказ", footer)
	}
}

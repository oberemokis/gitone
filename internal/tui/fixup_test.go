package tui_test

import (
	"strings"
	"testing"
)

// F на коммите создаёт из индекса «fixup!» к нему, курсор остаётся на нём, S
// сливает fixup в цель (#225, случай 1); без индекса F отказывает (случай 2).
func TestFixupFromLog(t *testing.T) {
	repo, model := rewriteRepo(t)
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))

	model = pressAndSettle(model, key('F'))
	if footer := footerOf(model); !strings.Contains(footer, "nothing to commit") {
		t.Fatalf("подвал %q, ожидался отказ без индекса", footer)
	}

	write(t, repo, "two.txt", "two fixed\n")
	runGit(t, repo, "add", "two.txt")
	model = pressAndSettle(model, key('F'))
	if got := subjectsOf(t, repo); got != "fixup! two four three two one" {
		t.Fatalf("история %q, ожидался fixup-коммит", got)
	}
	if row := markedRow(model); !strings.Contains(row, "  two  ") {
		t.Fatalf("курсор на %q, ожидался тот же коммит two", row)
	}

	pressAndSettle(model, key('S'))
	if got := subjectsOf(t, repo); got != "four three two one" {
		t.Fatalf("история %q, ожидался fixup внутри two", got)
	}
}

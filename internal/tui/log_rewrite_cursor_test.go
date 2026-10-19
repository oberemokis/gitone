package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

var (
	ctrlK = tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}
	ctrlJ = tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}
)

// subjectsRepo это лог из коммитов subjects, от старых к новым, открытый
// окном лога.
func subjectsRepo(t *testing.T, subjects ...string) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	for _, subject := range subjects {
		write(t, repo, subject+".txt", subject+"\n")
		runGit(t, repo, "add", "-A")
		runGit(t, repo, "commit", "-q", "-m", subject)
	}
	return repo, openAsync(resize(open(t, options(repo, layout.Focus())), 140, 30), key('l'))
}

// Под поиском перенос идёт по всей истории, а курсор остаётся на
// перенесённом коммите, а не на строке под поиском (#307, случай 1).
func TestMoveUnderSearchKeepsCommit(t *testing.T) {
	repo, model := subjectsRepo(t, "x-one", "a", "x-two", "b")
	model, _ = model.Update(key('/'))
	model = typeText(model, "x-")
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = model.Update(key('j'))

	model = pressAndSettle(model, ctrlK)

	if got := subjectsOf(t, repo); got != "b x-two x-one a" {
		t.Fatalf("история %q, ожидался x-one над a", got)
	}
	if row := markedRow(model); !strings.Contains(row, "x-one") {
		t.Fatalf("курсор на %q, ожидался перенесённый x-one", row)
	}
}

// F на two и потом S: курсор на two, хоть строкой он сдвигался дважды
// (случай 2).
func TestFixupThenAutosquashKeepsCommit(t *testing.T) {
	repo, model := subjectsRepo(t, "one", "two", "three", "four")
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))
	write(t, repo, "two.txt", "two fixed\n")
	runGit(t, repo, "add", "two.txt")

	model = pressAndSettle(model, key('F'))
	if row := markedRow(model); !strings.Contains(row, " two") || strings.Contains(row, "fixup!") {
		t.Fatalf("после F курсор на %q, ожидался two", row)
	}
	model = pressAndSettle(model, key('S'))
	if got := subjectsOf(t, repo); got != "four three two one" {
		t.Fatalf("история %q после autosquash", got)
	}
	if row := markedRow(model); !strings.Contains(row, " two") {
		t.Fatalf("после S курсор на %q, ожидался two", row)
	}
}

// Reword возвращает в лог и после отправки, и после esc (случай 3).
func TestRewordReturnsToLog(t *testing.T) {
	_, model := subjectsRepo(t, "one", "two")
	model, _ = model.Update(key('j'))

	model = pressAndSettle(model, key('r'))
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if row := markedRow(model); !strings.Contains(row, "one") {
		t.Fatalf("esc не вернул в лог:\n%s", screenText(model))
	}

	model = pressAndSettle(model, key('r'))
	model = enterKey(typeText(eraseText(model, len("one")), "first"))
	model = settle(model, nil, 4)
	if row := markedRow(model); !strings.Contains(row, "first") {
		t.Fatalf("после reword курсор не на переписанном коммите:\n%s", screenText(model))
	}
}

// ctrl+j дважды уводит коммит на две строки вниз, курсор идёт с ним
// (случай 5).
func TestMoveDownTwiceFollowsCommit(t *testing.T) {
	repo, model := subjectsRepo(t, "one", "two", "three", "four")

	model = pressAndSettle(model, ctrlJ)
	model = pressAndSettle(model, ctrlJ)

	if got := subjectsOf(t, repo); got != "three two four one" {
		t.Fatalf("история %q, ожидался four на две строки ниже", got)
	}
	if row := markedRow(model); !strings.Contains(row, "four") {
		t.Fatalf("курсор на %q, ожидался four", row)
	}
}

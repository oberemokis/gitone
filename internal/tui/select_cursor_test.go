package tui_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// lineNames это строки l1…lcount.
func lineNames(count int) []string {
	names := make([]string, count)
	for index := range names {
		names[index] = fmt.Sprintf("l%d", index+1)
	}
	return names
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n") + "\n"
}

// cursorRepo это файл из 40 строк, где в рабочем дереве над l2 вставлено 20
// строк new1…new20, а l15 и l30 правлены: три хунка, первый длиннее
// остальных и с большими новыми номерами.
func cursorRepo(t *testing.T, width, height int) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	base := lineNames(40)
	write(t, repo, "a.txt", joinLines(base))
	commitAll(t, repo)

	changed := slices.Clone(base)
	changed[14], changed[29] = "L15", "L30"
	var inserted []string
	for number := 1; number <= 20; number++ {
		inserted = append(inserted, fmt.Sprintf("new%d", number))
	}
	changed = slices.Insert(changed, 1, inserted...)
	write(t, repo, "a.txt", joinLines(changed))

	return repo, resize(open(t, options(repo, layout.Focus())), width, height)
}

var spaceKey = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}

// После space на среднем хунке выделен хунк ниже, а не хунк выше с большими
// новыми номерами (#304, случай 1).
func TestSelectionMovesDownAfterStage(t *testing.T) {
	_, model := cursorRepo(t, 100, 60)

	model = pressAndSettle(model, key('v'))
	model, _ = model.Update(key('j'))
	model = pressAndSettle(model, spaceKey)

	if marked := markedText(model); !strings.Contains(marked, "L30") || strings.Contains(marked, "new") {
		t.Fatalf("выделен не хунк ниже:\n%s", marked)
	}
}

// Сняты удалённые строки хунка «-a -b -c +A +B +C»: курсор на «+A» (случай 2).
func TestSelectionCursorAfterRemovedLines(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "a\nb\nc\nd\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "A\nB\nC\nd\n")
	model := resize(open(t, options(repo, layout.Focus())), 100, 30)

	model = pressAndSettle(model, key('v'))
	model, _ = model.Update(key('v'))
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))
	model = pressAndSettle(model, spaceKey)

	if got := runGit(t, repo, "show", ":a.txt"); got != "d" {
		t.Fatalf("индекс %q, ожидались снятые a, b, c", got)
	}
	if marked := markedText(model); !strings.Contains(marked, "A") || strings.Contains(marked, "B") {
		t.Fatalf("курсор не на +A:\n%s", marked)
	}
}

// После space отмеченные строки на экране, хоть следующий хунк был ниже
// экрана (случай 3).
func TestSelectionFollowsAfterStage(t *testing.T) {
	_, model := cursorRepo(t, 100, 16)

	model = pressAndSettle(model, key('v'))
	model, _ = model.Update(key('j'))
	model = pressAndSettle(model, spaceKey)

	if marked := markedText(model); !strings.Contains(marked, "l30") {
		t.Fatalf("отмеченных строк нет на экране:\n%s", screenText(model))
	}
}

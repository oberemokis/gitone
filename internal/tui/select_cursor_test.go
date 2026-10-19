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

// Удалённая строка наверху диффа от HEAD, а в индексе правки выше: v
// выделяет хунк, который был на экране, а не хунк выше по старым номерам
// индекса (случай 4).
func TestSelectKeepsTopByNewSide(t *testing.T) {
	repo := newRepo(t)
	base := lineNames(70)
	write(t, repo, "a.txt", joinLines(base))
	commitAll(t, repo)

	var inserted []string
	for number := 1; number <= 10; number++ {
		inserted = append(inserted, fmt.Sprintf("staged%d", number))
	}
	staged := slices.Insert(slices.Clone(base), 1, inserted...)
	write(t, repo, "a.txt", joinLines(staged))
	runGit(t, repo, "add", "a.txt")
	// в staged строка lN стоит под номером N+10: убраны l30 и l40, правлена l60
	changed := slices.Clone(staged)
	changed[69] = "L60"
	changed = slices.Delete(changed, 49, 50)
	changed = slices.Delete(changed, 39, 40)
	write(t, repo, "a.txt", joinLines(changed))
	model := resize(open(t, options(repo, layout.Focus())), 100, 16)

	for range 100 {
		if strings.Contains(screenOf(model)[bodyRow], "- l40") {
			break
		}
		model, _ = model.Update(key('j'))
	}
	if top := screenOf(model)[bodyRow]; !strings.Contains(top, "- l40") {
		t.Fatalf("наверх не вышла удалённая l40:\n%s", screenText(model))
	}

	model = pressAndSettle(model, key('v'))
	if marked := markedText(model); !strings.Contains(marked, "l40") || strings.Contains(marked, "l30") {
		t.Fatalf("выделен не хунк с экрана:\n%s", marked)
	}
}

// k идёт к хунку выше, упор в первый и последний хунк держит курсор, split и
// перенос длинной строки выделяют её целиком (случай 5).
func TestSelectionMovesAndModes(t *testing.T) {
	_, model := cursorRepo(t, 100, 60)

	model = pressAndSettle(model, key('v'))
	model, _ = model.Update(key('k'))
	if marked := markedText(model); !strings.Contains(marked, "new1") {
		t.Fatalf("k у первого хунка увёл курсор:\n%s", marked)
	}
	for range 5 {
		model, _ = model.Update(key('j'))
	}
	if marked := markedText(model); !strings.Contains(marked, "L30") {
		t.Fatalf("j у последнего хунка увёл курсор:\n%s", marked)
	}
	model, _ = model.Update(key('k'))
	if marked := markedText(model); !strings.Contains(marked, "L15") {
		t.Fatalf("k не перевёл на хунк выше:\n%s", marked)
	}

	_, model = cursorRepo(t, 140, 60)
	model = pressAndSettle(model, key('v'))
	model, _ = model.Update(key('j'))
	if marked := markedText(model); !strings.Contains(marked, "l15") || !strings.Contains(marked, "L15") {
		t.Fatalf("split не выделил хунк с обеих сторон:\n%s", marked)
	}

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\n"+strings.Repeat("long ", 40)+"end\n")
	model = resize(open(t, options(repo, layout.Focus())), 80, 30)
	model = pressAndSettle(model, key('v'))
	if marked := markedText(model); !strings.Contains(marked, "end") || strings.Count(marked, "\n") < 1 {
		t.Fatalf("перенесённая строка выделена не целиком:\n%s", marked)
	}
}

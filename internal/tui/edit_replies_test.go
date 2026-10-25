package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

var escPress = tea.KeyPressMsg{Code: tea.KeyEscape}

// submitPending нажимает enter и отдаёт команду правки, не выполняя её: ответ
// git придёт позже, когда тест уже сделает своё.
func submitPending(model tea.Model) (tea.Model, tea.Cmd) {
	return model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// deliver выполняет отложенную команду и доводит её сообщения до конца.
func deliver(model tea.Model, pending tea.Cmd) tea.Model {
	model, cmd := model.Update(pending())
	return settle(model, cmd, 6)
}

// Переименование ветки, esc до ответа: окно веток не открывается снова, а
// подвал перечитан с новым именем (#306, случай 1).
func TestBranchRenameReplyAfterClose(t *testing.T) {
	_, model := branchRepo(t)

	model, _ = model.Update(key('R'))
	model = typeText(eraseText(model, len("main")), "trunk")
	model, pending := submitPending(model)
	model, _ = model.Update(escPress)
	model = deliver(model, pending)

	body := screenText(model)
	if strings.Contains(body, "unmerged") {
		t.Fatalf("окно веток открылось снова:\n%s", body)
	}
	if footer := footerOf(model); !strings.Contains(footer, "trunk") {
		t.Fatalf("подвал %q, ожидалась новая ветка", footer)
	}
}

// Переименование stash, esc до ответа: окно stash не открывается снова
// (случай 2).
func TestStashRenameReplyAfterClose(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "stash", "push", "-q", "-m", "first")
	model := openAsync(resize(open(t, options(repo, layout.Focus())), 120, 24), key('Z'))

	model, _ = model.Update(key('r'))
	model = typeText(model, " renamed")
	model, pending := submitPending(model)
	model, _ = model.Update(escPress)
	model = deliver(model, pending)

	if body := screenText(model); strings.Contains(body, "stash@{0}") {
		t.Fatalf("окно stash открылось снова:\n%s", body)
	}
	if got := runGit(t, repo, "stash", "list", "--format=%gs"); !strings.Contains(got, "first renamed") {
		t.Fatalf("stash %q, ожидалось новое сообщение", got)
	}
}

// Новый тег, esc до ответа: итог в подвале, окно тегов не открывается
// (случай 3).
func TestTagNewReplyAfterClose(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	model := openAsync(resize(open(t, options(repo, layout.Focus())), 120, 24), key('t'))

	model, _ = model.Update(key('n'))
	model = typeText(model, "v9")
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model, pending := submitPending(model)
	model, _ = model.Update(escPress)
	model = deliver(model, pending)

	if body := screenText(model); strings.Contains(body, "Tags") {
		t.Fatalf("окно тегов открылось снова:\n%s", body)
	}
	if footer := footerOf(model); !strings.Contains(footer, "v9") {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}
}

// Удаление ветки, второй вопрос до ответа, n: удалённой ветки в окне нет
// (случай 4).
func TestBranchDeleteReplyUnderQuestion(t *testing.T) {
	_, model := branchRepo(t)
	for range 5 {
		if strings.Contains(markedRow(model), "merged") && !strings.Contains(markedRow(model), "unmerged") {
			break
		}
		model, _ = model.Update(key('j'))
	}

	model, _ = model.Update(key('d'))
	model, pending := model.Update(key('y'))
	model, _ = model.Update(key('d'))
	model = deliver(model, pending)
	model, _ = model.Update(key('n'))

	// окно на месте: пустой экран тоже обошёлся бы без удалённой ветки
	if body := screenText(model); !strings.Contains(body, "Branches") || !strings.Contains(body, "unmerged") {
		t.Fatalf("окно веток пропало после ответа:\n%s", body)
	}
	// тост «Branch merged deleted» строкой списка не считается
	for _, row := range strings.Split(screenText(model), "\n") {
		if strings.Contains(row, "merged") && !strings.Contains(row, "unmerged") && !strings.Contains(row, "deleted") {
			t.Fatalf("удалённая ветка осталась в окне:\n%s", screenText(model))
		}
	}
}

// Squash в логе, вопрос до ответа, n: лог с новой историей (случай 5).
func TestRewriteReplyUnderQuestion(t *testing.T) {
	_, model := rewriteRepo(t)
	model, _ = model.Update(key('j'))

	model, pending := model.Update(key('s'))
	model, _ = model.Update(key('d'))
	model = deliver(model, pending)
	model, _ = model.Update(key('n'))

	body := screenText(model)
	if !strings.Contains(body, "Log") || !strings.Contains(body, "four") || !strings.Contains(body, "two") {
		t.Fatalf("лог пропал после ответа:\n%s", body)
	}
	if strings.Contains(body, "three") {
		t.Fatalf("лог со старой историей после squash:\n%s", body)
	}
}

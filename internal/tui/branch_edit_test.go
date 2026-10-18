package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// branchRepo это репозиторий на main с коммитом и ветками merged (слита) и
// unmerged (свой коммит), открытый окном веток. Ветки по алфавиту: main,
// merged, unmerged, курсор на текущей main.
func branchRepo(t *testing.T) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "branch", "merged")
	runGit(t, repo, "switch", "-q", "-c", "unmerged")
	write(t, repo, "b.txt", "two\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")

	model := resize(open(t, options(repo, layout.Focus())), 120, 24)
	return repo, openAsync(model, key('B'))
}

// enterKey нажимает enter и доводит команды до конца.
func enterKey(model tea.Model) tea.Model {
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return settle(model, cmd, 4)
}

// n спрашивает имя, enter создаёт ветку на выбранной и переключает на неё:
// подвал показывает новую ветку (#227, случай 1).
func TestNewBranchFromBranchWindow(t *testing.T) {
	repo, model := branchRepo(t)

	model, _ = model.Update(key('n'))
	if body := screenText(model); !strings.Contains(body, "New branch from main") {
		t.Fatalf("n не открыл поле имени:\n%s", body)
	}
	model = enterKey(typeText(model, "topic"))

	if got := runGit(t, repo, "branch", "--show-current"); got != "topic" {
		t.Fatalf("текущая %q, ожидалась topic", got)
	}
	if footer := footerOf(model); !strings.Contains(footer, "topic") {
		t.Fatalf("подвал %q, ожидалась новая ветка", footer)
	}
}

// Имя с пробелом git отвергает, подвал показывает причину, ветка прежняя
// (случай 2).
func TestNewBranchBadNameShowsRefusal(t *testing.T) {
	repo, model := branchRepo(t)

	model, _ = model.Update(key('n'))
	model = enterKey(typeText(model, "a b"))

	if footer := footerOf(model); !strings.Contains(footer, "not a valid branch name") {
		t.Fatalf("подвал %q, ожидался отказ git", footer)
	}
	if got := runGit(t, repo, "branch", "--show-current"); got != "main" {
		t.Fatalf("текущая %q, ожидалась main", got)
	}
}

// esc в поле имени возвращает к списку веток, ничего не создаёт.
func TestNewBranchPromptClosesBackToList(t *testing.T) {
	repo, model := branchRepo(t)

	model, _ = model.Update(key('n'))
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if body := screenText(model); !strings.Contains(body, "Branches") || strings.Contains(body, "New branch") {
		t.Fatalf("esc не вернул к списку веток:\n%s", body)
	}
	if got := runGit(t, repo, "branch", "--list", "topic"); got != "" {
		t.Fatalf("ветка создана без enter: %q", got)
	}
}

// d спрашивает в окне, y удаляет слитую ветку, окно остаётся с новым списком
// (случай 3). Любая другая клавиша вопрос снимает и ничего не удаляет.
func TestDeleteMergedBranch(t *testing.T) {
	repo, model := branchRepo(t)
	model, _ = model.Update(key('j'))

	model, _ = model.Update(key('d'))
	if body := screenText(model); !strings.Contains(body, `Delete branch "merged"? y yes`) {
		t.Fatalf("d не задал вопрос в окне:\n%s", body)
	}
	model, _ = model.Update(key('n'))
	if got := runGit(t, repo, "branch", "--list", "merged"); got == "" {
		t.Fatal("ветка удалена без y")
	}

	model, _ = model.Update(key('d'))
	model = answerYes(model)

	if got := runGit(t, repo, "branch", "--list", "merged"); got != "" {
		t.Fatalf("ветка не удалена: %q", got)
	}
	body := screenText(model)
	if !strings.Contains(body, "Branches") || strings.Contains(body, "  merged") || strings.Contains(body, "› merged") {
		t.Fatalf("окно веток не осталось открытым с новым списком:\n%s", body)
	}
}

// Неслитую ветку git не удаляет, подвал показывает его отказ (случай 4).
func TestDeleteUnmergedBranchShowsRefusal(t *testing.T) {
	repo, model := branchRepo(t)
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))

	model, _ = model.Update(key('d'))
	model = answerYes(model)

	if footer := footerOf(model); !strings.Contains(footer, "not fully merged") {
		t.Fatalf("подвал %q, ожидался отказ git", footer)
	}
	if got := runGit(t, repo, "branch", "--list", "unmerged"); got == "" {
		t.Fatal("неслитая ветка удалена")
	}
}

// Текущую ветку удалить нельзя: вопрос не задаётся, подвал называет причину
// (случай 5).
func TestDeleteCurrentBranchRefused(t *testing.T) {
	_, model := branchRepo(t)

	model, _ = model.Update(key('d'))

	if footer := footerOf(model); !strings.Contains(footer, "the current branch cannot be deleted") {
		t.Fatalf("подвал %q, ожидался отказ", footer)
	}
	if body := screenText(model); strings.Contains(body, "Delete branch") {
		t.Fatalf("вопрос задан о текущей ветке:\n%s", body)
	}
}

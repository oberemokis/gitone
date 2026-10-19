package tui_test

import (
	"slices"
	"strings"
	"testing"
)

// Клик мимо окна закрывает его, как в opencode: подложка лежит на весь экран,
// и клик по подписи в подвале тоже идёт ей, а не подписи.
func TestClickBackdropClosesWindow(t *testing.T) {
	model := searchLog(t, "base", "subject-one")
	screen := screenOf(model)
	title := slices.IndexFunc(screen, func(line string) bool { return strings.Contains(line, "Log") })

	for _, point := range [][2]int{{1, title}, {1, footerRow(screen)}} {
		after := screenText(clickAt(model, point[0], point[1]))
		if strings.Contains(after, "Log") || strings.Contains(after, "Branches") {
			t.Fatalf("клик в (%d, %d) мимо лога не закрыл его:\n%s", point[0], point[1], after)
		}
	}
}

// Клик мимо поля ввода возвращает в окно, из которого его открыли.
func TestClickBackdropReturnsFromPrompt(t *testing.T) {
	_, model := branchesWith(t, "feature-x")
	model, _ = model.Update(key('n'))
	if body := screenText(model); !strings.Contains(body, "New branch from") {
		t.Fatalf("n не открыл поле ввода:\n%s", body)
	}

	body := screenText(clickAt(model, 0, 0))
	if strings.Contains(body, "New branch from") || !strings.Contains(body, "Branches") {
		t.Fatalf("клик мимо поля ввода не вернул к веткам:\n%s", body)
	}
}

// Клик мимо окна с вопросом снимает вопрос, окно остаётся, ветка цела.
func TestClickBackdropCancelsWindowQuestion(t *testing.T) {
	repo, model := branchRepo(t)
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('d'))

	body := screenText(clickAt(model, 0, 0))
	if strings.Contains(body, "Delete branch") || !strings.Contains(body, "Branches") {
		t.Fatalf("клик мимо окна не снял вопрос или закрыл окно:\n%s", body)
	}
	if got := runGit(t, repo, "branch", "--list", "merged"); got == "" {
		t.Fatal("ветка удалена кликом мимо окна")
	}
}

// Справка не окно: подложки под ней нет, и клик по диффу её не закрывает.
func TestClickBesideHelpKeepsIt(t *testing.T) {
	_, model := threeFileRepo(t)
	model, _ = model.Update(key('?'))

	if body := screenText(clickAt(model, 0, bodyRow)); !strings.Contains(body, "Hotkeys") {
		t.Fatalf("клик рядом со справкой её закрыл:\n%s", body)
	}
}

package tui_test

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// Под вопросом с главного экрана справка видна, но клики по ней и по диффу
// ничего не делают: вопрос на месте, файл не изменён (#308, случай 1).
func TestClicksRefusedUnderFooterQuestion(t *testing.T) {
	repo, model := stagingRepo(t)
	model, _ = model.Update(key('?'))
	model = pressAndSettle(model, key('v'))
	model, _ = model.Update(key('x'))
	before := readText(t, filepath.Join(repo, "a.txt"))

	for y := range len(screenOf(model)) {
		for _, x := range []int{2, 10, 30, 60, 90} {
			clicked, cmd := model.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
			clicked = settle(clicked, cmd, 2)
			if footer := footerOf(clicked); !strings.Contains(footer, "Discard the selected changes") {
				t.Fatalf("клик %d,%d снял вопрос, подвал %q", x, y, footer)
			}
		}
	}
	if got := readText(t, filepath.Join(repo, "a.txt")); got != before {
		t.Fatalf("a.txt изменился от клика: %q", got)
	}
}

// Ответ h пришёл, пока висит вопрос discard: вопрос виден (случай 2).
func TestLateNoticeKeepsDiscardQuestion(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "new.txt", "new\n")
	model := resize(open(t, options(repo, layout.Focus())), 120, 20)

	model, history := model.Update(key('h'))
	model, _ = model.Update(key('x'))
	model, _ = model.Update(history())

	if footer := footerOf(model); !strings.Contains(footer, "Discard changes") {
		t.Fatalf("подвал %q, вопрос discard не виден", footer)
	}
}

// Опрос прочёл дифф до space, а пришёл после перечитывания: на экране индекс
// после правки, второй space убирает файл из индекса (случай 3).
func TestPollStartedBeforeReloadIsDropped(t *testing.T) {
	repo, model := editedRepo(t, oneTick(), layout.Focus())

	model, polling := model.Update(model.Init()())
	var late []tea.Msg
	for _, cmd := range polling().(tea.BatchMsg) {
		if cmd == nil {
			continue
		}
		if msg := cmd(); msg != nil {
			late = append(late, msg)
		}
	}
	model = pressAndSettle(model, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	for _, msg := range late {
		model, _ = model.Update(msg)
	}
	pressAndSettle(model, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})

	if staged, err := openedRepo(t, repo).StagedPaths(); err != nil || len(staged) != 0 {
		t.Fatalf("в индексе %q, ошибка %v, второй space должен был убрать файл", staged, err)
	}
}

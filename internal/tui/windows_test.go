package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// Поведение окон, которое перестройка обязана сохранить. Каждый тест проходит и
// на коде до неё.

func key(char rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: char, Text: string(char)}
}

func screenText(model tea.Model) string {
	return strings.Join(screenOf(model), "\n")
}

func footerOf(model tea.Model) string {
	screen := screenOf(model)
	return screen[footerRow(screen)]
}

// footerRow это строка подвала: под ним одна пустая строка (#103).
func footerRow(screen []string) int {
	return len(screen) - 2
}

// openAsync нажимает клавишу, которая грузит список ответом git, и доставляет
// ответ, как это сделал бы Bubble Tea.
func openAsync(model tea.Model, press tea.KeyPressMsg) tea.Model {
	model, cmd := model.Update(press)
	model, _ = model.Update(cmd())
	return model
}

func TestHelpStaysUnderDiscardQuestion(t *testing.T) {
	_, model := threeFileRepo(t)

	model, _ = model.Update(key('?'))
	model, _ = model.Update(key('x'))

	if !strings.Contains(screenText(model), "Hotkeys") {
		t.Fatalf("справки нет под вопросом discard:\n%s", screenText(model))
	}
	if !strings.Contains(footerOf(model), "Discard changes") {
		t.Fatalf("подвал = %q, want вопрос discard", footerOf(model))
	}
}

func TestHelpReturnsAfterLogCloses(t *testing.T) {
	_, model := threeFileRepo(t)

	model, _ = model.Update(key('?'))
	model = openAsync(model, key('l'))
	if strings.Contains(screenText(model), "Hotkeys") {
		t.Fatalf("справка поверх лога:\n%s", screenText(model))
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(screenText(model), "Hotkeys") {
		t.Fatalf("справка не вернулась после лога:\n%s", screenText(model))
	}
}

func TestSearchAndCommitCloseHelp(t *testing.T) {
	opens := map[string]tea.KeyPressMsg{
		"поиск":  {Code: 'p', Mod: tea.ModCtrl},
		"коммит": key('c'),
	}

	for name, open := range opens {
		t.Run(name, func(t *testing.T) {
			_, model := threeFileRepo(t)

			model, _ = model.Update(key('?'))
			model, _ = model.Update(open)
			model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

			if strings.Contains(screenText(model), "Hotkeys") {
				t.Fatalf("%s не закрыл справку:\n%s", name, screenText(model))
			}
		})
	}
}

// q, l и Z внутри списков не выходят, не открывают другой список и не
// закрывают текущий.
func TestListsIgnoreGlobalKeys(t *testing.T) {
	lists := map[string]tea.KeyPressMsg{
		"Log":      key('l'),
		"Branches": key('B'),
		"Stash":    key('Z'),
	}

	for title, open := range lists {
		t.Run(title, func(t *testing.T) {
			_, model := threeFileRepo(t)
			model = openAsync(model, open)

			for _, press := range []rune{'q', 'l', 'Z'} {
				var cmd tea.Cmd
				model, cmd = model.Update(key(press))
				if cmd != nil {
					t.Fatalf("%s: %q дал команду, want ничего", title, press)
				}
			}
			if !strings.Contains(screenText(model), title) {
				t.Fatalf("%s закрылся от q, l или Z:\n%s", title, screenText(model))
			}
		})
	}
}

// Сообщение, пришедшее при открытом списке, переживает навигацию по нему и
// его закрытие.
func TestListKeepsFooterMessage(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	// stash без правок отвечает ошибкой, когда список уже открыт
	model, stash := model.Update(key('z'))
	model = openAsync(model, key('l'))
	model = settle(model, stash, 4)

	for _, press := range []tea.KeyPressMsg{key('j'), {Code: tea.KeyEscape}} {
		model, _ = model.Update(press)
		if !strings.Contains(footerOf(model), "no changes to save in stash") {
			t.Fatalf("после %s подвал = %q, want сообщение на месте", press, footerOf(model))
		}
	}
}

// ctrl c в вопросе это ответ «нет», а не выход.
func TestCtrlCAnswersQuestions(t *testing.T) {
	ctrlC := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}

	t.Run("discard", func(t *testing.T) {
		_, model := threeFileRepo(t)
		model, _ = model.Update(key('x'))

		if _, cmd := model.Update(ctrlC); cmd != nil {
			t.Fatal("ctrl c в вопросе discard дал команду, want ответ нет")
		}
	})

	t.Run("удаление stash", func(t *testing.T) {
		repo, _ := threeFileRepo(t)
		runGit(t, repo, "stash", "push", "-q")
		model := resize(open(t, options(repo, layout.Focus())), 100, 20)
		model = openAsync(model, key('Z'))
		model, _ = model.Update(key('d'))

		model, cmd := model.Update(ctrlC)
		if cmd != nil {
			t.Fatal("ctrl c в вопросе удаления stash дал команду, want ответ нет")
		}
		if !strings.Contains(screenText(model), "stash@{0}") {
			t.Fatalf("окно stash закрылось после ответа нет:\n%s", screenText(model))
		}
	})
}

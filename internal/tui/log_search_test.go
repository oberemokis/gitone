package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// commitSubjects делает репозиторий из коммитов с темами subjects, в a.txt
// каждого стоит его тема.
func commitSubjects(t *testing.T, subjects ...string) string {
	t.Helper()

	repo := newRepo(t)
	for _, subject := range subjects {
		write(t, repo, "a.txt", subject+"\n")
		runGit(t, repo, "add", "-A")
		runGit(t, repo, "commit", "-q", "-m", subject)
	}
	return repo
}

// searchLog открывает лог из коммитов с темами subjects (последний наверху).
func searchLog(t *testing.T, subjects ...string) tea.Model {
	t.Helper()

	model := resize(open(t, options(commitSubjects(t, subjects...), layout.Focus())), 120, 30)
	return openAsync(model, key('l'))
}

var (
	enter  = tea.KeyPressMsg{Code: tea.KeyEnter}
	escape = tea.KeyPressMsg{Code: tea.KeyEscape}
)

// / и набор оставляют в таблице только подходящие коммиты, без учёта
// регистра.
func TestLogSearchFilters(t *testing.T) {
	model := searchLog(t, "init", "fix: one", "feat: two", "fix: three")

	model = typeText(model, "/FIX")

	screen := screenText(model)
	if !strings.Contains(screen, "fix: one") || !strings.Contains(screen, "fix: three") {
		t.Fatalf("найденных коммитов нет:\n%s", screen)
	}
	if strings.Contains(screen, "feat: two") || strings.Contains(screen, "init") {
		t.Fatalf("неподходящие коммиты остались:\n%s", screen)
	}
	if !strings.Contains(screen, "/ FIX") {
		t.Fatalf("строки поиска нет:\n%s", screen)
	}
}

// Пока поле открыто, q и j набираются, а не выходят и не двигают курсор.
func TestLogSearchTakesKeysAsText(t *testing.T) {
	model := searchLog(t, "init", "qj marker")

	model, _ = model.Update(key('/'))
	for _, char := range "qj" {
		var cmd tea.Cmd
		model, cmd = model.Update(key(char))
		if cmd != nil {
			if _, quits := cmd().(tea.QuitMsg); quits {
				t.Fatalf("%c в поле поиска вышел из приложения", char)
			}
		}
	}

	if screen := screenText(model); !strings.Contains(screen, "/ qj") || !strings.Contains(screen, "qj marker") {
		t.Fatalf("q и j не набрались в поиск:\n%s", screen)
	}
}

// enter заканчивает набор: j ходит по найденному, enter открывает коммит.
func TestLogSearchEnterKeepsFilter(t *testing.T) {
	model := searchLog(t, "init", "fix: one", "feat: two", "fix: three")

	model = typeText(model, "/fix")
	model, _ = model.Update(enter)
	model, _ = model.Update(key('j'))

	if row := markedRow(model); !strings.Contains(row, "fix: one") {
		t.Fatalf("курсор не на втором найденном: %q", row)
	}

	model, cmd := model.Update(enter)
	model = settle(model, cmd, 3)
	if screen := screenText(model); strings.Contains(screen, "Log") || !strings.Contains(screen, "one") {
		t.Fatalf("enter не открыл найденный коммит:\n%s", screen)
	}
}

// esc в поле снимает фильтр, esc вне поля закрывает окно.
func TestLogSearchEscapeClears(t *testing.T) {
	model := searchLog(t, "init", "fix: one", "feat: two")

	model = typeText(model, "/fix")
	model, _ = model.Update(escape)
	if screen := screenText(model); !strings.Contains(screen, "feat: two") || strings.Contains(screen, "/ fix") {
		t.Fatalf("esc в поле не снял фильтр:\n%s", screen)
	}

	model, _ = model.Update(escape)
	if screen := screenText(model); strings.Contains(screen, "Log") {
		t.Fatalf("esc вне поля не закрыл окно:\n%s", screen)
	}
}

// Ничего не нашлось: надпись вместо таблицы.
func TestLogSearchNoMatches(t *testing.T) {
	model := searchLog(t, "init", "fix: one")

	model = typeText(model, "/zzz")

	if screen := screenText(model); !strings.Contains(screen, "No matching commits") {
		t.Fatalf("нет надписи о пустом поиске:\n%s", screen)
	}
}

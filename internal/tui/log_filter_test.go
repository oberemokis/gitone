package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

var ctrlS = tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}

// filteredRepo это лог из трёх коммитов: ann трогает src, bob трогает src и
// doc.txt; открыт окном лога.
func filteredRepo(t *testing.T) tea.Model {
	t.Helper()

	repo := newRepo(t)
	commitAs := func(author, name, subject string) {
		t.Setenv("GIT_AUTHOR_NAME", author)
		write(t, repo, name, subject+"\n")
		runGit(t, repo, "add", "-A")
		runGit(t, repo, "commit", "-q", "-m", subject)
	}
	commitAs("ann", "src/a.txt", "ann-src")
	commitAs("bob", "src/a.txt", "bob-src")
	commitAs("bob", "doc.txt", "bob-doc")

	return openAsync(resize(open(t, options(repo, layout.Focus())), 120, 30), key('l'))
}

// filterBy выбирает в меню фильтров строку row и набирает значение.
func filterBy(model tea.Model, row int, value string) tea.Model {
	model, _ = model.Update(ctrlS)
	for range row {
		model, _ = model.Update(key('j'))
	}
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return enterKey(typeText(model, value))
}

// Фильтры по пути и автору сужают лог и видны в заголовке, вместе работают
// оба, сброс возвращает полный лог с курсором на том же коммите (#226, случаи
// 1-4).
func TestLogFilters(t *testing.T) {
	model := filterBy(filteredRepo(t), 1, "src")
	body := screenText(model)
	if !strings.Contains(body, "Log · path src") || strings.Contains(body, "bob-doc") || !strings.Contains(body, "ann-src") {
		t.Fatalf("фильтр по пути не сработал:\n%s", body)
	}

	model = filterBy(model, 0, "bob")
	body = screenText(model)
	if !strings.Contains(body, "author bob · path src") || strings.Contains(body, "ann-src") || !strings.Contains(body, "bob-src") {
		t.Fatalf("два фильтра не сработали вместе:\n%s", body)
	}

	model, _ = model.Update(ctrlS)
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))
	model = enterKey(model)
	if body := screenText(model); !strings.Contains(body, "bob-doc") || strings.Contains(body, "Log ·") {
		t.Fatalf("сброс не вернул полный лог:\n%s", body)
	}
	if row := markedRow(model); !strings.Contains(row, "bob-src") {
		t.Fatalf("курсор на %q, ожидался тот же коммит bob-src", row)
	}
}

// Фильтр автора git ищет по всей истории долго, сброс приходит раньше: поздний
// ответ фильтра в лог не ложится (#380, случай 3).
func TestLateFilterReplyAfterClear(t *testing.T) {
	model, _ := filteredRepo(t).Update(ctrlS)
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model, slow := typeText(model, "zzz").Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	model, _ = model.Update(ctrlS)
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))
	model = settle(enterKey(model), slow, 2)
	if body := screenText(model); strings.Contains(body, "author zzz") || !strings.Contains(body, "bob-doc") {
		t.Fatalf("поздний ответ фильтра лёг после сброса:\n%s", body)
	}
}

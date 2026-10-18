package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// stagingRepo это файл из десяти строк с правкой первой и девятой: два хунка.
// Экран в 100 колонок показывает unified, строки идут одна под другой.
func stagingRepo(t *testing.T) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	lines := []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}
	write(t, repo, "a.txt", strings.Join(lines, "\n")+"\n")
	commitAll(t, repo)
	lines[0], lines[8] = "ONE", "NINE"
	write(t, repo, "a.txt", strings.Join(lines, "\n")+"\n")

	return repo, resize(open(t, options(repo, layout.Focus())), 100, 30)
}

// v выделяет первый хунк, j переводит курсор на второй, space ставит в индекс
// только его (#212, случай 1). Поставленный хунк пропадает с экрана
// выделения, а файл в панели помечен как поставленный частично (случай 3).
func TestStageHunk(t *testing.T) {
	repo, model := stagingRepo(t)

	model = pressAndSettle(model, key('v'))
	if marks := strings.Count(screenText(model), "▌"); marks != 2 {
		t.Fatalf("выделено %d строк, ожидался первый хунк из двух:\n%s", marks, screenText(model))
	}
	model, _ = model.Update(key('j'))
	model = pressAndSettle(model, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})

	cached := runGit(t, repo, "diff", "--cached")
	if !strings.Contains(cached, "+NINE") || strings.Contains(cached, "ONE") {
		t.Fatalf("в индексе не только второй хунк:\n%s", cached)
	}
	body := screenText(model)
	if strings.Contains(body, "NINE") || !strings.Contains(body, "ONE") {
		t.Fatalf("на экране выделения остался поставленный хунк:\n%s", body)
	}

	model = toggleFiles(model)
	// панель узкая, конец подписи обрезан
	if body := screenText(model); !strings.Contains(body, "a.txt partly") {
		t.Fatalf("панель не пометила частичный индекс:\n%s", body)
	}
}

// Второе v переключает на строки: j расширяет выделение, space ставит только
// выбранные строки (случай 2). esc возвращает дифф от HEAD.
func TestStageLines(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\nfirst\nsecond\nthird\n")
	model := resize(open(t, options(repo, layout.Focus())), 100, 30)

	model = pressAndSettle(model, key('v'))
	model, _ = model.Update(key('v'))
	model, _ = model.Update(key('j'))
	model = pressAndSettle(model, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})

	if got := runGit(t, repo, "show", ":a.txt"); got != "one\nfirst\nsecond" {
		t.Fatalf("индекс %q, ожидались две первые строки", got)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if body := screenText(model); !strings.Contains(body, "first") || strings.Contains(body, "▌") {
		t.Fatalf("esc не вернул дифф от HEAD:\n%s", body)
	}
}

// Неотслеженный файл хунков не даёт: подвал советует space (случай 4).
func TestSelectUntrackedFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "new.txt", "new\n")
	model := resize(open(t, options(repo, layout.Focus())), 100, 30)

	model = pressAndSettle(model, key('v'))
	if footer := footerOf(model); !strings.Contains(footer, "space stages this file whole") {
		t.Fatalf("подвал %q, ожидался совет", footer)
	}
}

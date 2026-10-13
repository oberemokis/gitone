package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// paste это вставка текста из буфера: Bubble Tea v2 присылает её одним
// сообщением, а не нажатиями.
func paste(model tea.Model, content string) (tea.Model, tea.Cmd) {
	return model.Update(tea.PasteMsg{Content: content})
}

// Вставка в поле темы коммита попадает в поле (#153).
func TestPasteIntoCommitSubject(t *testing.T) {
	_, model := threeFileRepo(t)

	model, _ = model.Update(key('c'))
	model, _ = paste(model, "pasted subject")

	if screen := screenText(model); !strings.Contains(screen, "> pasted subject_") {
		t.Fatalf("вставка не попала в поле темы:\n%s", screen)
	}
}

// Вставка в поиск файлов заполняет запрос и сужает список.
func TestPasteIntoFileSearch(t *testing.T) {
	_, model := threeFileRepo(t)

	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	model, _ = paste(model, "b.t")

	screen := screenText(model)
	if !strings.Contains(screen, "/ b.t") || !strings.Contains(screen, "› b.txt") || strings.Contains(screen, "c.txt") {
		t.Fatalf("вставка не заполнила запрос поиска файлов:\n%s", screen)
	}
}

// Вставка в поиск списка заполняет запрос в логе и в окне веток.
func TestPasteIntoListSearch(t *testing.T) {
	for _, tc := range []struct {
		name  string
		open  func(t *testing.T) tea.Model
		match string
	}{
		{name: "log", open: func(t *testing.T) tea.Model { return searchLog(t, "init", "fix: one", "feat: two") }, match: "fix: one"},
		{name: "branches", open: func(t *testing.T) tea.Model { _, model := branchesWith(t, "fix-one", "feat-two"); return model }, match: "fix-one"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model, _ := tc.open(t).Update(key('/'))
			model, _ = paste(model, "fix")

			screen := screenText(model)
			if !strings.Contains(screen, "/ fix") || !strings.Contains(screen, tc.match) || strings.Contains(screen, "two") {
				t.Fatalf("вставка не заполнила запрос поиска:\n%s", screen)
			}
		})
	}
}

// Без открытого поля вставка игнорируется: ни экран, ни коммит не меняются,
// «q» из вставки не выходит из приложения.
func TestPasteWithoutFieldIsIgnored(t *testing.T) {
	_, model := threeFileRepo(t)
	before := screenText(model)

	model, cmd := paste(model, "q\nc")

	if cmd != nil {
		t.Fatal("вставка без поля запустила команду")
	}
	if after := screenText(model); after != before {
		t.Fatalf("вставка без поля изменила экран:\n%s", after)
	}
}

// Окно списка с закрытым полем поиска вставку не берёт: и пока поле не
// открыто, и когда фильтр уже зафиксирован по enter.
func TestPasteIntoListWithoutSearchField(t *testing.T) {
	model := searchLog(t, "init", "fix: one", "feat: two")

	model, _ = paste(model, "fix")
	if screen := screenText(model); strings.Contains(screen, "/ fix") || !strings.Contains(screen, "feat: two") {
		t.Fatalf("вставка открыла поиск при закрытом поле:\n%s", screen)
	}

	model = typeText(model, "/fix")
	model, _ = model.Update(enter)
	model, _ = paste(model, "zzz")
	if screen := screenText(model); !strings.Contains(screen, "/ fix") || strings.Contains(screen, "zzz") || !strings.Contains(screen, "fix: one") {
		t.Fatalf("вставка изменила зафиксированный фильтр:\n%s", screen)
	}
}

// В поле уходит первая строка вставки: перевод строки сработал бы как enter и
// отправил коммит.
func TestPasteWithNewlineKeepsFirstLine(t *testing.T) {
	repo, model := threeFileRepo(t)
	before := runGit(t, repo, "rev-parse", "HEAD")

	model, _ = model.Update(key('c'))
	model, cmd := paste(model, "first line\nsecond line\n")

	screen := screenText(model)
	if !strings.Contains(screen, "> first line_") || strings.Contains(screen, "second") {
		t.Fatalf("в поле не первая строка вставки:\n%s", screen)
	}
	if cmd != nil {
		t.Fatal("вставка с переводом строки запустила команду")
	}
	if after := runGit(t, repo, "rev-parse", "HEAD"); after != before {
		t.Fatal("вставка с переводом строки отправила коммит")
	}
}

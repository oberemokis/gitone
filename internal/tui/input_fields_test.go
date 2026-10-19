package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
)

var ctrlP = tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}

// Описание из эмодзи с вариантом представления переносится по ширине
// терминала: строка поля не шире строки из латиницы (#312, случай 1).
func TestDescriptionWrapsWideGraphemes(t *testing.T) {
	typed := func(text string) []string {
		_, model := threeFileRepo(t)
		model = resize(model, 40, 30)
		model, _ = model.Update(key('c'))
		model, _ = model.Update(tab)
		model, _ = paste(model, text)
		return fieldRows(t, model)
	}

	limit := ansi.StringWidth(typed(strings.Repeat("x", 100))[0])
	for _, row := range typed(strings.Repeat("⚠️", 40)) {
		if width := ansi.StringWidth(row); width > limit {
			t.Fatalf("строка поля шириной %d, шире строки латиницы %d: %q", width, limit, row)
		}
	}
}

// Курсор и стирание не рвут кластер: ← после «⚠️» встаёт перед ним целиком,
// backspace стирает его целиком (случай 1).
func TestDescriptionCursorKeepsGraphemes(t *testing.T) {
	_, model := threeFileRepo(t)
	model, _ = model.Update(key('c'))
	model, _ = model.Update(tab)
	model, _ = paste(model, "a⚠️b")

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if rows := fieldRows(t, model); rows[0] != "ab" {
		t.Fatalf("поле %q, ожидалось ab: backspace стёр кластер не целиком", rows[0])
	}
}

// Поиск файлов находит путь в NFD по запросу в NFC и наоборот (случай 2).
func TestPickerMatchesAcrossNormalization(t *testing.T) {
	repo := newRepo(t)
	runGit(t, repo, "config", "core.precomposeUnicode", "false")
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "café.go", "new\n")
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	for _, query := range []string{"é", "é"} {
		picked, _ := model.Update(ctrlP)
		picked = typeText(picked, query)
		if row := markedRow(picked); !strings.Contains(row, ".go") {
			t.Fatalf("запрос %q не нашёл café.go:\n%s", query, screenText(picked))
		}
	}
}

// Вставка в однострочное поле: строка до первого \r или \n, таб пробелами,
// управляющие символы отброшены (случаи 3 и 4).
func TestPasteIntoLineFieldsIsCleaned(t *testing.T) {
	_, model := threeFileRepo(t)

	subject, _ := model.Update(key('c'))
	subject, _ = paste(subject, "a\tb\x1b[31mc")
	if body := screenText(subject); !strings.Contains(body, "> a    b[31mc_") {
		t.Fatalf("тема не очищена:\n%q", body)
	}

	search, _ := model.Update(ctrlP)
	search, _ = paste(search, "a.txt\rb.txt")
	if body := screenText(search); !strings.Contains(body, "/ a.txt") || strings.Contains(body, "/ a.txtb") {
		t.Fatalf("поиск взял не первую строку:\n%s", body)
	}
}

// Поиск файлов: регистр не важен, backspace стирает руну целиком, а набор
// после down возвращает выделение на первое найденное (случай 5).
func TestPickerQueryEditing(t *testing.T) {
	_, model := threeFileRepo(t)

	model, _ = model.Update(ctrlP)
	model = typeText(model, "B")
	if row := markedRow(model); !strings.Contains(row, "b.txt") {
		t.Fatalf("поиск B не нашёл b.txt, отмечено %q", row)
	}
	model = typeText(model, "é")
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if body := screenText(model); !strings.Contains(body, "/ B") || strings.Contains(body, "/ B�") {
		t.Fatalf("backspace стёр не руну:\n%s", body)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	model = typeText(model, ".")
	if row := markedRow(model); !strings.Contains(row, "a.txt") {
		t.Fatalf("набор после down не вернул выделение на первое, отмечено %q", row)
	}
}

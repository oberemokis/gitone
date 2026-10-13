package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/keymap"
	"gitone/internal/layout"
	"gitone/internal/tui"
)

// loadKeys читает клавиши из keybinds.json с заданным содержимым.
func loadKeys(t *testing.T, content string) keymap.Keymap {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keybinds.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	km, err := keymap.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return km
}

// openWithKeys открывает экран без диффа с заданными клавишами.
func openWithKeys(km keymap.Keymap) tea.Model {
	opts := options("", layout.Focus())
	opts.Keys = km
	return resize(tui.New(opts, nil), 100, 20)
}

// Переназначенный выход работает, встроенный перестаёт, а подвал показывает
// новую клавишу.
func TestRebindingQuitChangesKeyAndFooter(t *testing.T) {
	model := openWithKeys(loadKeys(t, `{"keybinds":{"app.quit":"e"}}`))

	if footer := footerOf(model); !strings.Contains(footer, "e quit") || strings.Contains(footer, "q quit") {
		t.Fatalf("в подвале %q, ожидалось «e quit» без «q quit»", footer)
	}

	if _, cmd := model.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); cmd != nil {
		if _, isQuit := cmd().(tea.QuitMsg); isQuit {
			t.Fatal("q выходит, хотя выход переназначен на e")
		}
	}

	_, cmd := model.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if cmd == nil {
		t.Fatal("x не выходит, хотя назначен на выход")
	}
	if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Fatal("команда по x не выходит из приложения")
	}
}

// Клавиша «+» остаётся «+» в подвале и в строке справки: пробелом «+» заменяется
// только как разделитель модификаторов, «ctrl++» читается «ctrl +».
func TestPlusKeyIsShown(t *testing.T) {
	tests := []struct {
		name, keybinds string
		press          tea.KeyPressMsg
		wantFooter     string
		wantHelpRow    string
	}{
		{name: "плюс", keybinds: `{"keybinds":{"app.help":"+"}}`, press: key('+'), wantFooter: "+ help", wantHelpRow: " +"},
		{name: "ctrl плюс", keybinds: `{"keybinds":{"app.help":"ctrl++"}}`, press: tea.KeyPressMsg{Code: '+', Mod: tea.ModCtrl}, wantFooter: "ctrl + help", wantHelpRow: "ctrl +"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model := openWithKeys(loadKeys(t, tc.keybinds))
			if footer := footerOf(model); !strings.Contains(footer, tc.wantFooter) {
				t.Fatalf("в подвале %q, want %q", footer, tc.wantFooter)
			}

			model, _ = model.Update(tc.press)
			row := rowWith(t, screenOf(model), "Toggle this help")
			if !strings.HasSuffix(strings.TrimSpace(row), tc.wantHelpRow) {
				t.Fatalf("строка справки %q, want окончание %q", row, tc.wantHelpRow)
			}
		})
	}
}

// В вопросе discard клавиша ответа «+» тоже видна.
func TestQuestionShowsPlusKey(t *testing.T) {
	repo, _ := threeFileRepo(t)
	model := openWithKeybinds(t, repo, `{"keybinds":{"question.yes":"+"}}`, 100)
	model, _ = model.Update(key('x'))

	if footer := footerOf(model); !strings.Contains(footer, "? + yes") {
		t.Fatalf("в вопросе %q, want «? + yes»", footer)
	}
}

// Отключённая справка не открывается, а отключённые клавиши закрытия не
// роняют отрисовку: справка без клавиши закрытия остаётся работоспособной.
func TestDisabledActionsDoNotBreakScreen(t *testing.T) {
	model := openWithKeys(loadKeys(t, `{"keybinds":{"app.help":"none"}}`))
	model, _ = model.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if strings.Contains(strings.Join(screenOf(model), "\n"), "Hotkeys") {
		t.Fatal("справка открылась, хотя ? отключён")
	}

	model = openWithKeys(loadKeys(t, `{"keybinds":{"help.close":"none"}}`))
	model, _ = model.Update(tea.KeyPressMsg{Code: '?', Text: "?"})

	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "Hotkeys") {
		t.Fatalf("справка не открылась без клавиши закрытия:\n%s", body)
	}
}

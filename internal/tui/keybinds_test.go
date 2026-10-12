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

	if footer := screenOf(model)[len(screenOf(model))-1]; !strings.Contains(footer, "e quit") || strings.Contains(footer, "q quit") {
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

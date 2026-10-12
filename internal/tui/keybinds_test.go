package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/config"
	"gitone/internal/keymap"
	"gitone/internal/tui"
	"gitone/internal/tui/theme"
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

// Переназначенный выход работает, встроенный перестаёт, а подвал показывает
// новую клавишу.
func TestRebindingQuitChangesKeyAndFooter(t *testing.T) {
	km := loadKeys(t, `{"keybinds":{"app.quit":"w"}}`)
	var model tea.Model = tui.New(tui.Options{Config: config.Default(), Keys: km, Theme: theme.Dark()}, nil)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	if footer := screenOf(model)[len(screenOf(model))-1]; !strings.Contains(footer, "w quit") || strings.Contains(footer, "q quit") {
		t.Fatalf("в подвале %q, ожидалось «w quit» без «q quit»", footer)
	}

	if _, cmd := model.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); cmd != nil {
		if _, isQuit := cmd().(tea.QuitMsg); isQuit {
			t.Fatal("q выходит, хотя выход переназначен на w")
		}
	}

	_, cmd := model.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
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
	km := loadKeys(t, `{"keybinds":{"app.help":"none"}}`)
	var model tea.Model = tui.New(tui.Options{Config: config.Default(), Keys: km, Theme: theme.Dark()}, nil)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	model, _ = model.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if strings.Contains(strings.Join(screenOf(model), "\n"), "Hotkeys") {
		t.Fatal("справка открылась, хотя ? отключён")
	}

	km = loadKeys(t, `{"keybinds":{"help.close":"none"}}`)
	model = tui.New(tui.Options{Config: config.Default(), Keys: km, Theme: theme.Dark()}, nil)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	model, _ = model.Update(tea.KeyPressMsg{Code: '?', Text: "?"})

	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "Hotkeys") {
		t.Fatalf("справка не открылась без клавиши закрытия:\n%s", body)
	}
}

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/tui"
)

func TestGreetingIsCentered(t *testing.T) {
	model, _ := tui.New().Update(tea.WindowSizeMsg{Width: 21, Height: 5})
	lines := strings.Split(model.View().Content, "\n")

	if len(lines) != 5 {
		t.Fatalf("строк %d, ожидалось 5", len(lines))
	}
	if want := "     hello world     "; lines[2] != want {
		t.Fatalf("средняя строка %q, ожидалось %q", lines[2], want)
	}
}

func TestQuitKeys(t *testing.T) {
	keys := []tea.KeyPressMsg{
		{Code: 'q', Text: "q"},
		{Code: 'c', Mod: tea.ModCtrl},
	}

	for _, key := range keys {
		_, cmd := tui.New().Update(key)
		if cmd == nil {
			t.Fatalf("%s: команды нет, ожидался выход", key)
		}
		if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
			t.Fatalf("%s: команда не выходит из приложения", key)
		}
	}
}

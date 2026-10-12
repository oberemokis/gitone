package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/tui"
)

func TestScreenLayout(t *testing.T) {
	model, _ := tui.New().Update(tea.WindowSizeMsg{Width: 60, Height: 10})
	lines := strings.Split(ansi.Strip(model.View().Content), "\n")

	if len(lines) != 10 {
		t.Fatalf("строк %d, ожидалось 10", len(lines))
	}

	hints := "q quit   ctrl p files   ? help"
	margin := strings.Repeat(" ", (60-len(hints))/2)
	if footer := lines[len(lines)-1]; footer != margin+hints+margin {
		t.Errorf("подвал не по центру: %q", footer)
	}

	// тело идёт со строки 0 по 8, его середина строка 4
	if got := strings.TrimSpace(lines[4]); got != "hello world" {
		t.Errorf("в середине тела %q, ожидалось hello world", got)
	}
}

func TestHelpToggle(t *testing.T) {
	var model tea.Model = tui.New()
	model, _ = model.Update(tea.WindowSizeMsg{Width: 60, Height: 20})

	isHelpShown := func() bool {
		return strings.Contains(ansi.Strip(model.View().Content), "Hotkeys")
	}

	steps := []struct {
		key       tea.KeyPressMsg
		wantShown bool
	}{
		{key: tea.KeyPressMsg{Code: '?', Text: "?"}, wantShown: true},
		{key: tea.KeyPressMsg{Code: '?', Text: "?"}, wantShown: false},
		{key: tea.KeyPressMsg{Code: '?', Text: "?"}, wantShown: true},
		{key: tea.KeyPressMsg{Code: tea.KeyEscape}, wantShown: false},
	}

	for _, step := range steps {
		model, _ = model.Update(step.key)
		if isHelpShown() != step.wantShown {
			t.Fatalf("после %s справка видна: %v, ожидалось %v", step.key, isHelpShown(), step.wantShown)
		}
	}
}

func TestHelpGroupsByCategory(t *testing.T) {
	var model tea.Model = tui.New()
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	model, _ = model.Update(tea.KeyPressMsg{Code: '?', Text: "?"})

	lines := strings.Split(ansi.Strip(model.View().Content), "\n")
	lineOf := func(prefix string) int {
		for index, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), prefix) {
				return index
			}
		}
		return -1
	}

	// «Toggle this help» стоит в списке после «File list», но относится к
	// General, поэтому в справке идёт раньше секции Files
	want := []string{"General", "Quit", "Toggle this help", "Files", "File list"}
	previous := -1
	for _, prefix := range want {
		index := lineOf(prefix)
		if index <= previous {
			t.Fatalf("строка %q не на своём месте, порядок должен быть %v", prefix, want)
		}
		previous = index
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

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
	"gitone/internal/tui"
)

func TestWithoutChangesBodySaysNoChanges(t *testing.T) {
	lines := render(tui.New(options("", layout.Focus()), nil), 60, 10)

	if len(lines) != 10 {
		t.Fatalf("строк %d, ожидалось 10", len(lines))
	}

	hints := "q quit   ctrl p files   ? help"
	margin := strings.Repeat(" ", (60-len(hints))/2)
	if footer := lines[len(lines)-1]; footer != margin+hints+margin {
		t.Errorf("подвал не по центру: %q", footer)
	}

	// заголовок и пустая строка занимают строки 0 и 1, тело идёт со 2 по 8,
	// его середина строка 5
	if got := strings.TrimSpace(lines[5]); got != "No changes" {
		t.Errorf("в середине тела %q, ожидалось No changes", got)
	}
}

func TestHelpToggle(t *testing.T) {
	model := resize(tui.New(options("", layout.Focus()), nil), 60, 20)

	isHelpShown := func() bool {
		return strings.Contains(strings.Join(screenOf(model), "\n"), "Hotkeys")
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
	model := resize(tui.New(options("", layout.Focus()), nil), 80, 30)
	model, _ = model.Update(tea.KeyPressMsg{Code: '?', Text: "?"})

	lines := screenOf(model)
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
		_, cmd := tui.New(options("", layout.Focus()), nil).Update(key)
		if cmd == nil {
			t.Fatalf("%s: команды нет, ожидался выход", key)
		}
		if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
			t.Fatalf("%s: команда не выходит из приложения", key)
		}
	}
}

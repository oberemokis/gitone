package tui_test

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

// escKey это клавиша esc как слово: «Description» в окне коммита содержит «esc»
// внутри слова.
var escKey = regexp.MustCompile(`\besc\b`)

// windowOpeners открывают каждое окно поверх диффа. Заголовок окна по тексту.
var windowOpeners = []struct {
	title string
	open  func(tea.Model) tea.Model
}{
	{title: "Log", open: func(m tea.Model) tea.Model { return openAsync(m, key('l')) }},
	{title: "Branches", open: func(m tea.Model) tea.Model { return openAsync(m, key('B')) }},
	{title: "Worktrees", open: func(m tea.Model) tea.Model { return openAsync(m, key('W')) }},
	{title: "Stash", open: func(m tea.Model) tea.Model { return openAsync(m, key('Z')) }},
	{title: "Commit", open: func(m tea.Model) tea.Model { m, _ = m.Update(key('c')); return m }},
	{title: "Files", open: func(m tea.Model) tea.Model {
		m, _ = m.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
		return m
	}},
	{title: "Hotkeys", open: func(m tea.Model) tea.Model { m, _ = m.Update(key('?')); return m }},
}

// titledWindow открывает три изменённых файла с приглушённым цветом #030303
// и accent #0a0a0a, чтобы цвета заголовка читались в выводе.
func titledWindow(t *testing.T) tea.Model {
	t.Helper()

	repo, _ := threeFileRepo(t)
	custom := theme.Dark()
	custom.TextMuted = lipgloss.Color("#030303")
	custom.Accent = lipgloss.Color("#0a0a0a")
	opts := options(repo, layout.Focus())
	opts.Theme = custom
	return resize(open(t, opts), 120, 40)
}

// У всех окон заголовок как у справки: жирный без accent слева, «esc close»
// справа, под ним пустая строка. Закрытия в нижней подсказке нет.
func TestWindowTitlesLookAlike(t *testing.T) {
	for _, window := range windowOpeners {
		t.Run(window.title, func(t *testing.T) {
			model := window.open(titledWindow(t))
			screen := screenOf(model)

			row := -1
			for index, line := range screen {
				if strings.Contains(line, " "+window.title+" ") {
					row = index
					break
				}
			}
			if row < 0 {
				t.Fatalf("окна %q нет на экране:\n%s", window.title, strings.Join(screen, "\n"))
			}

			title := strings.TrimSpace(screen[row])
			if !strings.HasPrefix(title, window.title) || !strings.HasSuffix(title, "esc close") {
				t.Errorf("заголовок %q, want «%s … esc close»", title, window.title)
			}
			// под заголовком пусто в тех же колонках: строка окна, а не
			// продолжение диффа по краям
			at := strings.Index(screen[row], window.title)
			if below := screen[row+1]; strings.TrimSpace(below[at:at+len(window.title)]) != "" {
				t.Errorf("под заголовком не пустая строка: %q", below)
			}
			for _, line := range screen[row+1:] {
				if escKey.MatchString(line) {
					t.Errorf("esc остался ниже заголовка: %q", line)
				}
			}

			// в сыром выводе строка заголовка единственная с «close»
			raw := model.View().Content
			if code := styleBefore(t, raw, " close", window.title); strings.Contains(code, "38;2;10;10;10") || !isBold(code) {
				t.Errorf("заголовок цветом accent или не жирный: %q", code)
			}
			if code := styleBefore(t, raw, " close", "esc"); strings.Contains(code, mutedFg) {
				t.Errorf("esc приглушён: %q", code)
			}
			if code := styleBefore(t, raw, " close", " close"); !strings.Contains(code, mutedFg) {
				t.Errorf("close не приглушён: %q", code)
			}
		})
	}
}

// Переназначенное закрытие видно в заголовке.
func TestRemappedCloseInTitle(t *testing.T) {
	repo, _ := threeFileRepo(t)
	model := openAsync(openWithKeybinds(t, repo, `{"keybinds":{"list.close":"backspace"}}`, 120), key('l'))

	if screen := screenText(model); !strings.Contains(screen, "backspace close") {
		t.Fatalf("в заголовке лога нет переназначенного закрытия:\n%s", screen)
	}
}

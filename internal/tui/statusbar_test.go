package tui_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

// Подсказки прижаты к правому краю, ctrl p в подвале нет: окно поиска
// открывается по-прежнему, а клавишу видно в справке.
func TestFooterHintsOnTheRight(t *testing.T) {
	_, model := threeFileRepo(t)

	footer := footerOf(model)
	if !strings.HasSuffix(footer, "q quit   ? help ") {
		t.Fatalf("подвал %q, want подсказки у правого края", footer)
	}
	if strings.Contains(footer, "ctrl p") {
		t.Fatalf("подвал %q, want без ctrl p", footer)
	}
}

// Слева ветка и имя каталога worktree, каждое со своей клавишей, как
// подсказки справа (#108).
func TestFooterShowsBranchAndWorktree(t *testing.T) {
	repo, model := threeFileRepo(t)

	want := " B main   W " + filepath.Base(repo)
	if footer := footerOf(model); !strings.HasPrefix(footer, want) {
		t.Fatalf("подвал %q, want начало %q", footer, want)
	}
}

// Клавиши из keymap: переназначенная видна, отключённая не пишется.
func TestFooterLocationKeysFollowKeymap(t *testing.T) {
	repo, _ := threeFileRepo(t)
	model := openWithKeybinds(t, repo, `{"keybinds":{"app.branches":"o","app.worktrees":"none"}}`, 100)

	want := " o main   " + filepath.Base(repo)
	if footer := footerOf(model); !strings.HasPrefix(footer, want) {
		t.Fatalf("подвал %q, want начало %q", footer, want)
	}
}

var (
	// accent и worktree встроенной темы в ANSI
	accentFg   = colorCode(38, theme.Dark().Accent)
	worktreeFg = colorCode(38, theme.Dark().Worktree)
)

// Слева ветка цветом accent, worktree цветом worktree, а клавиши B и W
// остаются цветом текста, как подсказки справа.
func TestFooterLocationColors(t *testing.T) {
	repo, model := threeFileRepo(t)
	raw := model.View().Content

	// в сыром выводе «q» и « quit» разделены кодами цвета
	if code := styleBefore(t, raw, " quit", "main"); !strings.Contains(code, accentFg) {
		t.Errorf("ветка не цветом accent: %q", code)
	}
	if code := styleBefore(t, raw, " quit", filepath.Base(repo)); !strings.Contains(code, worktreeFg) {
		t.Errorf("worktree не цветом worktree: %q", code)
	}
	for _, name := range []string{"B", "W"} {
		code := styleBefore(t, raw, " quit", name)
		if strings.Contains(code, accentFg) || strings.Contains(code, worktreeFg) || strings.Contains(code, mutedFg) {
			t.Errorf("клавиша %s не цветом текста: %q", name, code)
		}
	}
}

// isBold говорит, включает ли код стиля жирное начертание, параметр 1. Цвета
// встроенной темы не содержат компоненты 1, поэтому с цветом он не спутается.
func isBold(code string) bool {
	params := strings.Split(strings.TrimSuffix(strings.TrimPrefix(code, "\x1b["), "m"), ";")
	return slices.Contains(params, "1")
}

// Имена ветки и worktree в подвале жирные, чтобы не сливаться с подсказками, а
// клавиши B и W остаются обычными. Цвета имён прежние.
func TestFooterNamesAreBold(t *testing.T) {
	repo, model := threeFileRepo(t)
	raw := model.View().Content

	names := []struct{ name, color string }{{"main", accentFg}, {filepath.Base(repo), worktreeFg}}
	for _, tc := range names {
		code := styleBefore(t, raw, " quit", tc.name)
		if !isBold(code) {
			t.Errorf("имя %q не жирное: %q", tc.name, code)
		}
		if !strings.Contains(code, tc.color) {
			t.Errorf("имя %q потеряло цвет %s: %q", tc.name, tc.color, code)
		}
	}
	for _, key := range []string{"B", "W"} {
		if code := styleBefore(t, raw, " quit", key); isBold(code) {
			t.Errorf("клавиша %s жирная: %q", key, code)
		}
	}
}

// Цвет worktree в подвале берётся из роли темы, а не зашит в код.
func TestFooterWorktreeFollowsTheme(t *testing.T) {
	repo, _ := threeFileRepo(t)
	custom := theme.Dark()
	custom.Worktree = lipgloss.Color("#ff0000")
	opts := options(repo, layout.Focus())
	opts.Theme = custom
	raw := resize(open(t, opts), 100, 20).View().Content

	if code := styleBefore(t, raw, " quit", filepath.Base(repo)); !strings.Contains(code, "38;2;255;0;0") {
		t.Errorf("worktree в подвале не цветом темы: %q", code)
	}
}

// Перечитывание по r: по центру спиннер и loading, края подвала на месте.
func TestReloadSpinnerKeepsEdges(t *testing.T) {
	repo, model := threeFileRepo(t)

	model, _ = model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})

	footer := footerOf(model)
	for _, want := range []string{"⠋ loading", "main   W " + filepath.Base(repo), "q quit"} {
		if !strings.Contains(footer, want) {
			t.Errorf("подвал при загрузке %q, want %q", footer, want)
		}
	}
	// на экране 100 колонок без полей остаётся 98, надпись шириной 9 встаёт в
	// колонку (98-9)/2 = 44 и ещё на колонку правее из-за левого поля
	if got := ansi.StringWidth(footer[:strings.Index(footer, "⠋")]); got != 45 {
		t.Errorf("спиннер с колонки %d, want 45, по центру: %q", got, footer)
	}
}

// openThemed открывает правку a.txt с темой, где цвет добавления #050505
// (38;2;5;5;5), а цвет ошибки #070707 (38;2;7;7;7).
func openThemed(t *testing.T) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	custom := theme.Dark()
	custom.DiffAdded = lipgloss.Color("#050505")
	custom.Error = lipgloss.Color("#070707")
	opts := options(repo, layout.Focus())
	opts.Theme = custom
	return repo, resize(open(t, opts), 100, 20)
}

// Спиннер и слово loading серые (textMuted), зелёного цвета добавленных строк
// в подвале нет.
func TestSpinnerIsMuted(t *testing.T) {
	model, _ := mutedTheme(t).Update(key('r'))
	raw := model.View().Content

	for _, text := range []string{"⠋", "loading"} {
		if got := styleBefore(t, raw, "loading", text); !strings.Contains(got, mutedFg) {
			t.Errorf("перед %q стиль %q, want приглушённый цвет %s", text, got, mutedFg)
		}
	}
	if line := rawLine(t, raw, "loading"); strings.Contains(line, colorCode(38, theme.Dark().DiffAdded)) {
		t.Errorf("в подвале при загрузке есть зелёный цвет: %q", line)
	}
}

// Ошибка git по центру подвала красная.
func TestGitErrorUsesErrorColor(t *testing.T) {
	repo, model := openThemed(t)
	// без .git git не узнаёт репозиторий, и перечитывание падает
	if err := os.RemoveAll(filepath.Join(repo, ".git")); err != nil {
		t.Fatal(err)
	}

	model = pressReload(t, model)

	if got := styleBefore(t, model.View().Content, "not a git repository", "not a git"); !strings.Contains(got, "38;2;7;7;7") {
		t.Errorf("перед текстом ошибки %q, want цвет ошибки 38;2;7;7;7", got)
	}
}

// Спиннер крутится, пока идёт загрузка, и пропадает после неё. Ответы
// доставляются по одному, как их доставил бы Bubble Tea: шаг спиннера раньше
// дочитанного диффа.
func TestSpinnerTurnsWhileLoading(t *testing.T) {
	_, model := threeFileRepo(t)

	model, cmd := model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	turned := false
	for _, msg := range runAll(cmd) {
		model, _ = model.Update(msg)
		turned = turned || strings.Contains(footerOf(model), "⠙ loading")
	}

	if !turned {
		t.Error("спиннер не сделал шаг, пока шла загрузка")
	}
	if footer := footerOf(model); strings.Contains(footer, "loading") {
		t.Errorf("после загрузки подвал %q, want без loading", footer)
	}
}

// Ветка, переключённая в другом терминале, видна после тика.
func TestTickUpdatesBranch(t *testing.T) {
	repo, model := editedRepo(t, oneTick(), layout.Focus())

	runGit(t, repo, "switch", "-q", "-c", "other")
	model = settle(model, model.Init(), 3)

	if footer := footerOf(model); !strings.HasPrefix(footer, " B other   W ") {
		t.Fatalf("подвал %q, want ветка other слева", footer)
	}
}

// Над подвалом и под ним пустые строки, даже когда дифф длиннее экрана и
// когда поверх лежит окно на весь экран.
func TestFooterHasBlankRowsAround(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", numberedLines(80))
	commitAll(t, repo)
	write(t, repo, "a.txt", strings.ReplaceAll(numberedLines(80), "l", "m"))
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	for name, screen := range map[string][]string{
		"дифф":      screenOf(model),
		"окно лога": screenOf(openAsync(model, key('l'))),
	} {
		last := len(screen) - 1
		if strings.TrimSpace(screen[last]) != "" || strings.TrimSpace(screen[last-2]) != "" {
			t.Errorf("%s: вокруг подвала не пустые строки: %q, %q", name, screen[last-2], screen[last])
		}
		if !strings.Contains(screen[last-1], "q quit") {
			t.Errorf("%s: подвал не в предпоследней строке: %q", name, screen[last-1])
		}
	}
}

// Счётчик файлов переехал из подвала в заголовок.
func TestHeaderShowsFileCounter(t *testing.T) {
	_, model := threeFileRepo(t)

	if header := screenOf(model)[headerRow]; !strings.Contains(header, "(1/3)") {
		t.Fatalf("заголовок %q, want счётчик (1/3)", header)
	}
}

package tui_test

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
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

// Слева ветка и имя каталога worktree.
func TestFooterShowsBranchAndWorktree(t *testing.T) {
	repo, model := threeFileRepo(t)

	want := " main  " + filepath.Base(repo)
	if footer := footerOf(model); !strings.HasPrefix(footer, want) {
		t.Fatalf("подвал %q, want начало %q", footer, want)
	}
}

// Перечитывание по r: по центру спиннер и loading, края подвала на месте.
func TestReloadSpinnerKeepsEdges(t *testing.T) {
	repo, model := threeFileRepo(t)

	model, _ = model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})

	footer := footerOf(model)
	for _, want := range []string{"⠋ loading", "main  " + filepath.Base(repo), "q quit"} {
		if !strings.Contains(footer, want) {
			t.Errorf("подвал при загрузке %q, want %q", footer, want)
		}
	}
	if start, width := strings.Index(footer, "⠋"), ansi.StringWidth(footer); ansi.StringWidth(footer[:start]) > width/2 {
		t.Errorf("спиннер не по центру: %q", footer)
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

	if footer := footerOf(model); !strings.HasPrefix(footer, " other  ") {
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

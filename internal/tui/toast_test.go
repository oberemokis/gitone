package tui_test

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

// toastRow это строка экрана, где начинается тост: пустая строка под
// заголовком.
const toastRow = headerRow + 1

// failingHook ставит pre-commit, который отказывает с текстом script в stderr.
func failingHook(t *testing.T, repo, script string) {
	t.Helper()

	path := filepath.Join(repo, ".git", "hooks", "pre-commit")
	write(t, repo, ".git/hooks/pre-commit", "#!/bin/sh\n"+script+"\nexit 1\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// commitX открывает окно коммита, набирает тему «x», коммитит по enter и
// доводит ответ git до модели.
func commitX(model tea.Model) tea.Model {
	model, _ = model.Update(key('c'))
	model = typeText(model, "x")
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return settle(model, cmd, 4)
}

// toastEdges это колонки левой и правой рамки тоста в строке row.
func toastEdges(t *testing.T, screen []string, row int) (left, right int) {
	t.Helper()

	line := screen[row]
	first, last := strings.Index(line, "┃"), strings.LastIndex(line, "┃")
	if first < 0 || first == last {
		t.Fatalf("в строке %d нет тоста:\n%s", row, strings.Join(screen, "\n"))
	}
	return ansi.StringWidth(line[:first]), ansi.StringWidth(line[:last])
}

// borderColor это цвет правой рамки тоста в строке row.
func borderColor(t *testing.T, model tea.Model, row int) color.Color {
	t.Helper()

	_, right := toastEdges(t, screenOf(model), row)
	content := model.View().Content
	canvas := lipgloss.NewCanvas(lipgloss.Width(content), lipgloss.Height(content))
	canvas.Compose(lipgloss.NewLayer(content))
	return canvas.CellAt(right, row).Style.Fg
}

func sameRGB(a, b color.Color) bool {
	if a == nil || b == nil {
		return false
	}
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	return ar == br && ag == bg && ab == bb
}

// Ошибка хука стоит тостом справа под заголовком целиком, обе строки вывода,
// а подвал её не повторяет (#371, случаи 1 и 2).
func TestErrorToastShowsWholeText(t *testing.T) {
	repo := stagedRepo(t)
	failingHook(t, repo, "echo 'lint failed in a.txt' >&2\necho 'fix the style and commit again' >&2")
	model := commitX(resize(open(t, options(repo, layout.Default())), 100, 20))

	notice := noticeOf(model)
	for _, want := range []string{"lint failed in a.txt", "fix the style and commit again"} {
		if !strings.Contains(notice, want) {
			t.Fatalf("тост %q, ожидалась строка %q", notice, want)
		}
	}
	if screen := screenOf(model); strings.Contains(screen[footerRow(screen)], "lint failed") {
		t.Fatalf("подвал %q повторяет ошибку", screen[footerRow(screen)])
	}
	// правая рамка в двух колонках от края, тост не шире 60
	if left, right := toastEdges(t, screenOf(model), toastRow); right != 97 || right-left+1 > 60 {
		t.Fatalf("рамки тоста в колонках %d и %d, ожидалась правая в 97 и ширина до 60", left, right)
	}
}

// Длинная строка переносится по словам, на узком экране тост ужимается и не
// выходит за край (случаи 2 и 9).
func TestToastWrapsOnNarrowScreen(t *testing.T) {
	repo := stagedRepo(t)
	long := strings.TrimSpace(strings.Repeat("word ", 30))
	failingHook(t, repo, "echo '"+long+"' >&2")
	model := commitX(resize(open(t, options(repo, layout.Default())), 40, 30))

	if notice := noticeOf(model); !strings.Contains(notice, long) {
		t.Fatalf("тост %q, ожидалась строка целиком", notice)
	}
	screen := screenOf(model)
	if left, right := toastEdges(t, screen, toastRow); left < 0 || right != 37 {
		t.Fatalf("рамки тоста в колонках %d и %d, ожидались в пределах экрана, правая в 37", left, right)
	}
	for row, line := range screen {
		if width := ansi.StringWidth(line); width > 40 {
			t.Fatalf("строка %d шириной %d вышла за экран:\n%s", row, width, line)
		}
	}
}

// Итог в рамке цвета primary, ошибка в рамке цвета error. Новая заметка
// заменяет прежний тост, нажатие на главном экране его убирает (случаи 3, 5
// и 6).
func TestToastColorsAndReplacement(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)
	colors := theme.Dark()

	model = pressAndSettle(model, ctrlW)
	if notice := noticeOf(model); !strings.Contains(notice, "whitespace hidden") {
		t.Fatalf("тост %q, ожидался итог ctrl+w", notice)
	}
	if got := borderColor(t, model, toastRow); !sameRGB(got, colors.Primary) {
		t.Fatalf("рамка итога %v, ожидался primary %v", got, colors.Primary)
	}

	model = pressAndSettle(model, key('M'))
	if notice := noticeOf(model); !strings.Contains(notice, "no merge") || strings.Contains(notice, "whitespace") {
		t.Fatalf("тост %q, ожидался только отказ M", notice)
	}
	if got := borderColor(t, model, toastRow); !sameRGB(got, colors.Error) {
		t.Fatalf("рамка ошибки %v, ожидался error %v", got, colors.Error)
	}

	model = pressAndSettle(model, key('j'))
	if notice := noticeOf(model); notice != "" {
		t.Fatalf("после нажатия тост %q, ожидалось пусто", notice)
	}
}

// Отказ при открытом окне стоит тостом поверх окна, и подложка его не гасит:
// рамка ровно цвета error (случай 7).
func TestToastOverWindowIsNotShaded(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	model := openAsync(resize(open(t, options(repo, layout.Focus())), 100, 20), key('B'))

	model, _ = model.Update(key('M'))
	if notice := noticeOf(model); !strings.Contains(notice, "current branch") {
		t.Fatalf("тост %q, ожидался отказ слить текущую ветку", notice)
	}
	if body := screenText(model); !strings.Contains(body, "Branches") {
		t.Fatalf("окно веток закрылось:\n%s", body)
	}
	if got := borderColor(t, model, toastRow); !sameRGB(got, theme.Dark().Error) {
		t.Fatalf("рамка тоста %v, ожидался error %v без подложки", got, theme.Dark().Error)
	}
}

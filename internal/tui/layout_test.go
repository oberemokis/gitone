package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
)

// openLayout открывает дифф двух изменённых файлов a.txt и b.txt в раскладке.
func openLayout(t *testing.T, root layout.Node, width int) tea.Model {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "old a\n")
	write(t, repo, "b.txt", "old b\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "new a\n")
	write(t, repo, "b.txt", "new b\n")

	return resize(open(t, options(repo, root)), width, 20)
}

func TestDefaultLayoutListsFiles(t *testing.T) {
	model := openLayout(t, layout.Default(), 100)
	screen := screenOf(model)

	if body := strings.Join(screen, "\n"); !strings.Contains(body, "b.txt") {
		t.Fatalf("списка файлов нет на экране:\n%s", body)
	}
	for index, line := range screen {
		if width := ansi.StringWidth(line); width != 100 {
			t.Fatalf("строка %d шириной %d, ожидалось 100: %q", index, width, line)
		}
	}
}

// markedFile узнаёт текущий файл по стрелке на той же строке, что и имя. Между
// стрелкой и именем может стоять отметка индекса, поэтому точная строка не
// подходит.
func markedFile(screen []string, name string) bool {
	for _, line := range screen {
		if strings.Contains(line, "›") && strings.Contains(line, name) {
			return true
		}
	}

	return false
}

func TestFileListMarksCurrentFile(t *testing.T) {
	var model tea.Model = openLayout(t, layout.Default(), 100)
	if !markedFile(screenOf(model), "a.txt") {
		t.Fatalf("текущий файл не помечен:\n%s", strings.Join(screenOf(model), "\n"))
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	if screen := screenOf(model); !markedFile(screen, "b.txt") || markedFile(screen, "a.txt") {
		t.Fatalf("после ] пометка не переехала на b.txt:\n%s", strings.Join(screen, "\n"))
	}
}

func TestFocusLayoutHidesFileList(t *testing.T) {
	model := openLayout(t, layout.Focus(), 100)

	if body := strings.Join(screenOf(model), "\n"); strings.Contains(body, "b.txt") {
		t.Fatalf("в focus список файлов виден:\n%s", body)
	}
}

func TestNarrowScreenHidesFileList(t *testing.T) {
	model := openLayout(t, layout.Default(), 60)

	if body := strings.Join(screenOf(model), "\n"); strings.Contains(body, "b.txt") {
		t.Fatalf("на узком экране список файлов остался:\n%s", body)
	}
}

// Колонка диффа в default уже, чем экран: длинная строка режется по ней, а не
// по краю окна.
func TestDiffColumnIsNarrowerInDefaultLayout(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "seed\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", strings.Repeat("x", 200)+"\n")

	model := resize(open(t, options(repo, layout.Default())), 100, 20)

	for _, line := range screenOf(model) {
		if strings.Count(line, "x") > 70 {
			t.Fatalf("строка диффа шире колонки 70: %q", line)
		}
	}
}

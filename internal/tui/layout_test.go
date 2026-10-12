package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
	"gitone/internal/tui"
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

// toggleFiles нажимает f: открывает или закрывает панель файлов.
func toggleFiles(model tea.Model) tea.Model {
	model, _ = model.Update(key('f'))
	return model
}

// Панель файлов при запуске скрыта даже в раскладке, где она есть.
func TestFilesPanelHiddenAtStart(t *testing.T) {
	model := openLayout(t, layout.Default(), 100)

	if body := screenText(model); strings.Contains(body, "b.txt") {
		t.Fatalf("панель файлов видна при запуске:\n%s", body)
	}
}

// f открывает панель, второй f закрывает. Экран остаётся шириной в окно.
func TestFKeyTogglesFilesPanel(t *testing.T) {
	model := toggleFiles(openLayout(t, layout.Default(), 100))

	screen := screenOf(model)
	if body := strings.Join(screen, "\n"); !strings.Contains(body, "b.txt") {
		t.Fatalf("f не открыл панель файлов:\n%s", body)
	}
	for index, line := range screen {
		if width := ansi.StringWidth(line); width != 100 {
			t.Fatalf("строка %d шириной %d, ожидалось 100: %q", index, width, line)
		}
	}

	if body := screenText(toggleFiles(model)); strings.Contains(body, "b.txt") {
		t.Fatalf("второй f не закрыл панель:\n%s", body)
	}
}

// У открытой панели нет рамки, а фон как у окон поверх диффа.
func TestFilesPanelHasPanelBackgroundWithoutBorder(t *testing.T) {
	model := toggleFiles(openLayout(t, layout.Default(), 100))

	if body := screenText(model); strings.Contains(body, "│") {
		t.Fatalf("у панели осталась рамка:\n%s", body)
	}
	// фон backgroundPanel тёмной темы #141414
	if raw := model.View().Content; !strings.Contains(raw, "48;2;20;20;20") {
		t.Fatal("у панели нет фона backgroundPanel")
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
	model := toggleFiles(openLayout(t, layout.Default(), 100))
	if !markedFile(screenOf(model), "a.txt") {
		t.Fatalf("текущий файл не помечен:\n%s", screenText(model))
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	if screen := screenOf(model); !markedFile(screen, "b.txt") || markedFile(screen, "a.txt") {
		t.Fatalf("после ] пометка не переехала на b.txt:\n%s", strings.Join(screen, "\n"))
	}
}

// Где встаёт панель, решает раскладка; в раскладке без неё панель слева.
func TestFilesPanelSide(t *testing.T) {
	right := layout.Node{Direction: "row", Children: []layout.Node{
		{Pane: "diff"},
		{Pane: "files", Size: layout.Size{Percent: 30}},
	}}

	tests := []struct {
		name     string
		root     layout.Node
		wantLeft bool
	}{
		{name: "default слева", root: layout.Default(), wantLeft: true},
		{name: "focus слева", root: layout.Focus(), wantLeft: true},
		{name: "своя раскладка справа", root: right, wantLeft: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			screen := screenOf(toggleFiles(openLayout(t, tc.root, 100)))

			// первая строка тела: строка панели с текущим файлом
			line := screen[bodyRow]
			marker := strings.Index(line, "›")
			if marker < 0 {
				t.Fatalf("панели нет на экране: %q", line)
			}
			if left := marker < len(line)/2; left != tc.wantLeft {
				t.Fatalf("панель слева = %v, want %v: %q", left, tc.wantLeft, line)
			}
		})
	}
}

// Открыть панель решает пользователь, и на узком экране тоже.
func TestNarrowScreenOpensFilesPanel(t *testing.T) {
	model := toggleFiles(openLayout(t, layout.Default(), 60))

	if body := screenText(model); !strings.Contains(body, "b.txt") {
		t.Fatalf("на узком экране f не открыл панель:\n%s", body)
	}
}

// Без файлов f открывает пустую панель с надписью.
func TestFilesPanelOpensWithoutFiles(t *testing.T) {
	model := toggleFiles(resize(tui.New(options("", layout.Default()), nil), 120, 20))

	if got := strings.Count(screenText(model), "No changes"); got != 2 {
		t.Fatalf("«No changes» %d раз, want 2: в панели и в диффе:\n%s", got, screenText(model))
	}
}

// Колонка диффа при открытой панели уже экрана: длинная строка режется по
// ней, а не по краю окна. Панель занимает 20 колонок из 100.
func TestDiffColumnIsNarrowerWithFilesPanel(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "seed\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", strings.Repeat("x", 200)+"\n")

	model := toggleFiles(resize(open(t, options(repo, layout.Default())), 100, 20))

	for _, line := range screenOf(model) {
		if strings.Count(line, "x") > 80 {
			t.Fatalf("строка диффа шире колонки 80: %q", line)
		}
	}
}

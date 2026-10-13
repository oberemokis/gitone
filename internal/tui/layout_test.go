package tui_test

import (
	"slices"
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

// У открытой панели нет рамки, а фон как у окон поверх диффа: он стоит в каждой
// клетке панели, в строках файлов, в отступе сверху и в заполнении под
// списком, и не заходит в колонку диффа.
func TestFilesPanelHasPanelBackgroundWithoutBorder(t *testing.T) {
	model := toggleFiles(openLayout(t, layout.Default(), 100))

	if body := screenText(model); strings.Contains(body, "│") {
		t.Fatalf("у панели осталась рамка:\n%s", body)
	}

	// фон backgroundPanel тёмной темы #141414, панель 20 колонок из 100, тело
	// экрана 14 строк
	rows := strings.Split(model.View().Content, "\n")
	for row := bodyRow; row < bodyRow+14; row++ {
		cells := cellBackgrounds(rows[row])
		if got := slices.Compact(slices.Clone(cells[:20])); !slices.Equal(got, []string{"20;20;20"}) {
			t.Errorf("строка тела %d: фон панели %q, want 20;20;20 во всех клетках", row-bodyRow, got)
		}
		if slices.Contains(cells[20:], "20;20;20") {
			t.Errorf("строка тела %d: фон панели зашёл в колонку диффа", row-bodyRow)
		}
	}
}

// Внутри панели отступы, как у окон: сверху пустая строка, слева две колонки.
func TestFilesPanelPadding(t *testing.T) {
	screen := screenOf(toggleFiles(openLayout(t, layout.Default(), 100)))

	// панель шириной 20 колонок из 100
	if top := strings.TrimSpace(screen[bodyRow][:20]); top != "" {
		t.Errorf("над списком не пустая строка: %q", screen[bodyRow])
	}
	if first := screen[bodyRow+1]; !strings.HasPrefix(first, "  › a.txt") {
		t.Errorf("первая строка списка %q, want «  › a.txt»", first)
	}
	if second := screen[bodyRow+2]; !strings.HasPrefix(second, "    b.txt") {
		t.Errorf("вторая строка списка %q, want «    b.txt»", second)
	}
}

// Справа у списка те же две колонки отступа, что слева: длинное имя обрезается
// «…» за две колонки до края панели, а не вплотную к диффу.
func TestFilesPanelKeepsRightPadding(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "very-long-directory-name.txt", "old\n")
	commitAll(t, repo)
	write(t, repo, "very-long-directory-name.txt", "new\n")

	model := toggleFiles(resize(open(t, options(repo, layout.Default())), 100, 20))

	// панель 20 колонок: две колонки отступа, «› », 13 символов имени и «…»
	// дают 18, ещё две колонки справа пустые
	row := []rune(screenOf(model)[bodyRow+1])
	if got, want := string(row[:20]), "  › very-long-dir…  "; got != want {
		t.Errorf("строка панели %q, want %q", got, want)
	}
}

// markedFile узнаёт текущий файл по стрелке на той же строке, что и имя.
// Заголовок не в счёт: там › это стрелка к следующему файлу (#120).
func markedFile(screen []string, name string) bool {
	for row, line := range screen {
		if row != headerRow && strings.Contains(line, "›") && strings.Contains(line, name) {
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

			// вторая строка тела: под отступом панели строка с текущим файлом
			line := screen[bodyRow+1]
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

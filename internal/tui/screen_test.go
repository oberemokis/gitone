package tui_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/tui"
)

func TestFirstChangedFileOpensWithPathAndCounts(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\ntwo\n")
	write(t, repo, "b.txt", "x\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\nTWO\nthree\n")
	write(t, repo, "b.txt", "y\n")

	screen := render(open(t, repo, nil), 100, 20)

	if header := strings.TrimSpace(screen[0]); header != "a.txt +2-1" {
		t.Fatalf("заголовок %q, ожидался %q", header, "a.txt +2-1")
	}
	if body := strings.Join(screen[1:], "\n"); !strings.Contains(body, "TWO") || strings.Contains(body, "y") {
		t.Fatalf("на экране не первый файл:\n%s", body)
	}
}

func TestModeFollowsWidthUntilToggled(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\ntwo\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\nTWO\n")
	model := open(t, repo, nil)
	toggle := tea.KeyPressMsg{Code: 'm', Text: "m"}

	steps := []struct {
		name      string
		msg       tea.Msg
		wantSplit bool
	}{
		{name: "ширина 120", msg: tea.WindowSizeMsg{Width: 120, Height: 20}, wantSplit: true},
		{name: "ширина 119", msg: tea.WindowSizeMsg{Width: 119, Height: 20}, wantSplit: false},
		{name: "m на 119", msg: toggle, wantSplit: true},
		{name: "после m ширина 100", msg: tea.WindowSizeMsg{Width: 100, Height: 20}, wantSplit: true},
		{name: "второй m", msg: toggle, wantSplit: false},
		{name: "после второго m ширина 150", msg: tea.WindowSizeMsg{Width: 150, Height: 20}, wantSplit: false},
	}

	for _, step := range steps {
		model, _ = model.Update(step.msg)

		if got := isSplit(screenOf(model), "two", "TWO"); got != step.wantSplit {
			t.Fatalf("%s: split %v, ожидалось %v\n%s", step.name, got, step.wantSplit, strings.Join(screenOf(model), "\n"))
		}
	}
}

func TestLongLinesAreCutAtColumnEdgeAndTabsBecomeSpaces(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "short\n"+strings.Repeat("x", 300)+"\n\ttabbed\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "SHORT\n"+strings.Repeat("y", 300)+"\n\tTABBED\n")

	screen := render(open(t, repo, nil), 120, 20)

	for index, line := range screen[2 : len(screen)-1] {
		if width := ansi.StringWidth(line); width != 120 {
			t.Fatalf("строка тела %d шириной %d, ожидалось 120: %q", index, width, line)
		}
	}
	if body := strings.Join(screen, "\n"); !strings.Contains(body, "    tabbed") || strings.Contains(body, "\t") {
		t.Fatalf("таб не заменён четырьмя пробелами:\n%s", body)
	}
}

func TestScrollStopsAtBothEnds(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "seed.txt", "seed\n")
	commitAll(t, repo)
	var content strings.Builder
	for number := 1; number <= 60; number++ {
		fmt.Fprintf(&content, "l%d\n", number)
	}
	write(t, repo, "long.txt", content.String())

	// высота 20: заголовок с пустой строкой и подвал оставляют телу 17 строк
	var model tea.Model = open(t, repo, nil)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	steps := []struct {
		name    string
		msg     tea.Msg
		wantTop int
	}{
		{name: "j", msg: tea.KeyPressMsg{Code: 'j', Text: "j"}, wantTop: 2},
		{name: "стрелка вниз", msg: tea.KeyPressMsg{Code: tea.KeyDown}, wantTop: 3},
		{name: "k", msg: tea.KeyPressMsg{Code: 'k', Text: "k"}, wantTop: 2},
		{name: "стрелка вверх", msg: tea.KeyPressMsg{Code: tea.KeyUp}, wantTop: 1},
		{name: "k в начале", msg: tea.KeyPressMsg{Code: 'k', Text: "k"}, wantTop: 1},
		{name: "ctrl d", msg: tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}, wantTop: 9},
		{name: "колесо вниз", msg: tea.MouseWheelMsg{Button: tea.MouseWheelDown}, wantTop: 12},
		{name: "колесо вверх", msg: tea.MouseWheelMsg{Button: tea.MouseWheelUp}, wantTop: 9},
		{name: "ctrl u", msg: tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}, wantTop: 1},
		{name: "G", msg: tea.KeyPressMsg{Code: 'g', Text: "G", Mod: tea.ModShift}, wantTop: 44},
		{name: "j в конце", msg: tea.KeyPressMsg{Code: 'j', Text: "j"}, wantTop: 44},
		{name: "колесо в конце", msg: tea.MouseWheelMsg{Button: tea.MouseWheelDown}, wantTop: 44},
		{name: "g", msg: tea.KeyPressMsg{Code: 'g', Text: "g"}, wantTop: 1},
	}

	for _, step := range steps {
		model, _ = model.Update(step.msg)

		if got := topNumber(t, screenOf(model)); got != step.wantTop {
			t.Fatalf("%s: первая строка тела %d, ожидалась %d", step.name, got, step.wantTop)
		}
	}
}

func TestModeToggleKeepsTopLine(t *testing.T) {
	repo := newRepo(t)
	var before, after strings.Builder
	for number := 1; number <= 40; number++ {
		fmt.Fprintf(&before, "r%d\n", number)
		fmt.Fprintf(&after, "a%d\n", number)
	}
	write(t, repo, "a.txt", before.String())
	commitAll(t, repo)
	write(t, repo, "a.txt", after.String())

	// unified: сорок удалённых строк, затем сорок добавленных. Пять раз на
	// полэкрана (по 8 строк) ставят наверх первую добавленную, её номер 1.
	var model tea.Model = open(t, repo, nil)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	for range 5 {
		model, _ = model.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	}
	if top := strings.Fields(screenOf(model)[2]); len(top) < 3 || top[2] != "a1" {
		t.Fatalf("наверху не первая добавленная строка: %v", top)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})

	if got := topNumber(t, screenOf(model)); got != 1 {
		t.Fatalf("после m наверху строка %d, ожидалась та же строка 1", got)
	}
}

func TestSpecialFilesShowNotices(t *testing.T) {
	cases := []struct {
		name   string
		before map[string]string
		change func(t *testing.T, repo string)
		args   []string
		want   string
	}{
		{
			name:   "бинарный файл",
			before: map[string]string{"image.bin": "\x00\x01"},
			change: func(t *testing.T, repo string) { write(t, repo, "image.bin", "\x00\x02") },
			want:   "Binary or large file",
		},
		{
			name:   "смена прав",
			before: map[string]string{"script.sh": "echo\n"},
			change: func(t *testing.T, repo string) {
				if err := os.Chmod(filepath.Join(repo, "script.sh"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: "mode 644 → 755",
		},
		{
			name:   "пустой новый файл",
			before: map[string]string{"seed.txt": "seed\n"},
			change: func(t *testing.T, repo string) { write(t, repo, "empty.txt", "") },
			want:   "No content changes",
		},
		{
			name:   "нет перевода строки в конце",
			before: map[string]string{"a.txt": "a\nb\n"},
			change: func(t *testing.T, repo string) { write(t, repo, "a.txt", "a\nB") },
			want:   "No newline at end of file",
		},
		{
			name:   "нераспознанный вывод git показывается как есть",
			before: map[string]string{"a.txt": "a\n"},
			change: func(t *testing.T, repo string) { write(t, repo, "a.txt", "b\n") },
			args:   []string{"--stat"},
			want:   "1 file changed",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := newRepo(t)
			for name, content := range testCase.before {
				write(t, repo, name, content)
			}
			commitAll(t, repo)
			testCase.change(t, repo)

			screen := render(open(t, repo, testCase.args), 100, 20)

			if body := strings.Join(screen, "\n"); !strings.Contains(body, testCase.want) {
				t.Fatalf("на экране нет %q:\n%s", testCase.want, body)
			}
		})
	}
}

// topNumber читает номер строки файла в первой строке тела: в unified у
// добавленной строки старого номера нет, первым идёт новый.
func topNumber(t *testing.T, screen []string) int {
	t.Helper()

	fields := strings.Fields(screen[2])
	if len(fields) == 0 {
		t.Fatalf("первая строка тела пустая:\n%s", strings.Join(screen, "\n"))
	}

	number, err := strconv.Atoi(fields[0])
	if err != nil {
		t.Fatalf("в первой строке тела нет номера: %q", screen[2])
	}

	return number
}

// isSplit узнаёт split по тому, что удалённая и добавленная строки стоят на
// одной строке экрана.
func isSplit(screen []string, removed, added string) bool {
	for _, line := range screen {
		if strings.Contains(line, removed) && strings.Contains(line, added) {
			return true
		}
	}

	return false
}

// open загружает дифф репозитория так же, как cmd/gitone при запуске.
func open(t *testing.T, repo string, args []string) tea.Model {
	t.Helper()

	files, err := tui.Load(repo, args)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	return tui.New(files)
}

// render задаёт размер окна и возвращает экран построчно без цветов.
func render(model tea.Model, width, height int) []string {
	model, _ = model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return screenOf(model)
}

func screenOf(model tea.Model) []string {
	return strings.Split(ansi.Strip(model.View().Content), "\n")
}

// newRepo создаёт репозиторий во временном каталоге. Глобальный и системный
// конфиги отключены: результат не должен зависеть от настроек машины.
func newRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	repo := t.TempDir()
	runGit(t, repo, "init", "-q", "-b", "main")
	return repo
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	command := exec.Command("git", args...)
	command.Dir = dir
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()

	target := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, repo string) {
	t.Helper()
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "commit")
}

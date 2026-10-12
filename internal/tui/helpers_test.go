package tui_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/config"
	"gitone/internal/keymap"
	"gitone/internal/layout"
	"gitone/internal/tui"
	"gitone/internal/tui/theme"
)

// Хелперы тестов экрана: репозиторий, открытие модели, чтение экрана и
// выполнение команд так, как это делает Bubble Tea.

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

// runGit запускает git в каталоге и возвращает вывод без пробелов по краям.
// Сбой git валит тест вместе с текстом ошибки git.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	var stderr strings.Builder
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, stderr.String())
	}

	return strings.TrimSpace(string(out))
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

func readText(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func commitAll(t *testing.T, repo string) {
	t.Helper()
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "commit")
}

// threeFileRepo это репозиторий с коммитом и правками в a.txt, b.txt и c.txt,
// открытый в один дифф на экране 100 на 20.
func threeFileRepo(t *testing.T) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		write(t, repo, name, "old "+name+"\n")
	}
	commitAll(t, repo)
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		write(t, repo, name, "new "+name+"\n")
	}

	return repo, resize(open(t, options(repo, layout.Focus())), 100, 20)
}

// numberedLines это count строк l1, l2, … с переводом строки в конце.
func numberedLines(count int) string {
	var content strings.Builder
	for number := 1; number <= count; number++ {
		fmt.Fprintf(&content, "l%d\n", number)
	}
	return content.String()
}

// options это встроенные настройки, с которыми cmd/gitone запускается без
// файлов настроек. Раскладка параметром: в layout.Focus() на экране один
// дифф, и тесты диффа не видят панель файлов.
func options(repo string, root layout.Node) tui.Options {
	return tui.Options{Dir: repo, Config: config.Default(), Keys: keymap.Default(), Theme: theme.Dark(), Layout: root}
}

// open загружает дифф каталога opts.Dir с аргументами opts.Args так же, как
// cmd/gitone при запуске.
func open(t *testing.T, opts tui.Options) tea.Model {
	t.Helper()

	files, err := tui.Load(opts.Dir, opts.Args)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	return tui.New(opts, files)
}

func resize(model tea.Model, width, height int) tea.Model {
	model, _ = model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return model
}

// render задаёт размер окна и возвращает экран построчно без цветов.
func render(model tea.Model, width, height int) []string {
	return screenOf(resize(model, width, height))
}

func screenOf(model tea.Model) []string {
	return strings.Split(ansi.Strip(model.View().Content), "\n")
}

// openedFile читает путь открытого файла из заголовка: первое слово.
func openedFile(screen []string) string {
	fields := strings.Fields(screen[0])
	if len(fields) == 0 {
		return ""
	}

	return fields[0]
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

// typeText набирает текст по одной клавише, как с клавиатуры.
func typeText(model tea.Model, text string) tea.Model {
	for _, char := range text {
		model, _ = model.Update(tea.KeyPressMsg{Code: char, Text: string(char)})
	}

	return model
}

// pressReload нажимает r и доводит перечитывание до конца так же, как Bubble
// Tea: команда выполняется, её сообщение возвращается в модель.
func pressReload(t *testing.T, model tea.Model) tea.Model {
	t.Helper()

	model, cmd := model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if cmd == nil {
		t.Fatal("r не запустил перечитывание")
	}
	model, _ = model.Update(cmd())
	return model
}

// runAll выполняет команду и её части, если она пакетная. Части, которые
// блокируются без сигнала, отбрасываются по времени.
func runAll(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}

	results := make(chan tea.Msg, 8)
	go func() {
		msg := cmd()
		batch, ok := msg.(tea.BatchMsg)
		if !ok {
			results <- msg
			return
		}
		for _, sub := range batch {
			go func(sub tea.Cmd) { results <- sub() }(sub)
		}
	}()

	var msgs []tea.Msg
	for {
		select {
		case msg := <-results:
			msgs = append(msgs, msg)
		case <-time.After(500 * time.Millisecond):
			return msgs
		}
	}
}

// settle выполняет команду и её ответы, пока команды дают ответы. Ограничение
// глубины не даёт зациклиться на ожидании сигнала.
func settle(model tea.Model, cmd tea.Cmd, depth int) tea.Model {
	if depth == 0 {
		return model
	}
	for _, msg := range runAll(cmd) {
		var next tea.Cmd
		model, next = model.Update(msg)
		model = settle(model, next, depth-1)
	}
	return model
}

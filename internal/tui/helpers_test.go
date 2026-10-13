package tui_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
// конфиги отключены, язык C: результат не должен зависеть от настроек машины,
// а тесты сверяют английские сообщения git. Git с gettext (Homebrew) при C
// игнорирует и LANGUAGE.
func newRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("LC_ALL", "C")
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
// дифф, и тесты диффа не видят панель файлов. Заметки подвала в нём не гаснут:
// settle доводит команды до конца и ждал бы таймер в пять секунд, а потом
// стёр бы заметку, которую тест хочет увидеть. Гашение проверяет notice_test.go.
func options(repo string, root layout.Node) tui.Options {
	return tui.Options{Dir: repo, Config: config.Default(), Keys: keymap.Default(), Theme: theme.Dark(), Layout: root, KeepNotices: true}
}

// open загружает дифф каталога opts.Dir с аргументами opts.Args, индекс и
// место подвала так же, как cmd/gitone при запуске: space решает по индексу.
func open(t *testing.T, opts tui.Options) tea.Model {
	t.Helper()

	files, err := tui.Load(opts.Dir, opts.Args)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if opts.Staged, err = tui.Staged(opts.Dir); err != nil {
		t.Fatalf("Staged: %v", err)
	}
	if opts.Location, err = tui.Locate(opts.Dir); err != nil {
		t.Fatalf("Locate: %v", err)
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

// Строки экрана с заголовком: пустая, заголовок, пустая, дальше тело.
const (
	headerRow = 1
	bodyRow   = 3
)

// headerText это строка заголовка без стрелок по краям (#120), клавиш рядом с
// ними (#195) и пробелов. Клавиши здесь встроенные: «[» и «]».
func headerText(screen []string) string {
	return strings.Trim(screen[headerRow], "‹›[] ")
}

// openedFile читает путь открытого файла из заголовка: первое слово.
func openedFile(screen []string) string {
	fields := strings.Fields(headerText(screen))
	if len(fields) == 0 {
		return ""
	}

	return fields[0]
}

// topNumber читает номер строки файла в первой строке тела: в unified у
// добавленной строки старого номера нет, первым идёт новый.
func topNumber(t *testing.T, screen []string) int {
	t.Helper()

	fields := strings.Fields(screen[bodyRow])
	if len(fields) == 0 {
		t.Fatalf("первая строка тела пустая:\n%s", strings.Join(screen, "\n"))
	}

	number, err := strconv.Atoi(fields[0])
	if err != nil {
		t.Fatalf("в первой строке тела нет номера: %q", screen[bodyRow])
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
// Tea: команда выполняется, её ответы возвращаются в модель. Следующий шаг
// спиннера не выполняется: после загрузки он ничего не меняет.
func pressReload(t *testing.T, model tea.Model) tea.Model {
	t.Helper()

	model, cmd := model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if cmd == nil {
		t.Fatal("r не запустил перечитывание")
	}
	for _, msg := range runAll(cmd) {
		model, _ = model.Update(msg)
	}
	return model
}

// oneTick это таймер опроса с одним тиком. Канал закрыт: ожидание следующего
// тика возвращается сразу, иначе runAll ждал бы его вечно.
func oneTick() <-chan time.Time {
	ticks := make(chan time.Time, 1)
	ticks <- time.Time{}
	close(ticks)
	return ticks
}

// runAll выполняет команду, как Bubble Tea: части пакетной команды идут
// одновременно. Ждёт все части и возвращает ответы в порядке пакета, а не в
// порядке прихода. Ожидание тика возвращается только по тику или закрытию
// канала, поэтому тест с таймером берёт закрытый канал из oneTick.
func runAll(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}

	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}

	msgs := make([]tea.Msg, len(batch))
	var wg sync.WaitGroup
	for index, sub := range batch {
		wg.Go(func() { msgs[index] = sub() })
	}
	wg.Wait()

	return msgs
}

// settle выполняет команду и её ответы, пока команды дают ответы. Глубина
// ограничивает цепочку: перечитывание по ответу может запустить следующее.
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

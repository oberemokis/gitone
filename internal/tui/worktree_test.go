package tui_test

import (
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

// twoWorktrees это репозиторий с правкой «main-edit» и соседний worktree
// side-tree на ветке side с правкой «side-edit». gitone открыт в основном.
func twoWorktrees(t *testing.T) (repo, side string, model tea.Model) {
	t.Helper()

	repo = newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	side = filepath.Join(t.TempDir(), "side-tree")
	runGit(t, repo, "worktree", "add", "-q", "-b", "side", side)
	write(t, repo, "a.txt", "main-edit\n")
	write(t, side, "a.txt", "side-edit\n")

	opts := options(repo, layout.Focus())
	opts.Ticks = oneTick()
	return repo, side, resize(open(t, opts), 100, 20)
}

// W открывает список worktree, текущий отмечен после имени каталога, курсор на
// нём.
func TestWorktreeWindowListsTrees(t *testing.T) {
	repo, _, model := twoWorktrees(t)

	model = openAsync(model, key('W'))

	screen := screenText(model)
	current := "› " + filepath.Base(repo) + " (current)"
	if !strings.Contains(screen, current) || !strings.Contains(screen, "  side-tree") {
		t.Fatalf("в окне нет обоих worktree с отметкой текущего:\n%s", screen)
	}
}

// Окно worktree это таблица: заголовки «Worktree», «Branch», «Path», а имя
// каталога, ветка и путь стоят под ними с одной колонки при разной длине имён.
func TestWorktreeColumnsLineUp(t *testing.T) {
	screen := screenOf(openAsync(sideWorktree(t, "long-tree-name", "feature-x"), key('W')))

	// «Worktree» есть и в названии окна, поэтому строка заголовков ищется по Branch
	header := rowWith(t, screen, "Branch")
	current, other := rowWith(t, screen, "(current)"), rowWith(t, screen, "long-tree-name")
	for _, check := range []struct{ title, row, text string }{
		{title: "Worktree", row: other, text: "long-tree-name"},
		{title: "Branch", row: current, text: "main"},
		{title: "Branch", row: other, text: "feature-x"},
		// первый «/» в строке это начало пути
		{title: "Path", row: current, text: "/"},
		{title: "Path", row: other, text: "/"},
	} {
		want, got := column(header, check.title), column(check.row, check.text)
		if want < 0 || got != want {
			t.Errorf("%q с колонки %d, заголовок %q с %d:\n%s\n%s", check.text, got, check.title, want, header, check.row)
		}
	}
	if column(current, "(current)") > column(current, "main") {
		t.Errorf("отметка текущего не до ветки: %q", current)
	}
}

// Длинные имя каталога и ветка обрезаются «…» до предела столбца, окно остаётся
// шириной 90: на экране 140 колонок «close» кончается в колонке 25+90-2.
func TestWorktreeLongValuesAreTruncated(t *testing.T) {
	screen := screenOf(openAsync(sideWorktree(t, "worktree-directory-with-a-long-name", "feature-a-really-long-branch-name"), key('W')))

	row := rowWith(t, screen, "worktree-directory-with…")
	if !strings.Contains(row, "feature-a-really-long-b…") {
		t.Errorf("ветка не обрезана «…» до предела столбца: %q", row)
	}
	if end := column(rowWith(t, screen, "close"), "close") + len("close"); end != 113 {
		t.Errorf("окно кончается в колонке %d, want 113 (ширина 90)", end)
	}
}

// На узком экране окно ужимается, и ни одна строка не выходит за экран.
func TestWorktreeWindowFitsNarrowScreen(t *testing.T) {
	model := resize(sideWorktree(t, "worktree-directory-with-a-long-name", "feature-a-really-long-branch-name"), 50, 20)

	screen := screenOf(openAsync(model, key('W')))

	for _, line := range screen {
		if width := ansi.StringWidth(line); width > 50 {
			t.Errorf("строка шире экрана (%d): %q", width, line)
		}
	}
	if row := rowWith(t, screen, "worktree-d"); !strings.Contains(row, "…") {
		t.Errorf("строка не обрезана «…»: %q", row)
	}
}

// При detached HEAD в столбце «Branch» короткий хэш, те же семь символов, что в
// подвале.
func TestWorktreeDetachedHeadShowsShortHash(t *testing.T) {
	repo, model := threeFileRepo(t)
	runGit(t, repo, "checkout", "-q", "--detach")
	hash := runGit(t, repo, "rev-parse", "HEAD")[:7]

	screen := screenOf(openAsync(model, key('W')))

	header, row := rowWith(t, screen, "Branch"), rowWith(t, screen, "(current)")
	if got, want := column(row, hash), column(header, "Branch"); want < 0 || got != want {
		t.Errorf("хэш %s с колонки %d, заголовок Branch с %d:\n%s\n%s", hash, got, want, header, row)
	}
}

// Окно worktree красит имя каталога цветом роли worktree из темы.
func TestWorktreeWindowFollowsTheme(t *testing.T) {
	repo, _ := threeFileRepo(t)
	custom := theme.Dark()
	custom.Worktree = lipgloss.Color("#ff0000")
	opts := options(repo, layout.Focus())
	opts.Theme = custom
	model := resize(open(t, opts), 100, 20)

	raw := openAsync(model, key('W')).View().Content

	if code := styleBefore(t, raw, "(current)", filepath.Base(repo)); !strings.Contains(code, "38;2;255;0;0") {
		t.Errorf("имя каталога не цветом темы: %q", code)
	}
}

// enter на соседнем: дифф его рабочего дерева, в подвале его имя.
func TestWorktreeSwitchShowsItsDiff(t *testing.T) {
	_, _, model := twoWorktrees(t)

	model = openAsync(model, key('W'))
	model, _ = model.Update(key('j'))
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 3)

	if screen := screenText(model); !strings.Contains(screen, "side-edit") || strings.Contains(screen, "main-edit") {
		t.Fatalf("после переключения не дифф side-tree:\n%s", screen)
	}
	if footer := footerOf(model); !strings.HasPrefix(footer, " B side   W side-tree") {
		t.Fatalf("подвал %q, want side-tree слева", footer)
	}
}

// switchPending выбирает side-tree в окне worktree и возвращает проверку его
// каталога недоставленной: она идёт, пока пользователь работает дальше.
func switchPending(model tea.Model) (tea.Model, tea.Cmd) {
	model = openAsync(model, key('W'))
	model, _ = model.Update(key('j'))
	return model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// Пока шла проверка каталога, открыли лог прежнего дерева и меню reset.
// Переключение их закрывает: hard и y после него не стирают правку side-tree
// (#380, случай 1).
func TestWorktreeSwitchClosesOpenWindows(t *testing.T) {
	_, side, model := twoWorktrees(t)
	head := runGit(t, side, "rev-parse", "HEAD")

	model, check := switchPending(model)
	model = openAsync(model, key('l'))
	model, _ = model.Update(key('g'))
	model = settle(model, check, 3)
	if body := screenText(model); strings.Contains(body, "Hard: discard") || !strings.Contains(body, "side-edit") {
		t.Fatalf("после переключения ожидался дифф side-tree без меню reset:\n%s", body)
	}

	answerYes(hardReset(model))
	if got := readText(t, filepath.Join(side, "a.txt")); got != "side-edit\n" || runGit(t, side, "rev-parse", "HEAD") != head {
		t.Fatalf("side-tree изменён: a.txt %q", got)
	}
}

// Лог, запрошенный в прежнем дереве и пришедший после переключения, окна не
// открывает: в нём коммиты прежнего дерева (случай 2).
func TestWorktreeSwitchDropsOldList(t *testing.T) {
	_, _, model := twoWorktrees(t)

	model, check := switchPending(model)
	model, log := model.Update(key('l'))
	model = settle(model, check, 3)
	model = settle(model, log, 2)
	if body := screenText(model); strings.Contains(body, "Log") {
		t.Fatalf("лог прежнего дерева открылся в side-tree:\n%s", body)
	}
}

// enter на текущем закрывает окно и ничего не перечитывает; esc закрывает.
func TestWorktreeWindowCloses(t *testing.T) {
	for _, press := range []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: tea.KeyEscape}} {
		_, _, model := twoWorktrees(t)
		model = openAsync(model, key('W'))

		model, cmd := model.Update(press)
		if cmd != nil {
			t.Errorf("%s на текущем дал команду, want ничего", press)
		}
		if strings.Contains(screenText(model), "Worktrees") {
			t.Errorf("%s не закрыл окно:\n%s", press, screenText(model))
		}
	}
}

// Аргументы запуска относятся к прежнему worktree: после переключения идёт
// дифф рабочего дерева нового, и b возвращает к нему же, а не к аргументам.
func TestWorktreeSwitchDropsLaunchArgs(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)
	side := filepath.Join(t.TempDir(), "side-tree")
	runGit(t, repo, "worktree", "add", "-q", "-b", "side", side)
	write(t, side, "a.txt", "side-edit\n")

	// у запуска дифф последнего коммита: «- one» и «+ two»
	opts := options(repo, layout.Focus())
	opts.Args = []string{"HEAD~1", "HEAD"}
	model := resize(open(t, opts), 100, 20)

	model = openAsync(model, key('W'))
	model, _ = model.Update(key('j'))
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 3)
	if screen := screenText(model); !strings.Contains(screen, "+ side-edit") {
		t.Fatalf("после переключения не рабочее дерево side-tree:\n%s", screen)
	}

	model, cmd = model.Update(key('b'))
	model = settle(model, cmd, 3)
	if screen := screenText(model); !strings.Contains(screen, "+ side-edit") {
		t.Fatalf("b вернул не рабочее дерево side-tree, а аргументы запуска:\n%s", screen)
	}
}

// Опрос, ушедший до переключения, не возвращает дифф старого worktree.
func TestLatePollKeepsSwitchedWorktree(t *testing.T) {
	_, _, model := twoWorktrees(t)

	model, polling := model.Update(model.Init()())
	model = openAsync(model, key('W'))
	model, _ = model.Update(key('j'))
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 3)
	model = settle(model, polling, 2)

	if screen := screenText(model); strings.Contains(screen, "main-edit") {
		t.Fatalf("опрос старого каталога затёр дифф side-tree:\n%s", screen)
	}
}

// Запертое дерево на отключённом диске git держит в списке без prunable. Выбор
// такого дерева оставляет прежнее, в подвале причина, а W, b и l работают.
func TestWorktreeSwitchToMissingDirectoryKeepsCurrentTree(t *testing.T) {
	repo, side, model := twoWorktrees(t)
	runGit(t, repo, "worktree", "lock", side)
	if err := os.RemoveAll(side); err != nil {
		t.Fatal(err)
	}
	// длинный путь временного каталога не должен обрезать причину в подвале
	model = resize(model, 300, 20)

	model = openAsync(model, key('W'))
	model, _ = model.Update(key('j'))
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 3)

	if screen := screenText(model); !strings.Contains(screen, "main-edit") {
		t.Fatalf("прежний дифф пропал с экрана:\n%s", screen)
	}
	if footer := footerOf(model); !strings.Contains(footer, "side-tree: no such file or directory") {
		t.Fatalf("в подвале %q, ожидалась причина отказа", footer)
	}

	for press, title := range map[rune]string{'W': "Worktrees", 'B': "Branches", 'l': "Log"} {
		opened := openAsync(model, key(press))
		if screen := screenText(opened); !strings.Contains(screen, title) {
			t.Errorf("после отказа клавиша %q не открыла окно %s:\n%s", press, title, screen)
		}
	}

	model, cmd = model.Update(key('b'))
	model = settle(model, cmd, 3)
	if footer := footerOf(model); strings.Contains(footer, "no such file") {
		t.Errorf("после отказа b читает дифф недоступного каталога: %q", footer)
	}
}

// В репозитории с одним worktree окно открывается с одной строкой.
func TestSingleWorktreeWindow(t *testing.T) {
	repo, model := threeFileRepo(t)

	model = openAsync(model, key('W'))

	if screen := screenText(model); !strings.Contains(screen, "› "+filepath.Base(repo)+" (current)  main") {
		t.Fatalf("окно без единственного worktree:\n%s", screen)
	}
}

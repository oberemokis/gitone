package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
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

// W открывает список worktree, текущий отмечен, курсор на нём.
func TestWorktreeWindowListsTrees(t *testing.T) {
	repo, _, model := twoWorktrees(t)

	model = openAsync(model, key('W'))

	screen := screenText(model)
	current := "› " + filepath.Base(repo) + "  main"
	if !strings.Contains(screen, current) || !strings.Contains(screen, "(current)") || !strings.Contains(screen, "side-tree  side") {
		t.Fatalf("в окне нет обоих worktree с отметкой текущего:\n%s", screen)
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

	if screen := screenText(model); !strings.Contains(screen, "› "+filepath.Base(repo)+"  main") {
		t.Fatalf("окно без единственного worktree:\n%s", screen)
	}
}

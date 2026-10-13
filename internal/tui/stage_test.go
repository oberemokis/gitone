package tui_test

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/layout"
)

// Пробел ставит текущий файл в индекс. Отметки в панели нет (#99): индекс
// виден в git.
func TestSpaceStagesCurrentFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	model := toggleFiles(resize(open(t, options(repo, layout.Default())), 100, 20))
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	model = settle(model, cmd, 4)

	if staged, err := git.StagedPaths(repo); err != nil || len(staged) != 1 || staged[0] != "a.txt" {
		t.Fatalf("в индексе %q, ошибка %v", staged, err)
	}
	if body := strings.Join(screenOf(model), "\n"); strings.Contains(body, "●") {
		t.Fatalf("в панели осталась отметка индекса:\n%s", body)
	}
}

// x спрашивает, и только y отменяет правку. Любая другая клавиша отменяет
// вопрос и не выполняет действие.
func TestDiscardNeedsYes(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	model := resize(open(t, options(repo, layout.Default())), 100, 20)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if footer := footerOf(model); !strings.Contains(footer, "Discard changes") {
		t.Fatalf("вопроса нет в подвале: %q", footer)
	}

	model, cmd := model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	model = settle(model, cmd, 4)
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "two\n" {
		t.Fatalf("правка пропала без подтверждения: %q", got)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	model, cmd = model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	settle(model, cmd, 4)
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "one\n" {
		t.Fatalf("правка не отменена после y: %q", got)
	}
}

var space = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}

// Пробел ставит в индекс только текущий файл, а не все изменённые.
func TestSpaceStagesOnlyCurrentFile(t *testing.T) {
	repo, model := threeFileRepo(t)

	model, cmd := model.Update(space)
	settle(model, cmd, 4)

	if got := runGit(t, repo, "diff", "--cached", "--name-only"); got != "a.txt" {
		t.Fatalf("в индексе %q, want только a.txt", got)
	}
}

// x и y отменяют правку только текущего файла, чужие правки остаются.
func TestDiscardTouchesOnlyCurrentFile(t *testing.T) {
	repo, model := threeFileRepo(t)

	model, _ = model.Update(key('x'))
	answerYes(model)

	if got := readText(t, filepath.Join(repo, "a.txt")); got != "old a.txt\n" {
		t.Fatalf("a.txt = %q, want правка отменена", got)
	}
	if got := readText(t, filepath.Join(repo, "b.txt")); got != "new b.txt\n" {
		t.Fatalf("b.txt = %q, want правка осталась", got)
	}
	if got := readText(t, filepath.Join(repo, "c.txt")); got != "new c.txt\n" {
		t.Fatalf("c.txt = %q, want правка осталась", got)
	}
}

// У сырой записи («git output») пути нет: пробел не ставит в индекс всё
// дерево с неотслеженными файлами, x не задаёт вопроса, а y ничего не стирает.
func TestRawEntryHasNoFileToStageOrDiscard(t *testing.T) {
	repo := newRepo(t)
	for _, name := range []string{"a.txt", "b.txt"} {
		write(t, repo, name, "old "+name+"\n")
	}
	commitAll(t, repo)
	for _, name := range []string{"a.txt", "b.txt", "secret.env"} {
		write(t, repo, name, "new "+name+"\n")
	}
	opts := options(repo, layout.Focus())
	opts.Args = []string{"--stat"}
	model := resize(open(t, opts), 100, 20)

	model, cmd := model.Update(space)
	model = settle(model, cmd, 4)
	if got := runGit(t, repo, "ls-files"); got != "a.txt\nb.txt" {
		t.Fatalf("в индексе %q, want a.txt и b.txt без secret.env", got)
	}
	if footer := footerOf(model); !strings.Contains(footer, "this entry has no file") {
		t.Fatalf("после пробела подвал = %q, want заметку", footer)
	}

	model, _ = model.Update(key('x'))
	footer := footerOf(model)
	if strings.Contains(footer, "Discard changes") || !strings.Contains(footer, "this entry has no file") {
		t.Fatalf("после x подвал = %q, want заметку без вопроса", footer)
	}
	answerYes(model)
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "new a.txt\n" {
		t.Fatalf("a.txt = %q, want правка осталась", got)
	}
}

// Копия (diff.renames=copies): x и y убирают копию b.txt, а правка источника
// a.txt остаётся.
func TestDiscardCopyKeepsSource(t *testing.T) {
	repo := newRepo(t)
	runGit(t, repo, "config", "diff.renames", "copies")
	write(t, repo, "a.txt", "l1\nl2\nl3\nl4\nl5\nl6\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "l1\nl2\nl3\nl4\nl5\nl6\nl7\n")
	write(t, repo, "b.txt", "l1\nl2\nl3\nl4\nl5\nl6\n")

	model := resize(open(t, options(repo, layout.Focus())), 100, 20)
	model, _ = model.Update(key(']'))
	if title := headerText(screenOf(model)); !strings.Contains(title, "a.txt → b.txt") {
		t.Fatalf("заголовок = %q, want копию a.txt → b.txt", title)
	}

	model, _ = model.Update(key('x'))
	answerYes(model)

	if got := readText(t, filepath.Join(repo, "a.txt")); got != "l1\nl2\nl3\nl4\nl5\nl6\nl7\n" {
		t.Fatalf("a.txt = %q, want правка источника осталась", got)
	}
	if got := runGit(t, repo, "ls-files", "--others"); got != "" {
		t.Fatalf("неотслеженные файлы %q, want копия убрана", got)
	}
}

// Пока на экране не дифф рабочего дерева, пробел и x ничего не меняют: путь из
// чужого диффа указал бы на правку, которой на экране нет.
func TestReadOnlyViewsLeaveWorktreeAndIndex(t *testing.T) {
	views := map[string]func(t *testing.T) (string, tea.Model){
		"коммит из лога": func(t *testing.T) (string, tea.Model) {
			repo := editedTwiceRepo(t)
			model := resize(open(t, options(repo, layout.Focus())), 100, 20)
			model = openAsync(model, key('l'))
			model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			return repo, settle(model, cmd, 4)
		},
		"staged": func(t *testing.T) (string, tea.Model) {
			repo := editedTwiceRepo(t)
			opts := options(repo, layout.Focus())
			opts.Args = []string{"--staged"}
			return repo, resize(open(t, opts), 100, 20)
		},
	}

	const note = "press b to return to the working tree"
	for name, view := range views {
		t.Run(name+"/space", func(t *testing.T) {
			repo, model := view(t)

			model, cmd := model.Update(space)
			model = settle(model, cmd, 4)

			assertEditsKept(t, repo)
			if footer := footerOf(model); !strings.Contains(footer, note) {
				t.Fatalf("подвал = %q, want заметку", footer)
			}
		})

		t.Run(name+"/discard", func(t *testing.T) {
			repo, model := view(t)

			model, _ = model.Update(key('x'))
			footer := footerOf(model)
			if strings.Contains(footer, "Discard changes") || !strings.Contains(footer, note) {
				t.Fatalf("подвал = %q, want заметку без вопроса", footer)
			}
			answerYes(model)

			assertEditsKept(t, repo)
		})
	}
}

// editedTwiceRepo это репозиторий, где a.txt в HEAD «one», в индексе «two», а
// в рабочем дереве «three»: каждая версия видна по отдельности. Над HEAD есть
// ещё коммит «zero», поэтому в логе у HEAD есть дифф.
func editedTwiceRepo(t *testing.T) string {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "zero\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "add", "a.txt")
	write(t, repo, "a.txt", "three\n")

	return repo
}

func assertEditsKept(t *testing.T, repo string) {
	t.Helper()

	if got := runGit(t, repo, "show", ":a.txt"); got != "two" {
		t.Fatalf("в индексе %q, want two", got)
	}
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "three\n" {
		t.Fatalf("в рабочем дереве %q, want three", got)
	}
}

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// rewriteRepo это ветка из коммитов one, two, three, four, у каждого свой
// файл; открыта окном лога, курсор на four.
func rewriteRepo(t *testing.T) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	for _, name := range []string{"one", "two", "three", "four"} {
		write(t, repo, name+".txt", name+"\n")
		runGit(t, repo, "add", "-A")
		runGit(t, repo, "commit", "-q", "-m", name)
	}
	return repo, openAsync(resize(open(t, options(repo, layout.Focus())), 140, 30), key('l'))
}

// subjectsOf это темы коммитов ветки от новых к старым через пробел.
func subjectsOf(t *testing.T, repo string) string {
	t.Helper()
	return strings.Join(strings.Fields(runGit(t, repo, "log", "--format=%s")), " ")
}

// s сливает коммит в предыдущий, лог остаётся открытым (#219, случай 1).
func TestSquashFromLog(t *testing.T) {
	repo, model := rewriteRepo(t)
	model, _ = model.Update(key('j'))

	model = pressAndSettle(model, key('s'))

	if got := subjectsOf(t, repo); got != "four two one" {
		t.Fatalf("история %q, ожидалось three в two", got)
	}
	if body := screenText(model); !strings.Contains(body, "Log") || !strings.Contains(footerOf(model), "Squashed") {
		t.Fatalf("лог закрылся или нет итога:\n%s", body)
	}
}

// d спрашивает и убирает коммит только после y (случай 2).
func TestDropFromLog(t *testing.T) {
	repo, model := rewriteRepo(t)
	model, _ = model.Update(key('j'))

	model, _ = model.Update(key('d'))
	if body := screenText(model); !strings.Contains(body, "Drop") || !strings.Contains(body, "y yes") {
		t.Fatalf("d не спросил:\n%s", body)
	}
	model, _ = model.Update(key('n'))
	if got := subjectsOf(t, repo); got != "four three two one" {
		t.Fatalf("история %q изменилась без y", got)
	}

	model, _ = model.Update(key('d'))
	answerYes(model)
	if got := subjectsOf(t, repo); got != "four two one" {
		t.Fatalf("история %q, ожидалось без three", got)
	}
}

// ctrl+k поднимает коммит выше, курсор идёт за ним, и второй ctrl+k поднимает
// его ещё раз (случай 3).
func TestMoveCommitUp(t *testing.T) {
	repo, model := rewriteRepo(t)
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))

	model = pressAndSettle(model, tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if got := subjectsOf(t, repo); got != "four two three one" {
		t.Fatalf("история %q после первого переноса", got)
	}
	pressAndSettle(model, tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if got := subjectsOf(t, repo); got != "two four three one" {
		t.Fatalf("история %q после второго переноса", got)
	}
}

// r открывает окно коммита с сообщением коммита, enter меняет его (случай 4).
func TestRewordFromLog(t *testing.T) {
	repo, model := rewriteRepo(t)
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))

	model = pressAndSettle(model, key('r'))
	if body := screenText(model); !strings.Contains(body, "Reword") || !strings.Contains(body, "> two_") {
		t.Fatalf("r не открыл окно reword:\n%s", body)
	}
	enterKey(typeText(eraseText(model, 3), "second"))

	if got := subjectsOf(t, repo); got != "four three second one" {
		t.Fatalf("история %q, ожидалось новое сообщение", got)
	}
}

// Reword без правки не запускает rebase: таб в описании не превращается в
// пробелы, хэши на месте (#299, случай 2).
func TestRewordUnchangedKeepsCommit(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "--cleanup=verbatim", "-m", "one\n\n\tindented")
	write(t, repo, "b.txt", "two\n")
	commitAll(t, repo)
	head := revision(t, repo, "HEAD")
	model := openAsync(resize(open(t, options(repo, layout.Focus())), 140, 30), key('l'))
	model, _ = model.Update(key('j'))

	enterKey(pressAndSettle(model, key('r')))
	if got := revision(t, repo, "HEAD"); got != head {
		t.Fatalf("HEAD %s, reword без правки переписал историю", got)
	}
}

// Конфликт останавливает rebase, подвал говорит об этом (случай 5).
func TestRewriteConflictStops(t *testing.T) {
	repo := newRepo(t)
	for _, content := range []string{"one\n", "two\n", "three\n"} {
		write(t, repo, "a.txt", content)
		commitAll(t, repo)
	}
	model := openAsync(resize(open(t, options(repo, layout.Focus())), 140, 30), key('l'))
	model, _ = model.Update(key('j'))

	model = pressAndSettle(model, tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})

	if footer := footerOf(model); !strings.Contains(footer, "rebase stopped on a conflict") {
		t.Fatalf("подвал %q, ожидалась остановка", footer)
	}
}

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// z уносит правку в stash, Z показывает список, a применяет его обратно.
func TestStashFromApp(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	model := resize(open(t, options(repo, layout.Default())), 100, 20)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
	model = settle(model, cmd, 4)
	if got := readText(t, repo+"/a.txt"); got != "one\n" {
		t.Fatalf("после z файл %q, ожидался возврат к HEAD", got)
	}

	model, cmd = model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	model = settle(model, cmd, 4)
	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "stash@{0}") {
		t.Fatalf("список stash не открылся:\n%s", body)
	}

	model, cmd = model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	settle(model, cmd, 4)
	if got := readText(t, repo+"/a.txt"); got != "two\n" {
		t.Fatalf("после a файл %q, ожидалась правка", got)
	}
}

// Stash это коммит слияния HEAD и индекса, и git diff stash@{0}^! отдаёт для
// него комбинированный diff --cc одной сырой записью. Дифф берётся как у
// git stash show -p: правка вне индекса и правка в индексе идут обычными
// файлами.
func TestStashDiffShowsWorktreeAndIndexFiles(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	write(t, repo, "s.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	write(t, repo, "s.txt", "two\n")
	runGit(t, repo, "add", "s.txt")
	runGit(t, repo, "stash", "push", "-q")

	model := resize(open(t, options(repo, layout.Focus())), 100, 20)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	model = settle(model, cmd, 4)
	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 4)

	screen := screenOf(model)
	if got := openedFile(screen); got != "a.txt" {
		t.Fatalf("первый файл диффа stash %q, ожидался a.txt:\n%s", got, strings.Join(screen, "\n"))
	}
	if body := strings.Join(screen, "\n"); strings.Contains(body, "diff --cc") || !strings.Contains(body, "two") {
		t.Fatalf("дифф a.txt не обычный:\n%s", body)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	screen = screenOf(model)
	if got := openedFile(screen); got != "s.txt" {
		t.Fatalf("второй файл диффа stash %q, ожидался s.txt:\n%s", got, strings.Join(screen, "\n"))
	}
}

// Внешний git stash push сдвигает stash@{0}: открытый дифф держит свой stash
// по хэшу и при перечитывании не переходит на новый.
func TestStashDiffSurvivesExternalPush(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "stash", "push", "-q")

	model := resize(open(t, options(repo, layout.Focus())), 100, 20)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	model = settle(model, cmd, 4)
	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 4)

	write(t, repo, "a.txt", "three\n")
	runGit(t, repo, "stash", "push", "-q")
	model = pressReload(t, model)

	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "two") || strings.Contains(body, "three") {
		t.Fatalf("после перечитывания на экране не тот stash:\n%s", body)
	}
}

// openStaleStashWindow открывает окно Z с одним stash «two» и после этого
// кладёт в stash «three» из соседнего терминала: stash@{0} теперь другой
// stash, чем строка в окне. Рабочее дерево после обоих stash чистое, a.txt «one».
func openStaleStashWindow(t *testing.T) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "stash", "push", "-q")

	model := resize(open(t, options(repo, layout.Focus())), 100, 20)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	model = settle(model, cmd, 4)

	write(t, repo, "a.txt", "three\n")
	runGit(t, repo, "stash", "push", "-q")

	return repo, model
}

// Строка окна выбрана до внешнего git stash push. d, y не удаляют ни её, ни
// новый stash: пользователь выбирал строку, а stash@{0} теперь другой.
func TestStashDropRefusesAfterExternalPush(t *testing.T) {
	repo, model := openStaleStashWindow(t)
	before := runGit(t, repo, "stash", "list")

	model, _ = model.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	model = settle(model, cmd, 4)

	if after := runGit(t, repo, "stash", "list"); after != before {
		t.Fatalf("список stash изменился:\nдо:\n%s\nпосле:\n%s", before, after)
	}
	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "stash changed, reopen the list") {
		t.Fatalf("отказа нет в подвале:\n%s", body)
	}
}

// То же для a: применился бы новый stash «three», а выбран был «two».
func TestStashApplyRefusesAfterExternalPush(t *testing.T) {
	repo, model := openStaleStashWindow(t)

	model, cmd := model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	model = settle(model, cmd, 4)

	if got := readText(t, repo+"/a.txt"); got != "one\n" {
		t.Fatalf("после a файл %q, ожидался отказ без изменений", got)
	}
	if body := strings.Join(screenOf(model), "\n"); !strings.Contains(body, "stash changed, reopen the list") {
		t.Fatalf("отказа нет в подвале:\n%s", body)
	}
}

// openTwoStashes открывает окно Z с двумя stash: нижний stash@{1} хранит «two»,
// верхний stash@{0} «three». Рабочее дерево чистое, a.txt «one».
func openTwoStashes(t *testing.T) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "stash", "push", "-q")
	write(t, repo, "a.txt", "three\n")
	runGit(t, repo, "stash", "push", "-q")

	model := resize(open(t, options(repo, layout.Focus())), 100, 20)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	return repo, settle(model, cmd, 4)
}

// Курсор на втором stash: d, y удаляют его, а не первый.
func TestStashDropTakesStashUnderCursor(t *testing.T) {
	repo, model := openTwoStashes(t)

	model, _ = model.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	model, _ = model.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	settle(model, cmd, 4)

	if count := len(strings.Split(runGit(t, repo, "stash", "list"), "\n")); count != 1 {
		t.Fatalf("в списке %d stash, ожидался один", count)
	}
	if got := runGit(t, repo, "show", "stash@{0}:a.txt"); got != "three" {
		t.Fatalf("остался stash с %q, ожидался верхний с \"three\"", got)
	}
}

// Список после нового stash приходит под вопросом об удалении и сдвигает номера
// и курсор: вопрос и ответ остаются о записи, про которую спросили (#379,
// случай 1). Раньше y удалял только что сделанный stash.
func TestStashDropQuestionSurvivesNewStash(t *testing.T) {
	repo, model := openTwoStashes(t)
	write(t, repo, "a.txt", "four\n")

	model, _ = model.Update(key('n'))
	model = typeText(enterKey(model), "draft")
	model, push := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('d'))
	model = settle(model, push, 4)
	if body := screenText(model); !strings.Contains(body, "Drop stash@{2}?") {
		t.Fatalf("после нового stash вопрос не о stash@{2} «two»:\n%s", body)
	}

	answerYes(model)
	if got := runGit(t, repo, "stash", "list", "--format=%gs"); strings.Contains(got, "two") || !strings.Contains(got, "draft") {
		t.Fatalf("stash после y:\n%s\nожидались draft и three без two", got)
	}
}

// Запись, о которой спросили, удалили снаружи, а на её номер встала другая: y
// отказывает и ничего не удаляет (случай 2).
func TestStashDropRefusesWhenAskedEntryIsGone(t *testing.T) {
	repo, model := openTwoStashes(t)

	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('d'))
	runGit(t, repo, "stash", "drop", "-q", "stash@{1}")
	write(t, repo, "a.txt", "four\n")
	runGit(t, repo, "stash", "push", "-q")
	before := runGit(t, repo, "stash", "list")

	model = answerYes(model)
	if after := runGit(t, repo, "stash", "list"); after != before {
		t.Fatalf("список stash изменился:\nдо:\n%s\nпосле:\n%s", before, after)
	}
	if body := screenText(model); !strings.Contains(body, "stash changed, reopen the list") {
		t.Fatalf("отказа нет на экране:\n%s", body)
	}
}

// Курсор на втором stash: a применяет его, а не первый.
func TestStashApplyTakesStashUnderCursor(t *testing.T) {
	repo, model := openTwoStashes(t)

	model, _ = model.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	settle(model, cmd, 4)

	if got := readText(t, repo+"/a.txt"); got != "two\n" {
		t.Fatalf("после a файл %q, ожидалось содержимое нижнего stash \"two\\n\"", got)
	}
}

// enter в окне stash закрывает окно и открывает дифф выбранного stash.
func TestStashEnterOpensDiffAndClosesWindow(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "stash", "push", "-q")

	model := resize(open(t, options(repo, layout.Focus())), 100, 20)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	model = settle(model, cmd, 4)
	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 4)

	screen := screenOf(model)
	if body := strings.Join(screen, "\n"); strings.Contains(body, "Stash") {
		t.Fatalf("окно stash осталось открытым:\n%s", body)
	}
	if got := openedFile(screen); got != "a.txt" {
		t.Fatalf("открыт %q, ожидался дифф a.txt:\n%s", got, strings.Join(screen, "\n"))
	}
}

// Конфликт при apply: подвал называет строку CONFLICT, а не «Auto-merging
// a.txt», с которой git начинает вывод.
func TestStashApplyConflictShowsConflictInFooter(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "stash", "push", "-q")
	write(t, repo, "a.txt", "three\n")
	commitAll(t, repo)

	model := resize(open(t, options(repo, layout.Default())), 100, 20)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	model = settle(model, cmd, 4)
	model, cmd = model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	notice := noticeOf(settle(model, cmd, 4))

	if !strings.Contains(notice, "stash apply: CONFLICT (content): Merge conflict in a.txt") {
		t.Fatalf("тост = %q, ожидалась строка CONFLICT", notice)
	}
}

// d спрашивает, удаляет stash только y.
func TestStashDropNeedsYes(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "stash", "push", "-q")

	model := resize(open(t, options(repo, layout.Default())), 100, 20)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	model = settle(model, cmd, 4)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	model, cmd = model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	model = settle(model, cmd, 4)
	if out := runGit(t, repo, "stash", "list"); out == "" {
		t.Fatal("stash удалён без подтверждения")
	}

	// ответ «нет» оставил список открытым, вопрос задаётся в нём же
	model, _ = model.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	model, cmd = model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	settle(model, cmd, 4)
	if out := runGit(t, repo, "stash", "list"); out != "" {
		t.Fatalf("stash не удалён после y: %q", out)
	}
}

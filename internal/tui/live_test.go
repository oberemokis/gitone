package tui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/layout"
)

// editedRepo это репозиторий с закоммиченным a.txt «one» и правкой «two» в
// рабочей копии, открытый с таймером ticks.
func editedRepo(t *testing.T, ticks <-chan time.Time, root layout.Node) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	opts := options(repo, root)
	opts.Ticks = ticks
	return repo, resize(open(t, opts), 120, 20)
}

// Правка на диске и тик таймера: экран обновляется без r.
func TestTickReloadsWithoutKey(t *testing.T) {
	repo, model := editedRepo(t, oneTick(), layout.Focus())

	write(t, repo, "a.txt", "three\n")
	model = settle(model, model.Init(), 3)

	if body := screenText(model); !strings.Contains(body, "three") {
		t.Fatalf("после тика экран не обновился:\n%s", body)
	}
}

// Опрос тихий: пока git читает дифф, подвал тот же, загрузка не мигает.
func TestTickLeavesFooterAlone(t *testing.T) {
	_, model := editedRepo(t, oneTick(), layout.Focus())
	before := footerOf(model)

	model, _ = model.Update(model.Init()())

	if got := footerOf(model); got != before {
		t.Fatalf("подвал после тика %q, want %q", got, before)
	}
}

// git add из другого терминала: после тика gitone знает, что файл в индексе,
// и пробел убирает его оттуда, а не ставит второй раз.
func TestTickLearnsStageFromOutside(t *testing.T) {
	repo, model := editedRepo(t, oneTick(), layout.Focus())

	runGit(t, repo, "add", "a.txt")
	model = settle(model, model.Init(), 3)
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	settle(model, cmd, 4)

	if staged, err := git.StagedPaths(repo); err != nil || len(staged) != 0 {
		t.Fatalf("в индексе %q, ошибка %v, want пробел убрал файл", staged, err)
	}
}

// Терминал получил фокус: дифф перечитывается сразу, не дожидаясь тика.
func TestFocusReloads(t *testing.T) {
	repo, model := editedRepo(t, make(chan time.Time), layout.Focus())

	write(t, repo, "a.txt", "three\n")
	model, cmd := model.Update(tea.FocusMsg{})
	model = settle(model, cmd, 2)

	if body := screenText(model); !strings.Contains(body, "three") {
		t.Fatalf("после фокуса экран не обновился:\n%s", body)
	}
}

// Без автообновления фокус ничего не читает.
func TestFocusWithoutAutoReloadDoesNothing(t *testing.T) {
	_, model := editedRepo(t, nil, layout.Focus())

	if _, cmd := model.Update(tea.FocusMsg{}); cmd != nil {
		t.Fatal("фокус без таймера запустил чтение")
	}
}

// Опрос рабочего дерева, вернувшийся после открытия коммита из лога, дифф
// коммита не затирает.
func TestLatePollKeepsCommitDiff(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	// первый коммит лог не открывает, нужен второй
	write(t, repo, "b.txt", "bee\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	opts := options(repo, layout.Focus())
	opts.Ticks = oneTick()
	model := resize(open(t, opts), 120, 20)

	model, polling := model.Update(model.Init()())
	model = openAsync(model, key('l'))
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 2)
	model = settle(model, polling, 2)

	if body := screenText(model); strings.Contains(body, "two") || !strings.Contains(body, "bee") {
		t.Fatalf("want дифф коммита с b.txt, опрос со старыми аргументами его затёр:\n%s", body)
	}
}

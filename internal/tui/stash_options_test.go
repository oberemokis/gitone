package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// optionsRepo это репозиторий с коммитом, правкой a.txt в индексе и
// неотслеженным new.txt, открытый окном stash.
func optionsRepo(t *testing.T) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "add", "a.txt")
	write(t, repo, "new.txt", "new\n")

	model := resize(open(t, options(repo, layout.Focus())), 120, 24)
	return repo, openAsync(model, key('Z'))
}

// n открывает меню, первая строка спрашивает сообщение, enter сохраняет stash с
// ним: запись видна в окне, оно остаётся открытым (#236, случай 1).
func TestStashWithMessage(t *testing.T) {
	repo, model := optionsRepo(t)

	model, _ = model.Update(key('n'))
	body := screenText(model)
	for _, option := range []string{"New stash", "Stash tracked changes", "Stash including untracked files", "Stash tracked changes, keep the index"} {
		if !strings.Contains(body, option) {
			t.Fatalf("в меню нет %q:\n%s", option, body)
		}
	}

	model = enterKey(model)
	model = enterKey(typeText(model, "draft"))

	if got := runGit(t, repo, "stash", "list", "--format=%gs"); got != "On main: draft" {
		t.Fatalf("stash %q, ожидалось сообщение draft", got)
	}
	if body := screenText(model); !strings.Contains(body, "stash@{0}  On main: draft") {
		t.Fatalf("окно stash без новой записи:\n%s", body)
	}
	if _, err := os.Stat(filepath.Join(repo, "new.txt")); err != nil {
		t.Fatalf("неотслеженный файл ушёл без -u: %v", err)
	}
}

// Вторая строка уносит неотслеженные файлы, пустое сообщение оставляет
// сообщение git (случаи 2 и 4).
func TestStashIncludingUntracked(t *testing.T) {
	repo, model := optionsRepo(t)

	model, _ = model.Update(key('n'))
	model, _ = model.Update(key('j'))
	enterKey(enterKey(model))

	if _, err := os.Stat(filepath.Join(repo, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("new.txt остался в дереве: %v", err)
	}
	if got := runGit(t, repo, "stash", "list", "--format=%gs"); !strings.HasPrefix(got, "WIP on main") {
		t.Fatalf("stash %q, ожидалось сообщение git", got)
	}
}

// Третья строка оставляет индекс на месте (случай 3).
func TestStashKeepingIndex(t *testing.T) {
	repo, model := optionsRepo(t)

	model, _ = model.Update(key('n'))
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))
	enterKey(enterKey(model))

	if got := runGit(t, repo, "diff", "--cached", "--name-only"); got != "a.txt" {
		t.Fatalf("индекс %q, ожидался a.txt на месте", got)
	}
	if got := runGit(t, repo, "stash", "list"); got == "" {
		t.Fatal("stash не создан")
	}
}

// esc в меню возвращает к списку stash и ничего не сохраняет.
func TestStashMenuClosesBackToList(t *testing.T) {
	repo, model := optionsRepo(t)

	model, _ = model.Update(key('n'))
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if body := screenText(model); !strings.Contains(body, "No stash") || strings.Contains(body, "New stash") {
		t.Fatalf("esc не вернул к списку stash:\n%s", body)
	}
	if got := runGit(t, repo, "stash", "list"); got != "" {
		t.Fatalf("stash создан без выбора: %q", got)
	}
}

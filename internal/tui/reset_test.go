package tui_test

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// g в логе открывает меню режимов, soft переводит ветку на коммит под
// курсором, правка остаётся в индексе (#220, случай 1).
func TestResetSoftFromLog(t *testing.T) {
	repo, model := openLog(t, 3, 120, 30)
	target := runGit(t, repo, "rev-parse", "HEAD~1")
	model, _ = model.Update(key('j'))

	model, _ = model.Update(key('g'))
	if body := screenText(model); !strings.Contains(body, "Soft: keep changes staged") || !strings.Contains(body, "Hard: discard") {
		t.Fatalf("g не открыл меню режимов:\n%s", body)
	}
	model = enterKey(model)

	if got := runGit(t, repo, "rev-parse", "HEAD"); got != target {
		t.Fatalf("HEAD %s, ожидался %s", got, target)
	}
	if got := runGit(t, repo, "diff", "--cached", "--name-only"); got != "a.txt" {
		t.Fatalf("индекс %q, ожидалась правка a.txt", got)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Reset to "+target[:7]+" (soft)") {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}
}

// Hard спрашивает в окне лога: любая клавиша кроме y ничего не меняет, y
// стирает правки (случай 3).
func TestResetHardNeedsYes(t *testing.T) {
	repo, model := openLog(t, 2, 120, 30)
	write(t, repo, "a.txt", "dirty\n")
	before := runGit(t, repo, "rev-parse", "HEAD")
	model, _ = model.Update(key('j'))

	hardMenu := func(model tea.Model) tea.Model {
		model, _ = model.Update(key('g'))
		model, _ = model.Update(key('j'))
		model, _ = model.Update(key('j'))
		model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		return model
	}

	model = hardMenu(model)
	if body := screenText(model); !strings.Contains(body, "Hard reset to") {
		t.Fatalf("hard не спросил:\n%s", body)
	}
	model, _ = model.Update(key('n'))
	if got := runGit(t, repo, "rev-parse", "HEAD"); got != before {
		t.Fatal("hard reset выполнен без y")
	}

	answerYes(hardMenu(model))
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "0\n" {
		t.Fatalf("a.txt = %q, ожидалось содержимое коммита", got)
	}
}

// Reset под коммит, который уже на upstream, предупреждает: ветка разошлась
// с remote (случай 5).
func TestResetPublishedWarns(t *testing.T) {
	_, repo := remoteRepo(t)
	model := openAsync(resize(open(t, options(repo, layout.Focus())), 120, 30), key('l'))
	model, _ = model.Update(key('j'))

	model, _ = model.Update(key('g'))
	model = enterKey(model)

	if footer := footerOf(model); !strings.Contains(footer, "no longer matches its upstream") {
		t.Fatalf("подвал %q, ожидалось предупреждение", footer)
	}
}

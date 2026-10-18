package tui_test

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// C в логе feature копирует коммит, после переключения на main V в окне веток
// переносит его: коммит с тем же сообщением в main (#222, случай 1). Повторная
// вставка отказывает, коммит уже в ветке (случай 4).
func TestCherryPickCopyAndPaste(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "feature")
	write(t, repo, "f.txt", "feature\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "feature work")

	model := openAsync(resize(open(t, options(repo, layout.Focus())), 120, 24), key('l'))
	model, _ = model.Update(key('C'))
	if footer := footerOf(model); !strings.Contains(footer, "Copied") {
		t.Fatalf("подвал %q, ожидалось подтверждение копирования", footer)
	}

	runGit(t, repo, "switch", "-q", "main")
	model = openAsync(pressReload(t, closeWindow(model)), key('B'))
	model = pressAndSettle(model, key('V'))

	if got := readText(t, filepath.Join(repo, "f.txt")); got != "feature\n" {
		t.Fatalf("f.txt = %q, коммит не перенесён", got)
	}
	if got := runGit(t, repo, "log", "-1", "--format=%s"); got != "feature work" {
		t.Fatalf("сообщение %q, ожидалось сообщение коммита", got)
	}

	model = pressAndSettle(openAsync(model, key('l')), key('V'))
	if footer := footerOf(model); !strings.Contains(footer, "already in the branch") {
		t.Fatalf("подвал %q, ожидался отказ", footer)
	}
}

// closeWindow закрывает открытое окно по esc.
func closeWindow(model tea.Model) tea.Model {
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	return model
}

// Вставка без скопированного коммита говорит, как его скопировать (случай 2).
func TestCherryPickPasteWithoutCopy(t *testing.T) {
	_, model := openLog(t, 2, 120, 24)

	model = pressIdle(t, model, key('V'))

	if footer := footerOf(model); !strings.Contains(footer, "nothing to paste: copy a commit in the log with C") {
		t.Fatalf("подвал %q, ожидался отказ", footer)
	}
}

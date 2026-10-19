package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// u после switch спрашивает, что ветка переключится и файлы станут как в
// ней: откат checkout меняет дерево (#313, случай 1).
func TestUndoSwitchQuestionSaysFilesFollow(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "other")
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	model := resize(open(t, options(repo, layout.Focus())), 160, 20)

	model = pressAndSettle(model, key('u'))
	if footer := footerOf(model); !strings.Contains(footer, "files follow") || strings.Contains(footer, "files stay") {
		t.Fatalf("подвал %q, ожидался вопрос о переключении ветки", footer)
	}
}

// Остановка на конфликте советует клавиши gitone, а не терминал (случай 4).
func TestStoppedOnConflictNamesKeys(t *testing.T) {
	repo := newRepo(t)
	for _, content := range []string{"one\n", "two\n", "three\n"} {
		write(t, repo, "a.txt", content)
		commitAll(t, repo)
	}
	model := openAsync(resize(open(t, options(repo, layout.Focus())), 200, 30), key('l'))
	model, _ = model.Update(key('j'))

	model = pressAndSettle(model, tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if footer := footerOf(model); !strings.Contains(footer, "resolve it with v, then M continues") {
		t.Fatalf("подвал %q, ожидался совет с клавишами", footer)
	}
}

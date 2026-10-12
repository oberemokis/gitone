package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Ошибка перечитывания в подвале называет операцию git один раз и несёт текст
// самого git.
func TestReloadErrorNamesOperation(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	model, _ := open(t, repo, nil).Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	if err := os.RemoveAll(filepath.Join(repo, ".git")); err != nil {
		t.Fatal(err)
	}
	screen := screenOf(pressReload(t, model))

	footer := strings.TrimSpace(screen[len(screen)-1])
	if !strings.HasPrefix(footer, "diff: ") || strings.Count(footer, "diff: ") != 1 || !strings.Contains(footer, "not a git repository") {
		t.Fatalf("подвал = %q, want «diff: » один раз и текст git", footer)
	}
}

package tui_test

import (
	"fmt"
	"strings"
	"testing"

	"gitone/internal/layout"
)

// На низком экране без правок строка «нет изменений» не удлиняет экран:
// подвал остаётся на своей строке (#311, случай 2).
func TestEmptyFilesPaneKeepsScreenHeight(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	model := toggleFiles(resize(open(t, options(repo, layout.Default())), 60, 7))

	if rows := len(screenOf(model)); rows != 7 {
		t.Fatalf("экран из %d строк, ожидалось 7:\n%s", rows, screenText(model))
	}
}

// Файлов больше, чем строк панели: текущий после ] до последнего остаётся на
// экране (случай 3).
func TestFilesPaneScrollsToCurrent(t *testing.T) {
	repo := newRepo(t)
	for number := range 30 {
		write(t, repo, fmt.Sprintf("f%02d.txt", number), "one\n")
	}
	commitAll(t, repo)
	for number := range 30 {
		write(t, repo, fmt.Sprintf("f%02d.txt", number), "two\n")
	}
	model := toggleFiles(resize(open(t, options(repo, layout.Default())), 120, 15))

	for range 29 {
		model = pressAndSettle(model, key(']'))
	}
	if row := markedRow(model); !strings.Contains(row, "f29.txt") {
		t.Fatalf("текущий f29.txt не на экране, отмечено %q:\n%s", row, screenText(model))
	}
}

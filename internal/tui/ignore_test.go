package tui_test

import (
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/layout"
)

// i на неотслеженном файле дописывает его в .gitignore, файл пропадает из
// диффа, подвал говорит итог (#214, случаи 1 и 2).
func TestIgnoreUntrackedFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "debug.log", "x\n")

	model := resize(open(t, options(repo, layout.Focus())), 100, 20)
	model = pressAndSettle(model, key('i'))

	if got := readText(t, filepath.Join(repo, ".gitignore")); got != "/debug.log\n" {
		t.Fatalf(".gitignore = %q", got)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Added debug.log to .gitignore") {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}
	if got := openedFile(screenOf(model)); got == "debug.log" {
		t.Fatal("файл остался в диффе")
	}
}

// Отслеженный файл не игнорируется: подвал называет причину (случай 3).
func TestIgnoreTrackedFileRefused(t *testing.T) {
	_, model := threeFileRepo(t)

	model = pressAndSettle(model, key('i'))

	if footer := footerOf(model); !strings.Contains(footer, `"a.txt" is tracked`) {
		t.Fatalf("подвал %q, ожидался отказ", footer)
	}
}

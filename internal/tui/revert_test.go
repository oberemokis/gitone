package tui_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// t в логе отменяет коммит под курсором новым коммитом, подвал говорит итог;
// повтор отказывает с причиной (#221, случаи 1 и 4).
func TestRevertFromLog(t *testing.T) {
	repo, model := openLog(t, 2, 120, 30)
	target := runGit(t, repo, "rev-parse", "--short=7", "HEAD")

	model = pressAndSettle(model, key('t'))

	if got := readText(t, filepath.Join(repo, "a.txt")); got != "0\n" {
		t.Fatalf("a.txt = %q, ожидался обратный дифф", got)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Reverted "+target) {
		t.Fatalf("подвал %q, ожидался итог revert", footer)
	}

	model = openAsync(model, key('l'))
	model, _ = model.Update(key('j'))
	model = pressAndSettle(model, key('t'))
	if footer := footerOf(model); !strings.Contains(footer, "already reverted") {
		t.Fatalf("подвал %q, ожидался отказ", footer)
	}
}

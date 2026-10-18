package tui_test

import (
	"strings"
	"testing"
)

// n в логе спрашивает имя и создаёт ветку на коммите под курсором, она
// становится текущей (#223, случай 1); занятое имя git отвергает (случай 4).
func TestBranchFromLogCommit(t *testing.T) {
	repo, model := openLog(t, 2, 120, 24)
	first := runGit(t, repo, "rev-parse", "HEAD~1")
	model, _ = model.Update(key('j'))

	model, _ = model.Update(key('n'))
	if body := screenText(model); !strings.Contains(body, "New branch from "+first[:7]) {
		t.Fatalf("n не открыл поле имени:\n%s", body)
	}
	model = enterKey(typeText(model, "topic"))

	if got := runGit(t, repo, "branch", "--show-current"); got != "topic" {
		t.Fatalf("текущая %q, ожидалась topic", got)
	}
	if got := runGit(t, repo, "rev-parse", "HEAD"); got != first {
		t.Fatalf("HEAD %s, ожидался коммит из лога %s", got, first)
	}

	model = openAsync(model, key('l'))
	model, _ = model.Update(key('n'))
	model = enterKey(typeText(model, "main"))
	if footer := footerOf(model); !strings.Contains(footer, "already exists") {
		t.Fatalf("подвал %q, ожидался отказ", footer)
	}
}

// space в логе отсоединяет HEAD на коммите, подвал предупреждает (случай 2).
func TestCheckoutLogCommit(t *testing.T) {
	repo, model := openLog(t, 2, 120, 24)
	first := runGit(t, repo, "rev-parse", "HEAD~1")
	model, _ = model.Update(key('j'))

	model = pressAndSettle(model, space)

	if got := runGit(t, repo, "rev-parse", "HEAD"); got != first {
		t.Fatalf("HEAD %s, ожидался %s", got, first)
	}
	if footer := footerOf(model); !strings.Contains(footer, "HEAD is detached at "+first[:7]) {
		t.Fatalf("подвал %q, ожидалось предупреждение", footer)
	}
}

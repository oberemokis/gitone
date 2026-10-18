package tui_test

import (
	"strings"
	"testing"

	"gitone/internal/layout"
)

// u в окне веток показывает удалённые ветки меню, выбор привязывает ветку к
// выбранной, окно веток остаётся, подвал говорит итог (#230, случаи 1 и 3).
func TestSetUpstreamFromBranchWindow(t *testing.T) {
	_, repo := remoteRepo(t)
	runGit(t, repo, "push", "-q", "origin", "main:other")
	runGit(t, repo, "switch", "-q", "-c", "topic")

	model := openAsync(resize(open(t, options(repo, layout.Focus())), 120, 24), key('B'))
	model = pressAndSettle(model, key('u'))
	body := screenText(model)
	if !strings.Contains(body, "Upstream for topic") || !strings.Contains(body, "origin/other") {
		t.Fatalf("u не показал удалённые ветки:\n%s", body)
	}

	model, _ = model.Update(key('j'))
	model = enterKey(model)

	if got := runGit(t, repo, "rev-parse", "--abbrev-ref", "topic@{upstream}"); got != "origin/other" {
		t.Fatalf("upstream %q, ожидался origin/other", got)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Upstream of topic is now origin/other") {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}
	if body := screenText(model); !strings.Contains(body, "Branches") {
		t.Fatalf("окно веток закрылось:\n%s", body)
	}
}

// Без удалённых веток подвал подсказывает fetch.
func TestSetUpstreamWithoutRemoteBranches(t *testing.T) {
	_, model := branchRepo(t)

	model = pressAndSettle(model, key('u'))

	if footer := footerOf(model); !strings.Contains(footer, "no remote branches, fetch them with F first") {
		t.Fatalf("подвал %q, ожидалась подсказка", footer)
	}
}

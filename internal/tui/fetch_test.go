package tui_test

import (
	"strings"
	"testing"
)

// F забирает новый коммит с origin в origin/main, локальная ветка стоит на
// месте, подвал говорит «Fetched» (#232, случай 1).
func TestFetchUpdatesRemoteBranchOnly(t *testing.T) {
	origin, repo := remoteRepo(t)
	model := openRemote(t, repo)
	before := revision(t, repo, "HEAD")
	publishPeer(t, origin, "b.txt", "two\n")

	model = pressAndSettle(model, key('F'))

	if got := revision(t, repo, "origin/main"); got != revision(t, origin, "main") {
		t.Errorf("origin/main %s, ожидался main с origin %s", got, revision(t, origin, "main"))
	}
	if got := revision(t, repo, "HEAD"); got != before {
		t.Errorf("HEAD сдвинулся: %s, был %s", got, before)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Fetched") {
		t.Errorf("подвал %q, ожидалось «Fetched»", footer)
	}
}

// Недоступный remote и репозиторий без remote дают ошибку в подвале (случаи 2
// и 3).
func TestFetchErrorsShowInFooter(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	model := pressAndSettle(openRemote(t, repo), key('F'))
	if footer := footerOf(model); !strings.Contains(footer, "no remote is configured") {
		t.Errorf("без remote подвал %q, ожидался отказ", footer)
	}

	runGit(t, repo, "remote", "add", "origin", t.TempDir()+"/missing.git")
	model = pressAndSettle(model, key('F'))
	if footer := footerOf(model); !strings.Contains(footer, "fetch: ") || strings.Contains(footer, "Fetched") {
		t.Errorf("с недоступным remote подвал %q, ожидалась ошибка git", footer)
	}
}

// Пока идёт fetch, правки репозитория и вторая сетевая операция отказывают
// так же, как при pull (случай 4).
func TestActionsRefuseWhileFetchRuns(t *testing.T) {
	_, repo := remoteRepo(t)
	model, _ := openRemote(t, repo).Update(key('F'))
	if footer := footerOf(model); !strings.Contains(footer, "Fetching…") {
		t.Fatalf("подвал %q, ожидался спиннер «Fetching…»", footer)
	}

	for _, pressed := range []rune{'x', 'c', 'p', 'P', 'F'} {
		model = pressIdle(t, model, key(pressed))

		if footer := footerOf(model); !strings.Contains(footer, "fetch is running") {
			t.Errorf("после %q подвал %q, ожидалось «fetch is running»", pressed, footer)
		}
	}
}

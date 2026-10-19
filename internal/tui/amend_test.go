package tui_test

import (
	"strings"
	"testing"

	"gitone/internal/layout"
)

// A открывает окно amend с сообщением последнего коммита, правка темы и enter
// переписывают коммит вместе с индексом (#218, случаи 1 и 2).
func TestAmendFromApp(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "fix tpyo")
	write(t, repo, "b.txt", "two\n")
	runGit(t, repo, "add", "b.txt")

	model := resize(open(t, options(repo, layout.Focus())), 100, 24)
	model = pressAndSettle(model, key('A'))
	if body := screenText(model); !strings.Contains(body, "Amend last commit") || !strings.Contains(body, "> fix tpyo_") {
		t.Fatalf("A не открыл окно amend с прежней темой:\n%s", body)
	}

	model = enterKey(typeText(eraseText(model, len("tpyo")), "typo"))

	if got := runGit(t, repo, "log", "--format=%s"); got != "fix typo" {
		t.Fatalf("история %q, ожидался один исправленный коммит", got)
	}
	if staged := runGit(t, repo, "diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("в индексе осталось %q", staged)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Last commit amended") {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}
}

// enter без правки не меняет сообщение: первый абзац из двух строк и таб в
// описании остаются как были; новая тема меняет только тему (#299, случаи 1
// и 3).
func TestAmendUnchangedKeepsMessage(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "-A")
	message := "Fix parser\nAlso handle CRLF\n\n\tindented"
	runGit(t, repo, "commit", "-q", "--cleanup=verbatim", "-m", message)
	model := resize(open(t, options(repo, layout.Focus())), 100, 24)

	model = enterKey(pressAndSettle(model, key('A')))
	if got := runGit(t, repo, "log", "-1", "--format=%B"); got != message {
		t.Fatalf("сообщение %q, ожидалось прежнее %q", got, message)
	}

	// при правке таб уходит пробелами: поле описания хранит то, что рисует
	enterKey(typeText(pressAndSettle(model, key('A')), "!"))
	if got := runGit(t, repo, "log", "-1", "--format=%B"); !strings.HasPrefix(got, "Fix parser!\n\nAlso handle CRLF\n\n") {
		t.Fatalf("сообщение %q, ожидалась новая тема с прежним описанием", got)
	}
}

// Без коммитов подвал называет причину, окно не открывается (случай 3).
func TestAmendWithoutCommits(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")

	model := pressAndSettle(resize(open(t, options(repo, layout.Focus())), 100, 24), key('A'))

	if footer := footerOf(model); !strings.Contains(footer, "no commits yet") {
		t.Fatalf("подвал %q, ожидался отказ", footer)
	}
}

// Amend коммита, который уже на upstream, предупреждает (случай 4).
func TestAmendPublishedWarns(t *testing.T) {
	_, repo := remoteRepo(t)

	model := pressAndSettle(openRemote(t, repo), key('A'))
	model = enterKey(model)

	if footer := footerOf(model); !strings.Contains(footer, "a plain push will be rejected") {
		t.Fatalf("подвал %q, ожидалось предупреждение", footer)
	}
}

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// remotesRepo это репозиторий без upstream с remote fork, у которого адреса
// fetch и push разные, открытый окном remote.
func remotesRepo(t *testing.T) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "remote", "add", "fork", "https://example.com/fork.git")
	runGit(t, repo, "remote", "set-url", "--push", "fork", "ssh://example.com/fork.git")

	model := resize(open(t, options(repo, layout.Focus())), 120, 24)
	return repo, openAsync(model, key('R'))
}

// R открывает окно: remote виден с адресами fetch и push (#233, случай 1).
func TestRemotesList(t *testing.T) {
	_, model := remotesRepo(t)

	body := screenText(model)
	if !strings.Contains(body, "https://example.com/fork.git") || !strings.Contains(body, "ssh://example.com/fork.git") {
		t.Fatalf("адреса fork не видны:\n%s", body)
	}
	if strings.Contains(body, "fork upstream") {
		t.Fatalf("fork помечен upstream без upstream у ветки:\n%s", body)
	}
}

// n спрашивает имя, потом адрес, и remote появляется в git; занятое имя git
// отвергает, подвал говорит причину (случаи 2 и 3).
func TestRemotesNew(t *testing.T) {
	repo, model := remotesRepo(t)

	model, _ = model.Update(key('n'))
	if body := screenText(model); !strings.Contains(body, "New remote") {
		t.Fatalf("n не открыл поле имени:\n%s", body)
	}
	model = enterKey(typeText(model, "mirror"))
	if body := screenText(model); !strings.Contains(body, "URL for mirror") {
		t.Fatalf("enter не спросил адрес:\n%s", body)
	}
	model = enterKey(typeText(model, "https://example.com/mirror.git"))

	if got := runGit(t, repo, "remote", "get-url", "mirror"); got != "https://example.com/mirror.git" {
		t.Fatalf("адрес mirror %q", got)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Remote mirror added") {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}

	model, _ = model.Update(key('n'))
	model = enterKey(typeText(model, "fork"))
	model = enterKey(typeText(model, "https://example.com/other.git"))
	if footer := footerOf(model); !strings.Contains(footer, "already exists") {
		t.Fatalf("подвал %q, ожидался отказ git", footer)
	}
}

// d на remote с upstream текущей ветки предупреждает в вопросе, удаляет только
// после y (случай 4).
func TestRemotesDeleteUpstream(t *testing.T) {
	_, repo := remoteRepo(t)
	model := openAsync(openRemote(t, repo), key('R'))
	if body := screenText(model); !strings.Contains(body, "origin upstream") {
		t.Fatalf("origin не помечен upstream:\n%s", body)
	}

	model, _ = model.Update(key('d'))
	if body := screenText(model); !strings.Contains(body, "loses its upstream") {
		t.Fatalf("вопрос без предупреждения об upstream:\n%s", body)
	}
	model = pressAndSettle(model, key('n'))
	if got := runGit(t, repo, "remote"); got != "origin" {
		t.Fatalf("remote %q, удалён без y", got)
	}

	model, _ = model.Update(key('d'))
	model = answerYes(model)
	if got := runGit(t, repo, "remote"); got != "" {
		t.Fatalf("remote %q, ожидалось удаление", got)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Remote origin removed") {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}
}

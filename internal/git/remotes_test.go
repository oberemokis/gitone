package git_test

import (
	"strings"
	"testing"
)

// Список отдаёт адреса fetch и push и помечает remote с upstream текущей
// ветки; добавление и удаление видны в нём, занятое имя git отвергает (#233,
// случаи 1–4).
func TestRemotes(t *testing.T) {
	origin, local := remotePair(t)
	runGit(t, local, "remote", "set-url", "--push", "origin", "ssh://push.example/repo.git")

	if err := open(t, local).AddRemote("fork", "https://example.com/fork.git"); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	if err := open(t, local).AddRemote("fork", "https://example.com/other.git"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("AddRemote занятого = %v, ожидался отказ", err)
	}

	remotes, err := open(t, local).Remotes()
	if err != nil || len(remotes) != 2 {
		t.Fatalf("Remotes = %+v, %v", remotes, err)
	}
	if got := remotes[0]; got.Name != "fork" || got.Fetch != "https://example.com/fork.git" || got.Push != got.Fetch || got.Upstream {
		t.Fatalf("fork = %+v", got)
	}
	if got := remotes[1]; got.Name != "origin" || got.Fetch != origin || got.Push != "ssh://push.example/repo.git" || !got.Upstream {
		t.Fatalf("origin = %+v, ожидались два адреса и upstream", got)
	}

	if err := open(t, local).RemoveRemote("origin"); err != nil {
		t.Fatalf("RemoveRemote: %v", err)
	}
	if remotes, _ := open(t, local).Remotes(); len(remotes) != 1 || remotes[0].Name != "fork" {
		t.Fatalf("после удаления %+v, ожидался один fork", remotes)
	}
}

// Репозиторий без remote даёт пустой список, а не ошибку.
func TestRemotesEmpty(t *testing.T) {
	repo := newRepo(t)

	if remotes, err := open(t, repo).Remotes(); err != nil || len(remotes) != 0 {
		t.Fatalf("Remotes = %+v, %v", remotes, err)
	}
}

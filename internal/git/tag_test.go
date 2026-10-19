package git_test

import (
	"context"
	"strings"
	"testing"

	"gitone/internal/git"
)

// Аннотированный и лёгкий теги видны в списке со своим коммитом, удаление
// убирает тег, занятое имя git отвергает (#238, случаи 1, 2, 3, 5).
func TestTags(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	head := revision(t, repo, "HEAD")

	if err := open(t, repo).CreateTag("v1", "first release"); err != nil {
		t.Fatalf("CreateTag annotated: %v", err)
	}
	if err := open(t, repo).CreateTag("light", ""); err != nil {
		t.Fatalf("CreateTag light: %v", err)
	}
	if err := open(t, repo).CreateTag("v1", ""); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("CreateTag занятого = %v, ожидался отказ", err)
	}

	tags, err := open(t, repo).Tags()
	if err != nil || len(tags) != 2 {
		t.Fatalf("Tags = %+v, %v", tags, err)
	}
	byName := map[string]git.Tag{}
	for _, tag := range tags {
		byName[tag.Name] = tag
	}
	// HEAD это первый коммит: у тегов на нём нет родителя (#297)
	if tag := byName["v1"]; !tag.Annotated || tag.Commit != head || tag.Subject != "first release" || !tag.Root {
		t.Fatalf("v1 = %+v, ожидался аннотированный на первом коммите", tag)
	}
	if tag := byName["light"]; tag.Annotated || tag.Commit != head || !tag.Root {
		t.Fatalf("light = %+v, ожидался лёгкий на первом коммите", tag)
	}

	if err := open(t, repo).DeleteTag("light"); err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}
	if tags, _ := open(t, repo).Tags(); len(tags) != 1 {
		t.Fatalf("после удаления тегов %d, ожидался 1", len(tags))
	}
}

// Тег уходит на origin (случай 4).
func TestPushTag(t *testing.T) {
	origin, local := remotePair(t)
	runGit(t, local, "tag", "v1")

	if err := open(t, local).PushTag(context.Background(), "v1"); err != nil {
		t.Fatalf("PushTag: %v", err)
	}
	if got := strings.TrimSpace(runGit(t, origin, "tag", "--list")); got != "v1" {
		t.Fatalf("теги на origin %q, ожидался v1", got)
	}
}

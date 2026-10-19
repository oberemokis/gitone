package git_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/git"
)

// Ошибка экспортируемой функции начинается с имени операции git, и имя
// встречается ровно один раз: обёртка стоит только на границе пакета.
func TestErrorsNameOperationOnce(t *testing.T) {
	// каталог вне репозитория: TMPDIR внутри дерева git иначе нашёл бы его
	outside := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(outside))

	tests := []struct {
		name   string
		prefix string
		call   func(repo string) error
	}{
		{name: "коммит с пустым индексом", prefix: "commit: ", call: func(repo string) error {
			return open(t, repo).Commit("пусто", "")
		}},
		{name: "переключение на несуществующую ветку", prefix: "switch: ", call: func(repo string) error {
			return open(t, repo).Switch("ghost")
		}},
		{name: "применение несуществующего stash", prefix: "stash apply: ", call: func(repo string) error {
			return open(t, repo).StashApply(git.Stash{Ref: "stash@{5}", Hash: "0000000000000000000000000000000000000000"})
		}},
		{name: "открытие вне репозитория", prefix: "rev-parse: ", call: func(string) error {
			_, err := git.Open(outside)
			return err
		}},
		{name: "pull ветки без upstream", prefix: "pull: ", call: func(repo string) error {
			_, err := open(t, repo).Pull(context.Background())
			return err
		}},
		{name: "push без origin", prefix: "push: ", call: func(repo string) error {
			_, err := open(t, repo).Push(context.Background())
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			write(t, repo, "a.txt", "one\n")
			commitAll(t, repo)

			err := tc.call(repo)
			if err == nil {
				t.Fatalf("%s: ошибки нет, want ошибка с %q", tc.name, tc.prefix)
			}
			if got := err.Error(); !strings.HasPrefix(got, tc.prefix) || strings.Count(got, tc.prefix) != 1 {
				t.Errorf("%s = %q, want ровно один префикс %q в начале", tc.name, got, tc.prefix)
			}
		})
	}
}

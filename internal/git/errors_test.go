package git_test

import (
	"strings"
	"testing"

	"gitone/internal/git"
)

// Ошибка экспортируемой функции начинается с имени операции git, и имя
// встречается ровно один раз: обёртка стоит только на границе пакета.
func TestErrorsNameOperationOnce(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		call   func(repo string) error
	}{
		{name: "коммит с пустым индексом", prefix: "commit: ", call: func(repo string) error {
			return git.Commit(repo, "пусто")
		}},
		{name: "переключение на несуществующую ветку", prefix: "switch: ", call: func(repo string) error {
			return git.Switch(repo, "ghost")
		}},
		{name: "применение несуществующего stash", prefix: "stash apply: ", call: func(repo string) error {
			return git.StashApply(repo, "stash@{5}")
		}},
		{name: "дифф вне репозитория", prefix: "diff: ", call: func(string) error {
			_, err := git.Diff(t.TempDir(), nil)
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

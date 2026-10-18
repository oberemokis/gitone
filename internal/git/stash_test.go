package git_test

import (
	"errors"
	"path/filepath"
	"testing"

	"gitone/internal/git"
	"gitone/internal/i18n"
)

// Stash уносит правку и возвращает файл к HEAD. Apply возвращает правку и
// оставляет stash, Drop удаляет его.
func TestStashPushApplyDrop(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	if err := git.StashPush(repo); err != nil {
		t.Fatalf("StashPush: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "one\n" {
		t.Fatalf("после stash файл %q, ожидался возврат к HEAD", got)
	}
	stashes, err := git.Stashes(repo)
	if err != nil || len(stashes) != 1 {
		t.Fatalf("stash %v, ошибка %v; ожидался один", stashes, err)
	}

	if err := git.StashApply(repo, stashes[0]); err != nil {
		t.Fatalf("StashApply: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "two\n" {
		t.Fatalf("после apply файл %q, ожидалась правка", got)
	}
	if again, _ := git.Stashes(repo); len(again) != 1 {
		t.Fatalf("apply удалил stash: %v", again)
	}

	if err := git.StashDrop(repo, stashes[0]); err != nil {
		t.Fatalf("StashDrop: %v", err)
	}
	if left, _ := git.Stashes(repo); len(left) != 0 {
		t.Fatalf("drop оставил stash: %v", left)
	}
}

// Отказ apply называет причину одной строкой: при конфликте строку CONFLICT, а
// не «Auto-merging a.txt», которой git начинает вывод; без конфликта первую.
func TestStashApplyErrorNamesReason(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(t *testing.T, repo string)
		want    string
	}{
		{name: "конфликт", want: "stash apply: CONFLICT (content): Merge conflict in a.txt", prepare: func(t *testing.T, repo string) {
			write(t, repo, "a.txt", "three\n")
			commitAll(t, repo)
		}},
		{name: "правка мешает", want: "stash apply: error: Your local changes to the following files would be overwritten by merge:", prepare: func(t *testing.T, repo string) {
			write(t, repo, "a.txt", "three\n")
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			write(t, repo, "a.txt", "one\n")
			commitAll(t, repo)
			write(t, repo, "a.txt", "two\n")
			if err := git.StashPush(repo); err != nil {
				t.Fatalf("StashPush: %v", err)
			}
			tc.prepare(t, repo)
			stashes, err := git.Stashes(repo)
			if err != nil || len(stashes) != 1 {
				t.Fatalf("stash %v, ошибка %v; ожидался один", stashes, err)
			}

			err = git.StashApply(repo, stashes[0])

			if err == nil || err.Error() != tc.want {
				t.Fatalf("StashApply = %v, want %q", err, tc.want)
			}
		})
	}
}

// Pop без конфликта возвращает правку и удаляет запись; с конфликтом говорит,
// что запись осталась; правка в дереве, которой мешает pop, оставляет stash
// нетронутым (#235).
func TestStashPop(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(t *testing.T, repo string)
		want    string
		left    int
	}{
		{name: "без конфликта", prepare: func(*testing.T, string) {}, left: 0},
		{name: "конфликт", want: "stash pop: conflict: fix the files, the stash entry is kept", left: 1, prepare: func(t *testing.T, repo string) {
			write(t, repo, "a.txt", "three\n")
			commitAll(t, repo)
		}},
		{name: "правка мешает", want: "stash pop: error: Your local changes to the following files would be overwritten by merge:", left: 1, prepare: func(t *testing.T, repo string) {
			write(t, repo, "a.txt", "three\n")
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			write(t, repo, "a.txt", "one\n")
			commitAll(t, repo)
			write(t, repo, "a.txt", "two\n")
			if err := git.StashPush(repo); err != nil {
				t.Fatalf("StashPush: %v", err)
			}
			tc.prepare(t, repo)
			stashes, _ := git.Stashes(repo)

			err := git.StashPop(repo, stashes[0])

			if got := ""; err != nil {
				got = err.Error()
				if got != tc.want {
					t.Fatalf("StashPop = %q, want %q", got, tc.want)
				}
			} else if tc.want != "" {
				t.Fatalf("StashPop = nil, want %q", tc.want)
			}
			if after, _ := git.Stashes(repo); len(after) != tc.left {
				t.Fatalf("stash осталось %d, ожидалось %d", len(after), tc.left)
			}
			if tc.left == 0 && readFile(t, filepath.Join(repo, "a.txt")) != "two\n" {
				t.Fatal("pop не вернул правку")
			}
		})
	}
}

func TestStashPushWithNothingFails(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	if err := git.StashPush(repo); err == nil {
		t.Fatal("stash без правок прошёл")
	}
}

// Сдвинутый указатель подмодуля git stash push игнорирует: отвечает «No local
// changes to save» с кодом 0 и stash не создаёт. Для проверки «нечего
// сохранять» подмодуль тоже не правка.
func TestStashPushIgnoresSubmodulePointer(t *testing.T) {
	upstream := newRepo(t)
	write(t, upstream, "lib.txt", "one\n")
	commitAll(t, upstream)
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", upstream, "sub")
	runGit(t, repo, "commit", "-q", "-m", "add submodule")
	sub := filepath.Join(repo, "sub")
	write(t, sub, "lib.txt", "two\n")
	runGit(t, sub, "commit", "-q", "-a", "-m", "next")

	err := git.StashPush(repo)

	var refusal *i18n.Error
	if !errors.As(err, &refusal) || refusal.Key != "git.nothingToStash" {
		t.Fatalf("StashPush error = %v, ожидался отказ git.nothingToStash", err)
	}
	if listed := runGit(t, repo, "stash", "list"); listed != "" {
		t.Fatalf("stash создан: %q", listed)
	}
}

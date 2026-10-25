package git_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

	if err := open(t, repo).StashPush(git.StashOptions{}); err != nil {
		t.Fatalf("StashPush: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "one\n" {
		t.Fatalf("после stash файл %q, ожидался возврат к HEAD", got)
	}
	stashes, err := open(t, repo).Stashes()
	if err != nil || len(stashes) != 1 {
		t.Fatalf("stash %v, ошибка %v; ожидался один", stashes, err)
	}

	if err := open(t, repo).StashApply(stashes[0]); err != nil {
		t.Fatalf("StashApply: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "two\n" {
		t.Fatalf("после apply файл %q, ожидалась правка", got)
	}
	if again, _ := open(t, repo).Stashes(); len(again) != 1 {
		t.Fatalf("apply удалил stash: %v", again)
	}

	if err := open(t, repo).StashDrop(stashes[0]); err != nil {
		t.Fatalf("StashDrop: %v", err)
	}
	if left, _ := open(t, repo).Stashes(); len(left) != 0 {
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
			if err := open(t, repo).StashPush(git.StashOptions{}); err != nil {
				t.Fatalf("StashPush: %v", err)
			}
			tc.prepare(t, repo)
			stashes, err := open(t, repo).Stashes()
			if err != nil || len(stashes) != 1 {
				t.Fatalf("stash %v, ошибка %v; ожидался один", stashes, err)
			}

			err = open(t, repo).StashApply(stashes[0])

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
			if err := open(t, repo).StashPush(git.StashOptions{}); err != nil {
				t.Fatalf("StashPush: %v", err)
			}
			tc.prepare(t, repo)
			stashes, _ := open(t, repo).Stashes()

			err := open(t, repo).StashPop(stashes[0])

			if got := ""; err != nil {
				got = err.Error()
				if got != tc.want {
					t.Fatalf("StashPop = %q, want %q", got, tc.want)
				}
			} else if tc.want != "" {
				t.Fatalf("StashPop = nil, want %q", tc.want)
			}
			if after, _ := open(t, repo).Stashes(); len(after) != tc.left {
				t.Fatalf("stash осталось %d, ожидалось %d", len(after), tc.left)
			}
			if tc.left == 0 && readFile(t, filepath.Join(repo, "a.txt")) != "two\n" {
				t.Fatal("pop не вернул правку")
			}
		})
	}
}

// Pop посреди слияния с конфликтом отказывает до git: запись цела, а отказ не
// говорит, что конфликт дал сам pop (#388, случай 2).
func TestStashPopDuringConflict(t *testing.T) {
	repo := conflictBranches(t)
	write(t, repo, "c.txt", "c\n")
	commitAll(t, repo)
	write(t, repo, "c.txt", "dirty\n")
	if err := open(t, repo).StashPush(git.StashOptions{}); err != nil {
		t.Fatalf("StashPush: %v", err)
	}
	stopsOnConflict(t, repo, "merge", "topic")
	stashes, _ := open(t, repo).Stashes()

	refusedWith(t, open(t, repo).StashPop(stashes[0]), "git.stashUnmerged")
	if after, _ := open(t, repo).Stashes(); len(after) != 1 {
		t.Fatalf("stash осталось %d, ожидалась одна запись", len(after))
	}
	if got := readFile(t, filepath.Join(repo, "c.txt")); got != "c\n" {
		t.Fatalf("c.txt %q, pop не должен был ничего применить", got)
	}
}

// Под немецким языком конфликт apply узнаётся: git отвечает по-английски, и
// headline находит строку CONFLICT (#388, случай 3).
func TestStashApplyConflictInOtherLanguage(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	if err := open(t, repo).StashPush(git.StashOptions{}); err != nil {
		t.Fatalf("StashPush: %v", err)
	}
	write(t, repo, "a.txt", "three\n")
	commitAll(t, repo)
	stashes, _ := open(t, repo).Stashes()
	// newRepo ставит LC_ALL=C, тест её переопределяет; без немецкой локали в
	// системе git говорит по-английски, и тест проходит без проверки
	t.Setenv("LANGUAGE", "de")
	t.Setenv("LC_ALL", "de_DE.UTF-8")

	err := open(t, repo).StashApply(stashes[0])
	if want := "stash apply: CONFLICT (content): Merge conflict in a.txt"; err == nil || err.Error() != want {
		t.Fatalf("StashApply = %v, want %q", err, want)
	}
}

// Переименование средней записи меняет только её сообщение: коммит и порядок
// записей прежние. Пустое сообщение отвергается (#237).
func TestStashRenameKeepsOrder(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	for _, message := range []string{"first", "second", "third"} {
		write(t, repo, "a.txt", message+"\n")
		if err := open(t, repo).StashPush(git.StashOptions{Message: message}); err != nil {
			t.Fatalf("StashPush(%s): %v", message, err)
		}
	}
	before, _ := open(t, repo).Stashes()

	if err := open(t, repo).StashRename(before[1], "renamed"); err != nil {
		t.Fatalf("StashRename: %v", err)
	}

	after, _ := open(t, repo).Stashes()
	if len(after) != 3 {
		t.Fatalf("stash %+v, ожидалось три записи", after)
	}
	for index, want := range []string{"On main: third", "renamed", "On main: first"} {
		if after[index].Subject != want || after[index].Hash != before[index].Hash {
			t.Errorf("запись %d = %+v, ожидалось %q с хэшем %s", index, after[index], want, before[index].Hash)
		}
	}

	var refusal *i18n.Error
	if err := open(t, repo).StashRename(after[0], "  "); !errors.As(err, &refusal) || refusal.Key != "git.emptyStashMessage" {
		t.Fatalf("StashRename с пустым сообщением = %v, ожидался отказ", err)
	}
	if err := open(t, repo).StashRename(before[1], "stale"); !errors.As(err, &refusal) || refusal.Key != "git.stashChanged" {
		t.Fatalf("StashRename устаревшей записи = %v, ожидался отказ git.stashChanged", err)
	}
}

// Сообщение видно в списке, -u уносит новые файлы, --keep-index оставляет
// индекс на месте, без сообщения git ставит своё (#236).
func TestStashPushOptions(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	write(t, repo, "new.txt", "new\n")
	if err := open(t, repo).StashPush(git.StashOptions{}); err == nil {
		t.Fatal("stash без отслеженных правок прошёл без Untracked")
	}
	if err := open(t, repo).StashPush(git.StashOptions{Message: "черновик", Untracked: true}); err != nil {
		t.Fatalf("StashPush с untracked: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "new.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new.txt остался в дереве: %v", err)
	}

	write(t, repo, "a.txt", "staged\n")
	runGit(t, repo, "add", "a.txt")
	if err := open(t, repo).StashPush(git.StashOptions{KeepIndex: true}); err != nil {
		t.Fatalf("StashPush с keep-index: %v", err)
	}
	if staged := runGit(t, repo, "diff", "--cached", "--name-only"); staged != "a.txt\n" {
		t.Fatalf("индекс %q, ожидался a.txt на месте", staged)
	}

	stashes, _ := open(t, repo).Stashes()
	if len(stashes) != 2 || !strings.HasPrefix(stashes[0].Subject, "WIP on main") || stashes[1].Subject != "On main: черновик" {
		t.Fatalf("stash %+v, ожидались WIP и «черновик»", stashes)
	}
}

func TestStashPushWithNothingFails(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	if err := open(t, repo).StashPush(git.StashOptions{}); err == nil {
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

	err := open(t, repo).StashPush(git.StashOptions{})

	var refusal *i18n.Error
	if !errors.As(err, &refusal) || refusal.Key != "git.nothingToStash" {
		t.Fatalf("StashPush error = %v, ожидался отказ git.nothingToStash", err)
	}
	if listed := runGit(t, repo, "stash", "list"); listed != "" {
		t.Fatalf("stash создан: %q", listed)
	}
}

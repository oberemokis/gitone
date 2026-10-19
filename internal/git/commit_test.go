package git_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/git"
	"gitone/internal/i18n"
)

func lastSubject(t *testing.T, repo string) string {
	t.Helper()
	return strings.TrimSpace(runGit(t, repo, "log", "-1", "--format=%s"))
}

// stopsOnConflict запускает git, который должен остановиться на конфликте и
// оставить операцию незавершённой.
func stopsOnConflict(t *testing.T, repo string, args ...string) {
	t.Helper()

	command := exec.Command("git", args...)
	command.Dir = repo
	if out, err := command.CombinedOutput(); err == nil {
		t.Fatalf("git %v прошёл без конфликта:\n%s", args, out)
	}
}

// Amend без новых правок меняет только сообщение, со застейдженными забирает
// их в последний коммит; LastMessage отдаёт тему и описание (#218, случаи 1
// и 2).
func TestAmend(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "tpyo", "-m", "body")
	parent := revision(t, repo, "HEAD")

	subject, description, err := git.LastMessage(repo)
	if err != nil || subject != "tpyo" || description != "body" {
		t.Fatalf("LastMessage = %q, %q, %v", subject, description, err)
	}

	if published, err := git.Amend(repo, "typo", "body"); err != nil || published {
		t.Fatalf("Amend = %v, %v", published, err)
	}
	if got := lastMessage(t, repo); got != "typo\n\nbody" {
		t.Fatalf("сообщение %q, ожидалось исправленное", got)
	}
	if count := strings.TrimSpace(runGit(t, repo, "rev-list", "--count", "HEAD")); count != "1" {
		t.Fatalf("коммитов %s, amend создал новый вместо замены (родитель был %s)", count, parent)
	}

	write(t, repo, "b.txt", "two\n")
	runGit(t, repo, "add", "b.txt")
	if _, err := git.Amend(repo, "typo", ""); err != nil {
		t.Fatalf("Amend с индексом: %v", err)
	}
	if staged := runGit(t, repo, "diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("в индексе осталось %q", staged)
	}
	if files := runGit(t, repo, "show", "--name-only", "--format=", "HEAD"); !strings.Contains(files, "b.txt") {
		t.Fatalf("b.txt не вошёл в коммит: %q", files)
	}
}

// Тема это первая строка, даже если первый абзац из двух строк; amend с пустой
// темой оставляет сообщение байт в байт, с табом (#299, случаи 1 и 2).
func TestMessageRoundTrip(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "-A")
	message := "Fix parser\nAlso handle CRLF\n\n\tindented"
	runGit(t, repo, "commit", "-q", "--cleanup=verbatim", "-m", message)

	subject, description, err := git.LastMessage(repo)
	if err != nil || subject != "Fix parser" || description != "Also handle CRLF\n\n\tindented" {
		t.Fatalf("LastMessage = %q, %q, %v", subject, description, err)
	}
	if _, err := git.Amend(repo, "", ""); err != nil {
		t.Fatalf("Amend без правки: %v", err)
	}
	if got := runGit(t, repo, "log", "-1", "--format=%B"); got != message+"\n\n" {
		t.Fatalf("сообщение %q, ожидалось прежнее %q", got, message)
	}
}

// Без коммитов amend нечего исправлять, а коммит, лежащий в upstream, после
// amend даёт расхождение (случаи 3 и 4).
func TestAmendRefusalsAndPublished(t *testing.T) {
	repo := newRepo(t)
	var refusal *i18n.Error
	if _, _, err := git.LastMessage(repo); !errors.As(err, &refusal) || refusal.Key != "git.noCommits" {
		t.Fatalf("LastMessage без коммитов = %v, ожидался отказ git.noCommits", err)
	}

	_, local := remotePair(t)
	if published, err := git.Amend(local, "rewritten", ""); err != nil || !published {
		t.Fatalf("Amend опубликованного = %v, %v; ожидалось расхождение", published, err)
	}
}

func TestCommitRecordsSubject(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "add", "a.txt")

	if err := git.Commit(repo, "поправить текст", ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := lastSubject(t, repo); got != "поправить текст" {
		t.Fatalf("тема коммита %q", got)
	}
}

// lastMessage это полное сообщение последнего коммита без перевода строки в
// конце.
func lastMessage(t *testing.T, repo string) string {
	t.Helper()
	return strings.TrimSuffix(runGit(t, repo, "log", "-1", "--format=%B"), "\n\n")
}

// stagedRepo это репозиторий с коммитом и застейдженной правкой a.txt.
func stagedRepo(t *testing.T) string {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "add", "a.txt")

	return repo
}

// Описание уходит вторым -m: git ставит между темой и телом пустую строку.
func TestCommitRecordsDescription(t *testing.T) {
	repo := stagedRepo(t)

	if err := git.Commit(repo, "fix", "line 1\nline 2"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if got := lastMessage(t, repo); got != "fix\n\nline 1\nline 2" {
		t.Fatalf("сообщение коммита %q, ожидались тема, пустая строка и тело", got)
	}
}

// Пробелы и пустые строки по краям описания обрезаются, описание из одних
// пробелов считается пустым и тела не даёт.
func TestCommitTrimsDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		message     string
	}{
		{name: "по краям", description: "\n  line 1\n\nline 3 \n\n", message: "fix\n\nline 1\n\nline 3"},
		{name: "одни пробелы", description: " \n\t \n", message: "fix"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := stagedRepo(t)

			if err := git.Commit(repo, "fix", tc.description); err != nil {
				t.Fatalf("Commit: %v", err)
			}

			if got := lastMessage(t, repo); got != tc.message {
				t.Fatalf("сообщение коммита %q, ожидалось %q", got, tc.message)
			}
		})
	}
}

// Пустой индекс: отказ из словаря, а не «On branch main» из вывода git commit.
func TestCommitWithNothingStagedFails(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	err := git.Commit(repo, "пусто", "")

	var refusal *i18n.Error
	if !errors.As(err, &refusal) || refusal.Key != "git.nothingToCommit" {
		t.Fatalf("ошибка %v, ожидался отказ git.nothingToCommit", err)
	}
	if err.Error() != "commit: nothing to commit" {
		t.Fatalf("ошибка %q, ожидалось «commit: nothing to commit»", err)
	}
}

// Завершение слияния с деревом, равным HEAD (`merge --no-commit -s ours`),
// проходит: индекс не отличается от HEAD, но MERGE_HEAD просит коммит слияния.
func TestCommitFinishesMergeWithTreeEqualToHead(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "other")
	write(t, repo, "b.txt", "other\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	runGit(t, repo, "merge", "-q", "--no-commit", "-s", "ours", "other")

	if err := git.Commit(repo, "слияние", ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := lastSubject(t, repo); got != "слияние" {
		t.Fatalf("тема коммита %q", got)
	}
	if parents := strings.Fields(runGit(t, repo, "log", "-1", "--format=%P")); len(parents) != 2 {
		t.Fatalf("родителей %d, ожидался коммит слияния с двумя", len(parents))
	}
}

// Cherry-pick, остановленный конфликтом, и revert при разрешении в пользу HEAD
// оставляют индекс равным HEAD. Решает git: пустой коммит он не делает, и этот
// отказ приходит его собственной ошибкой, а не отказом из словаря.
func TestCommitDuringCherryPickAndRevertLeavesDecisionToGit(t *testing.T) {
	tests := []struct {
		name  string
		start func(t *testing.T, repo string)
	}{
		{
			name: "cherry-pick",
			start: func(t *testing.T, repo string) {
				runGit(t, repo, "switch", "-q", "-c", "other", "HEAD~")
				write(t, repo, "a.txt", "other\n")
				commitAll(t, repo)
				runGit(t, repo, "switch", "-q", "main")
				stopsOnConflict(t, repo, "cherry-pick", "other")
			},
		},
		{
			name: "revert",
			start: func(t *testing.T, repo string) {
				stopsOnConflict(t, repo, "revert", "--no-edit", "HEAD~")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			write(t, repo, "a.txt", "one\n")
			commitAll(t, repo)
			write(t, repo, "a.txt", "two\n")
			commitAll(t, repo)
			write(t, repo, "a.txt", "three\n")
			commitAll(t, repo)
			tc.start(t, repo)
			runGit(t, repo, "checkout", "-q", "--ours", "a.txt")
			runGit(t, repo, "add", "a.txt")

			err := git.Commit(repo, "после остановки", "")

			var refusal *i18n.Error
			if err == nil || errors.As(err, &refusal) {
				t.Fatalf("ошибка %v, ожидался отказ самого git, а не отказ из словаря", err)
			}
		})
	}
}

// Первый коммит репозитория: HEAD ещё нет, а в индексе есть файл.
func TestCommitFirstCommit(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "a.txt")

	if err := git.Commit(repo, "первый", ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := lastSubject(t, repo); got != "первый" {
		t.Fatalf("тема коммита %q", got)
	}
}

// Упавший хук не даёт коммита: хуки не обходятся, а их отказ виден.
func TestFailingHookKeepsHead(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	before := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

	hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho отказ хука >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "add", "a.txt")

	if err := git.Commit(repo, "с хуком", ""); err == nil || !strings.Contains(err.Error(), "отказ хука") {
		t.Fatalf("Commit = %v, ожидался отказ с текстом хука", err)
	}
	if after := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD")); after != before {
		t.Fatal("HEAD сдвинулся при упавшем хуке")
	}
}

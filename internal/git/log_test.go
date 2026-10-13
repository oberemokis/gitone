package git_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/git"
)

// Коммиты идут от новых к старым, корневой помечен, у остальных родитель есть.
func TestLogListsNewestFirst(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)

	commits, err := git.Log(repo, 200)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("коммитов %d, ожидалось 2", len(commits))
	}
	if commits[0].Subject != "commit" || commits[0].Root {
		t.Fatalf("первый коммит %+v", commits[0])
	}
	if !commits[1].Root {
		t.Fatalf("корневой коммит не помечен: %+v", commits[1])
	}
}

// Слияние ветки рисуется, как в git log --graph: у коммитов свой рисунок,
// между ними строки соединения без коммита.
func TestLogDrawsGraphOfMerge(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "side")
	write(t, repo, "b.txt", "side\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	write(t, repo, "c.txt", "main\n")
	commitAll(t, repo)
	runGit(t, repo, "merge", "-q", "--no-ff", "-m", "merge", "side")

	entries, err := git.Log(repo, 200)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	want := []string{"*", `|\`, "| *", "* |", "|/", "*"}
	commits := []bool{true, false, true, true, false, true}
	if len(entries) != len(want) {
		t.Fatalf("строк %d, ожидалось %d: %+v", len(entries), len(want), entries)
	}
	for index, entry := range entries {
		if entry.Graph != want[index] || (entry.Hash != "") != commits[index] {
			t.Fatalf("строка %d: граф %q, хэш %q; ожидался граф %q, коммит %v", index, entry.Graph, entry.Hash, want[index], commits[index])
		}
	}
	if entries[0].Subject != "merge" || !entries[5].Root {
		t.Fatalf("слияние %+v, корень %+v", entries[0], entries[5])
	}
}

// log.showSignature=true из конфига пользователя вставляет строку проверки
// подписи между графом и форматом: граф и число коммитов остаются прежними.
func TestLogIgnoresShowSignatureOfSignedCommits(t *testing.T) {
	repo := newRepo(t)
	key := filepath.Join(t.TempDir(), "key")
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v\n%s", err, out)
	}
	runGit(t, repo, "config", "gpg.format", "ssh")
	runGit(t, repo, "config", "user.signingkey", key+".pub")
	runGit(t, repo, "config", "commit.gpgsign", "true")
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	commitAll(t, repo)
	runGit(t, repo, "config", "log.showSignature", "true")

	entries, err := git.Log(repo, 200)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("строк %d, ожидалось 2: %+v", len(entries), entries)
	}
	for _, entry := range entries {
		if entry.Graph != "*" || entry.Hash == "" {
			t.Errorf("строка %+v, ожидался коммит с графом «*»", entry)
		}
	}
}

// Разделитель полей может оказаться в теме: коммит остаётся в логе вместе со
// строкой графа, тема целая, остальные поля на местах.
func TestLogKeepsCommitWithSeparatorInSubject(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "fix\x1fthing")

	entries, err := git.Log(repo, 200)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("строк %d, ожидалась 1: %+v", len(entries), entries)
	}
	if entry := entries[0]; entry.Graph != "*" || entry.Subject != "fix\x1fthing" || entry.Author != "test" || !entry.Root || !strings.HasSuffix(entry.When, " ago") {
		t.Fatalf("строка %+v, ожидался корневой коммит «fix\\x1fthing» автора test с графом «*»", entry)
	}
}

// Разделитель полей может оказаться и в имени автора, а оно стоит в формате
// посередине: поля после него не сдвигаются, коммит один.
func TestLogKeepsFieldsWithSeparatorInAuthor(t *testing.T) {
	repo := newRepo(t)
	t.Setenv("GIT_AUTHOR_NAME", "Ann\x1fLee")
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	entries, err := git.Log(repo, 200)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("строк %d, ожидалась 1: %+v", len(entries), entries)
	}
	if entry := entries[0]; entry.Author != "Ann\x1fLee" || entry.Subject != "commit" || !entry.Root || !strings.HasSuffix(entry.When, " ago") {
		t.Fatalf("строка %+v, ожидался корневой коммит «commit» автора «Ann\\x1fLee»", entry)
	}
}

func TestLogOfEmptyRepositoryIsEmpty(t *testing.T) {
	repo := newRepo(t)

	commits, err := git.Log(repo, 200)
	if err != nil || len(commits) != 0 {
		t.Fatalf("коммиты %v, ошибка %v; ожидался пустой список без ошибки", commits, err)
	}
}

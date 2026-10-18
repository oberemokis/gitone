package git_test

import (
	"slices"
	"strings"
	"testing"

	"gitone/internal/git"
)

// lastJournal ищет в журнале последнюю запись с подкомандой command: журнал
// общий, и тесты пакета пишут в него параллельно.
func lastJournal(t *testing.T, command string, contains string) git.JournalEntry {
	t.Helper()

	for _, entry := range git.Journal() {
		if len(entry.Args) > 0 && entry.Args[0] == command && strings.Contains(strings.Join(entry.Args, " "), contains) {
			return entry
		}
	}
	t.Fatalf("в журнале нет git %s с %q", command, contains)
	return git.JournalEntry{}
}

// Успешная правка пишется с аргументами и временем, отказ с кодом выхода
// (#247, случаи 1 и 2); чтение не пишется.
func TestJournal(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	if err := git.CreateBranch(repo, "journal-branch", "HEAD"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if entry := lastJournal(t, "switch", "journal-branch"); entry.Exit != 0 || entry.When.IsZero() {
		t.Fatalf("запись %+v, ожидался успех со временем", entry)
	}

	if err := git.DeleteBranch(repo, "journal-missing"); err == nil {
		t.Fatal("удаление несуществующей ветки прошло")
	}
	if entry := lastJournal(t, "branch", "journal-missing"); entry.Exit == 0 {
		t.Fatalf("запись %+v, ожидался код выхода отказа", entry)
	}

	if _, err := git.Diff(repo, nil); err != nil {
		t.Fatalf("Diff: %v", err)
	}
	for _, entry := range git.Journal() {
		if entry.Args[0] == "diff" || entry.Args[0] == "rev-parse" {
			t.Fatalf("чтение в журнале: %q", entry.Args)
		}
	}
}

// Логин и пароль в адресе remote маскируются (случай 4), журнал помнит не
// больше 200 записей (случай 3).
func TestJournalMasksAndLimits(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	if err := git.AddRemote(repo, "secret", "https://user:token@example.com/repo.git"); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	entry := lastJournal(t, "remote", "secret")
	if joined := strings.Join(entry.Args, " "); strings.Contains(joined, "token") || !strings.Contains(joined, "https://***@example.com/repo.git") {
		t.Fatalf("адрес не замаскирован: %q", joined)
	}

	for range 210 {
		_ = git.DeleteTag(repo, "journal-limit")
	}
	if got := len(git.Journal()); got != 200 {
		t.Fatalf("в журнале %d записей, ожидалось 200", got)
	}
	if !slices.ContainsFunc(git.Journal()[:1], func(entry git.JournalEntry) bool { return entry.Args[0] == "tag" }) {
		t.Fatal("последняя запись не первая в журнале")
	}
}

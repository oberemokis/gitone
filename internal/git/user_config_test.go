package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/git"
)

// Настройки из ~/.gitconfig не меняют то, что gitone пишет в историю и
// разбирает (#381). Тесты ставят их локальным конфигом репозитория.

// commit.cleanup=strip не вырезает строки с «#» из сообщения Commit и Amend
// (случай 1).
func TestCommitIgnoresUserCleanup(t *testing.T) {
	repo := newRepo(t)
	runGit(t, repo, "config", "commit.cleanup", "strip")
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "-A")

	if err := open(t, repo).Commit("#42 fix crash", "details here"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := strings.TrimSpace(runGit(t, repo, "log", "-1", "--format=%B")); got != "#42 fix crash\n\ndetails here" {
		t.Fatalf("сообщение %q, ожидалось как набрано", got)
	}

	if _, err := open(t, repo).Amend("#43 amended", ""); err != nil {
		t.Fatalf("Amend: %v", err)
	}
	if got := strings.TrimSpace(runGit(t, repo, "log", "-1", "--format=%B")); got != "#43 amended" {
		t.Fatalf("сообщение после amend %q", got)
	}
}

// log.showSignature=true не ставит строки проверки подписи в сообщение
// подписанного коммита (случай 2).
func TestMessageIgnoresShowSignature(t *testing.T) {
	keygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		t.Skip("нет ssh-keygen для подписи коммита")
	}

	repo := newRepo(t)
	key := filepath.Join(t.TempDir(), "key")
	if out, err := exec.Command(keygen, "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v\n%s", err, out)
	}
	runGit(t, repo, "config", "gpg.format", "ssh")
	runGit(t, repo, "config", "user.signingkey", key)
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-S", "-m", "signed subject", "-m", "body")
	runGit(t, repo, "config", "log.showSignature", "true")

	subject, description, err := open(t, repo).Message("HEAD")
	if err != nil || subject != "signed subject" || description != "body" {
		t.Fatalf("Message = %q, %q, %v; ожидалось сообщение коммита", subject, description, err)
	}
}

// squash не оставляет в сообщении комментариев git при commit.cleanup
// whitespace и scissors (случай 3).
func TestSquashIgnoresUserCleanup(t *testing.T) {
	for _, cleanup := range []string{"whitespace", "scissors"} {
		t.Run(cleanup, func(t *testing.T) {
			repo, hashes := historyRepo(t)
			runGit(t, repo, "config", "commit.cleanup", cleanup)

			if _, err := open(t, repo).RewriteHistory(hashes[2], git.RewriteSquash, ""); err != nil {
				t.Fatalf("squash: %v", err)
			}
			if got := strings.TrimSpace(runGit(t, repo, "log", "-1", "--skip=1", "--format=%B")); got != "two\n\nthree" {
				t.Fatalf("сообщение после squash %q, ожидались две темы без комментариев git", got)
			}
		})
	}
}

// Дифф для постановки строк идёт без textconv: патч из него встаёт в индекс
// (случай 4).
func TestUnstagedDiffIgnoresTextconv(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, ".gitattributes", "*.txt diff=upper\n")
	runGit(t, repo, "config", "diff.upper.textconv", "tr a-z A-Z <")
	write(t, repo, "a.txt", "one\ntwo\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\nthree\n")

	text, err := open(t, repo).UnstagedDiff("a.txt")
	if err != nil || !strings.Contains(text, "+three") {
		t.Fatalf("UnstagedDiff = %v, ожидалась строка «+three» без textconv:\n%s", err, text)
	}
	if err := open(t, repo).ApplyPatch(text, true, false); err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if got := runGit(t, repo, "show", ":a.txt"); got != "one\nthree\n" {
		t.Fatalf("в индексе %q, ожидалась правка", got)
	}
}

// format.coverLetter=true не мешает копированию патча: один файл в корне
// (случай 5).
func TestPatchIgnoresCoverLetter(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "first change")
	runGit(t, repo, "config", "format.coverLetter", "true")

	name, _, err := open(t, repo).Patch(revision(t, repo, "HEAD"))
	if err != nil || name != "0001-first-change.patch" {
		t.Fatalf("Patch = %q, %v", name, err)
	}
	if patches, _ := filepath.Glob(filepath.Join(repo, "*.patch")); len(patches) != 1 {
		t.Fatalf("в корне патчи %q, ожидался один", patches)
	}
	if _, err := os.Stat(filepath.Join(repo, "0000-cover-letter.patch")); err == nil {
		t.Fatal("в корне осталось сопроводительное письмо")
	}
}

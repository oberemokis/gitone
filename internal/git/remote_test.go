package git_test

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/git"
	"gitone/internal/i18n"
)

// remotePair делает репозиторий с веткой main, привязанной к origin. Сети нет:
// origin это bare-репозиторий в соседнем временном каталоге.
func remotePair(t *testing.T) (origin, local string) {
	t.Helper()

	local = newRepo(t)
	write(t, local, "a.txt", "one\n")
	commitAll(t, local)

	origin = filepath.Join(t.TempDir(), "origin.git")
	runGit(t, local, "init", "-q", "--bare", "-b", "main", origin)
	runGit(t, local, "remote", "add", "origin", origin)
	runGit(t, local, "push", "-q", "--set-upstream", "origin", "main")

	return origin, local
}

// publish кладёт в main на origin коммит со стороны: так двигает ветку
// коллега.
func publish(t *testing.T, origin, name, content string) {
	t.Helper()

	peer := filepath.Join(t.TempDir(), "peer")
	runGit(t, filepath.Dir(peer), "clone", "-q", origin, peer)
	write(t, peer, name, content)
	commitAll(t, peer)
	runGit(t, peer, "push", "-q", "origin", "main")
}

func revision(t *testing.T, repo, name string) string {
	t.Helper()
	return strings.TrimSpace(runGit(t, repo, "rev-parse", name))
}

// Fetch сдвигает удалённую ветку, а локальная остаётся на месте (#232).
func TestFetchMovesRemoteBranchOnly(t *testing.T) {
	origin, local := remotePair(t)
	before := revision(t, local, "HEAD")
	publish(t, origin, "b.txt", "two\n")

	if err := git.Fetch(context.Background(), local); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got := revision(t, local, "origin/main"); got != revision(t, origin, "main") {
		t.Errorf("origin/main %s, ожидался main с origin %s", got, revision(t, origin, "main"))
	}
	if got := revision(t, local, "HEAD"); got != before {
		t.Errorf("HEAD сдвинулся: %s, был %s", got, before)
	}
}

// Без remote fetch отказывает, а не молчит, и недоступный remote даёт ошибку
// git.
func TestFetchRefusesWithoutRemoteOrNetwork(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	var refusal *i18n.Error
	if err := git.Fetch(context.Background(), repo); !errors.As(err, &refusal) || refusal.Key != "git.noRemote" {
		t.Fatalf("Fetch без remote = %v, ожидался отказ git.noRemote", err)
	}

	runGit(t, repo, "remote", "add", "origin", filepath.Join(t.TempDir(), "missing.git"))
	if err := git.Fetch(context.Background(), repo); err == nil || !strings.Contains(err.Error(), "fetch: ") {
		t.Fatalf("Fetch с недоступным remote = %v, ожидалась ошибка git", err)
	}
}

func TestPullFastForwardsBehindBranch(t *testing.T) {
	origin, local := remotePair(t)
	publish(t, origin, "b.txt", "two\n")

	updated, err := git.Pull(context.Background(), local)

	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if !updated {
		t.Error("Pull сказал, что ветка не изменилась")
	}
	if got := revision(t, local, "HEAD"); got != revision(t, origin, "main") {
		t.Errorf("HEAD %s, ожидался main с origin %s", got, revision(t, origin, "main"))
	}
	if got := readFile(t, filepath.Join(local, "b.txt")); got != "two\n" {
		t.Errorf("b.txt = %q, ожидалась перемотанная правка", got)
	}
}

func TestPullReportsUpToDateBranch(t *testing.T) {
	_, local := remotePair(t)
	before := revision(t, local, "HEAD")

	updated, err := git.Pull(context.Background(), local)

	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if updated {
		t.Error("Pull сказал, что ветка изменилась, а подтягивать было нечего")
	}
	if got := revision(t, local, "HEAD"); got != before {
		t.Errorf("HEAD %s, ожидался прежний %s", got, before)
	}
}

// Разошедшиеся ветки git не сливает и не перебазирует, даже если у пользователя
// включён pull.rebase: gitone просит только перемотку.
func TestPullRefusesDivergedBranches(t *testing.T) {
	cases := []struct {
		name   string
		config []string
	}{
		{name: "настройки по умолчанию"},
		{name: "pull.rebase=true", config: []string{"config", "pull.rebase", "true"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			origin, local := remotePair(t)
			publish(t, origin, "b.txt", "peer\n")
			write(t, local, "c.txt", "local\n")
			commitAll(t, local)
			if tc.config != nil {
				runGit(t, local, tc.config...)
			}
			before := revision(t, local, "HEAD")

			_, err := git.Pull(context.Background(), local)

			if err == nil || err.Error() != "pull: fatal: Not possible to fast-forward, aborting." {
				t.Fatalf("ошибка %v, ожидалась строка отказа git без отчёта о fetch и подсказок", err)
			}
			if got := revision(t, local, "HEAD"); got != before {
				t.Errorf("HEAD %s, ожидался прежний %s", got, before)
			}
			if _, statErr := os.Stat(filepath.Join(local, "b.txt")); !errors.Is(statErr, fs.ErrNotExist) {
				t.Errorf("b.txt появился в рабочем дереве: %v", statErr)
			}
			if status := runGit(t, local, "status", "--porcelain"); status != "" {
				t.Errorf("рабочее дерево изменилось:\n%s", status)
			}
		})
	}
}

// При detached HEAD отказ приходит из словаря: git не запускается.
func TestPullAndPushRefuseDetachedHead(t *testing.T) {
	cases := map[string]func(repo string) error{
		"pull": func(repo string) error {
			_, err := git.Pull(context.Background(), repo)
			return err
		},
		"push": func(repo string) error {
			_, err := git.Push(context.Background(), repo)
			return err
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			_, local := remotePair(t)
			runGit(t, local, "checkout", "-q", "--detach")

			err := call(local)

			var refusal *i18n.Error
			if !errors.As(err, &refusal) || refusal.Key != "git.detachedHead" {
				t.Fatalf("ошибка %v, ожидался отказ git.detachedHead", err)
			}
			if !strings.HasPrefix(err.Error(), name+": ") {
				t.Errorf("ошибка %q, ожидалось имя операции %q в начале", err, name)
			}
		})
	}
}

func TestPushSendsAheadCommits(t *testing.T) {
	origin, local := remotePair(t)
	write(t, local, "b.txt", "two\n")
	commitAll(t, local)

	target, err := git.Push(context.Background(), local)

	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if target != "origin/main" {
		t.Errorf("Push назвал цель %q, ожидалось origin/main", target)
	}
	if got := revision(t, origin, "main"); got != revision(t, local, "HEAD") {
		t.Errorf("main на origin %s, ожидался HEAD %s", got, revision(t, local, "HEAD"))
	}
}

// Ветка без upstream создаётся на origin и привязывается, как git push -u.
func TestPushCreatesUpstreamForNewBranch(t *testing.T) {
	origin, local := remotePair(t)
	runGit(t, local, "switch", "-q", "-c", "feature")
	write(t, local, "b.txt", "two\n")
	commitAll(t, local)

	target, err := git.Push(context.Background(), local)

	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if target != "origin/feature" {
		t.Errorf("Push назвал цель %q, ожидалось origin/feature", target)
	}
	if got := strings.TrimSpace(runGit(t, local, "rev-parse", "--abbrev-ref", "@{u}")); got != "origin/feature" {
		t.Errorf("upstream ветки %q, ожидался origin/feature", got)
	}
	if got := revision(t, origin, "feature"); got != revision(t, local, "HEAD") {
		t.Errorf("feature на origin %s, ожидался HEAD %s", got, revision(t, local, "HEAD"))
	}
}

func TestPushWithoutOriginShowsGitError(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	_, err := git.Push(context.Background(), repo)

	if err == nil || !strings.Contains(err.Error(), "'origin' does not appear to be a git repository") {
		t.Fatalf("ошибка %v, ожидался отказ git про origin", err)
	}
}

// Принудительного push нет: ветка, которую origin уже обогнал, отвергается, а
// чужой коммит остаётся на месте.
func TestPushNeverOverwritesRemoteCommits(t *testing.T) {
	origin, local := remotePair(t)
	publish(t, origin, "b.txt", "peer\n")
	peerHead := revision(t, origin, "main")
	write(t, local, "c.txt", "local\n")
	commitAll(t, local)

	_, err := git.Push(context.Background(), local)

	if err == nil || !strings.HasPrefix(err.Error(), "push: error: failed to push some refs") || strings.Contains(err.Error(), "\n") {
		t.Fatalf("ошибка %q, ожидалась одна строка отказа git «error: failed to push some refs»", err)
	}
	if got := revision(t, origin, "main"); got != peerHead {
		t.Errorf("main на origin %s, ожидался коммит коллеги %s", got, peerHead)
	}
}

// Переписанную ветку обычный push не отправляет и до сети говорит, что она
// разошлась с upstream; с --force-with-lease она уходит (#234, случай 1).
func TestPushForceLeaseSendsRewrittenBranch(t *testing.T) {
	origin, local := remotePair(t)
	runGit(t, local, "commit", "-q", "--amend", "-m", "rewritten")

	_, err := git.Push(context.Background(), local)
	refusedWith(t, err, "git.pushDiverged")

	target, err := git.PushForceLease(context.Background(), local)
	if err != nil || target != "origin/main" {
		t.Fatalf("PushForceLease = %q, %v", target, err)
	}
	if got, want := revision(t, origin, "main"), revision(t, local, "HEAD"); got != want {
		t.Fatalf("main на origin %s, ожидалась переписанная ветка %s", got, want)
	}
}

// При push.default=matching принудительный push не трогает другую ветку,
// отставшую от origin после fetch: коммит коллеги в ней на месте (#293,
// случай 1).
func TestPushForceLeaseSendsCurrentBranchOnly(t *testing.T) {
	origin, local := remotePair(t)
	runGit(t, local, "push", "-q", "origin", "main:other")
	runGit(t, local, "branch", "-q", "--track", "other", "origin/other")

	peer := filepath.Join(t.TempDir(), "peer")
	runGit(t, filepath.Dir(peer), "clone", "-q", "-b", "other", origin, peer)
	write(t, peer, "b.txt", "peer\n")
	commitAll(t, peer)
	runGit(t, peer, "push", "-q", "origin", "other")
	peerHead := revision(t, origin, "other")

	runGit(t, local, "config", "push.default", "matching")
	runGit(t, local, "fetch", "-q", "origin")
	runGit(t, local, "commit", "-q", "--amend", "-m", "rewritten")

	if _, err := git.PushForceLease(context.Background(), local); err != nil {
		t.Fatalf("PushForceLease: %v", err)
	}
	if got, want := revision(t, origin, "main"), revision(t, local, "HEAD"); got != want {
		t.Fatalf("main на origin %s, ожидалась переписанная ветка %s", got, want)
	}
	if got := revision(t, origin, "other"); got != peerHead {
		t.Fatalf("other на origin %s, ожидался коммит коллеги %s", got, peerHead)
	}
}

// Remote ушёл вперёд после последнего fetch: lease не совпал, git отказывает,
// чужой коммит на месте (случай 2).
func TestPushForceLeaseKeepsUnseenRemoteCommits(t *testing.T) {
	origin, local := remotePair(t)
	publish(t, origin, "b.txt", "peer\n")
	peerHead := revision(t, origin, "main")
	runGit(t, local, "commit", "-q", "--amend", "-m", "rewritten")

	if _, err := git.PushForceLease(context.Background(), local); err == nil || !strings.HasPrefix(err.Error(), "push: ") {
		t.Fatalf("PushForceLease = %v, ожидался отказ git", err)
	}
	if got := revision(t, origin, "main"); got != peerHead {
		t.Fatalf("main на origin %s, ожидался коммит коллеги %s", got, peerHead)
	}
}

// Remote просит пароль: git не читает терминал, причина приходит ошибкой.
func TestPullDoesNotPromptForPassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="gitone"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	_, local := remotePair(t)
	runGit(t, local, "remote", "set-url", "origin", server.URL+"/repo.git")

	_, err := git.Pull(context.Background(), local)

	if err == nil || !strings.Contains(err.Error(), "terminal prompts disabled") {
		t.Fatalf("ошибка %v, ожидался отказ git «terminal prompts disabled»", err)
	}
}

// fakeSSH ставит в PATH ssh, который записывает свои аргументы и отвечает
// отказом по ключу, как сервер без доступа. Возвращает путь к нему и файл с
// аргументами.
func fakeSSH(t *testing.T) (program, log string) {
	t.Helper()

	bin := t.TempDir()
	log = filepath.Join(t.TempDir(), "ssh-args")
	script := "#!/bin/sh\necho \"$@\" > '" + log + "'\necho 'Permission denied (publickey).' >&2\nexit 255\n"
	program = filepath.Join(bin, "ssh")
	if err := os.WriteFile(program, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	return program, log
}

// ssh не спрашивает пароль (BatchMode), если пользователь не задал свою
// команду ssh: GIT_SSH_COMMAND, GIT_SSH или core.sshCommand остаются его.
func TestSSHRunsInBatchModeUnlessUserChoseCommand(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, local, program string)
		batch bool
	}{
		{name: "по умолчанию", setup: func(*testing.T, string, string) {}, batch: true},
		{name: "GIT_SSH_COMMAND", setup: func(t *testing.T, _, _ string) { t.Setenv("GIT_SSH_COMMAND", "ssh -v") }},
		{name: "GIT_SSH", setup: func(t *testing.T, _, program string) { t.Setenv("GIT_SSH", program) }},
		{name: "core.sshCommand", setup: func(t *testing.T, local, _ string) { runGit(t, local, "config", "core.sshCommand", "ssh -v") }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, log := fakeSSH(t)
			_, local := remotePair(t)
			runGit(t, local, "remote", "set-url", "origin", "ssh://git@example.invalid/repo.git")
			tc.setup(t, local, program)

			_, err := git.Pull(context.Background(), local)

			if err == nil || !strings.Contains(err.Error(), "Permission denied (publickey)") {
				t.Fatalf("ошибка %v, ожидался отказ по ключу", err)
			}
			if got := strings.Contains(readFile(t, log), "BatchMode=yes"); got != tc.batch {
				t.Errorf("аргументы ssh %q: BatchMode=yes есть %v, ожидалось %v", readFile(t, log), got, tc.batch)
			}
		})
	}
}

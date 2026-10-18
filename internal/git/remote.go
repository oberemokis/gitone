package git

import (
	"context"
	"errors"
	"os"
	"strings"

	"gitone/internal/i18n"
)

// Fetch забирает коммиты всех remote, `git fetch --all`, и не трогает текущую
// ветку (#232). Без единого remote отказывает ошибкой из словаря: git fetch
// --all на таком репозитории молча выходит с успехом. Отмена ctx убивает git,
// ошибка тогда context.Canceled.
func Fetch(ctx context.Context, dir string) error {
	return failed("fetch", headline(fetch(ctx, dir)))
}

func fetch(ctx context.Context, dir string) error {
	remotes, err := run(call{dir: dir}, "remote")
	if err != nil {
		return err
	}
	if strings.TrimSpace(remotes) == "" {
		return i18n.New("git.noRemote", nil)
	}

	return network(ctx, dir, "fetch", "--all")
}

// Pull подтягивает текущую ветку из её upstream только перемоткой: `git pull
// --ff-only`. Разошедшиеся ветки git отвергает, ничего не сливая и не
// перебазируя, pull.rebase пользователя на это не влияет. updated говорит, что
// HEAD сдвинулся: по выводу git этого не узнать, git переводит его на язык
// пользователя. Отмена ctx убивает git, ошибка тогда context.Canceled.
func Pull(ctx context.Context, dir string) (updated bool, err error) {
	updated, err = pull(ctx, dir)
	return updated, failed("pull", headline(err))
}

func pull(ctx context.Context, dir string) (bool, error) {
	if _, err := currentBranch(dir); err != nil {
		return false, err
	}

	before := headCommit(dir)
	if err := network(ctx, dir, "pull", "--ff-only"); err != nil {
		return false, err
	}

	return headCommit(dir) != before, nil
}

// Push отправляет текущую ветку и возвращает, куда она ушла («origin/main»).
// Ветке без upstream создаётся копия на origin и привязывается к ней, как
// `git push -u origin HEAD`. Принудительного push нет: ветка, которую origin
// уже обогнал, отвергается. Отмена ctx убивает git, ошибка тогда
// context.Canceled.
func Push(ctx context.Context, dir string) (target string, err error) {
	target, err = push(ctx, dir)
	return target, failed("push", headline(err))
}

func push(ctx context.Context, dir string) (string, error) {
	branch, err := currentBranch(dir)
	if err != nil {
		return "", err
	}

	upstream, err := run(call{dir: dir}, "rev-parse", "--abbrev-ref", "@{upstream}")
	if err != nil {
		return "origin/" + branch, network(ctx, dir, "push", "--set-upstream", "origin", "HEAD")
	}

	return strings.TrimSpace(upstream), network(ctx, dir, "push")
}

// currentBranch возвращает имя ветки. При detached HEAD отказывает ошибкой из
// словаря: pull и push без ветки нечего делать, а git ответил бы своим
// текстом. symbolic-ref выходит с кодом 1 именно в этом случае, остальные
// отказы (нет репозитория) идут с другим кодом.
func currentBranch(dir string) (string, error) {
	ref, err := run(call{dir: dir}, "symbolic-ref", "--quiet", "HEAD")

	var refusal *gitError
	if errors.As(err, &refusal) && refusal.exit.ExitCode() == 1 {
		return "", i18n.New("git.detachedHead", nil)
	}

	return strings.TrimPrefix(strings.TrimSpace(ref), "refs/heads/"), err
}

// headCommit это хэш HEAD, а в репозитории без коммитов пустая строка.
func headCommit(dir string) string {
	head, _ := run(call{dir: dir}, "rev-parse", "--verify", "--quiet", "HEAD")
	return strings.TrimSpace(head)
}

// network запускает git, который ходит по сети. Он не читает терминал
// интерфейса: stdin пустой, пароль не спрашивается (GIT_TERMINAL_PROMPT=0 в
// run), а ssh работает в BatchMode. Отмена ctx приходит как ctx.Err(), а не
// как «signal: terminated» от убитого git.
func network(ctx context.Context, dir string, args ...string) error {
	_, err := run(call{ctx: ctx, dir: dir, env: sshEnv(dir)}, args...)
	if err != nil && ctx.Err() != nil {
		return ctx.Err()
	}

	return err
}

// sshEnv заставляет ssh отказывать, а не спрашивать пароль и passphrase в
// терминале интерфейса. Свою команду ssh пользователь задаёт через
// GIT_SSH_COMMAND, core.sshCommand или GIT_SSH: тогда ничего не добавляется,
// иначе наша команда перекрыла бы его (GIT_SSH_COMMAND старше всех трёх).
func sshEnv(dir string) []string {
	for _, name := range []string{"GIT_SSH_COMMAND", "GIT_SSH"} {
		if _, set := os.LookupEnv(name); set {
			return nil
		}
	}

	if configured, err := run(call{dir: dir}, "config", "--get", "core.sshCommand"); err == nil && strings.TrimSpace(configured) != "" {
		return nil
	}

	return []string{"GIT_SSH_COMMAND=ssh -o BatchMode=yes"}
}

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
func (r Repo) Fetch(ctx context.Context) error {
	return failed("fetch", headline(r.fetch(ctx)))
}

func (r Repo) fetch(ctx context.Context) error {
	remotes, err := r.inDir("remote")
	if err != nil {
		return err
	}
	if strings.TrimSpace(remotes) == "" {
		return i18n.New("git.noRemote", nil)
	}

	return r.network(ctx, r.dir, "fetch", "--all")
}

// Pull подтягивает текущую ветку из её upstream только перемоткой: `git pull
// --ff-only`. Разошедшиеся ветки git отвергает, ничего не сливая и не
// перебазируя, pull.rebase пользователя на это не влияет. updated говорит, что
// HEAD сдвинулся: по выводу git этого не узнать, git переводит его на язык
// пользователя. Отмена ctx убивает git, ошибка тогда context.Canceled.
func (r Repo) Pull(ctx context.Context) (updated bool, err error) {
	updated, err = r.pull(ctx)
	return updated, failed("pull", headline(err))
}

func (r Repo) pull(ctx context.Context) (bool, error) {
	if _, err := r.currentBranch(); err != nil {
		return false, err
	}

	before := r.headCommit()
	if err := r.network(ctx, r.dir, "pull", "--ff-only"); err != nil {
		return false, err
	}

	return r.headCommit() != before, nil
}

// Push отправляет текущую ветку и возвращает, куда она ушла («origin/main»).
// Ветке без upstream создаётся копия на origin и привязывается к ней, как
// `git push -u origin HEAD`. Ветка, у upstream которой есть коммиты, которых
// нет в ветке (после amend, reset, rebase или чужого push), обычным push не
// отправляется: отказ git.pushDiverged идёт до сети, и решать, отправлять ли
// принудительно, будет пользователь (PushForceLease, #234). Отмена ctx убивает
// git, ошибка тогда context.Canceled.
func (r Repo) Push(ctx context.Context) (target string, err error) {
	target, err = r.push(ctx, false)
	return target, failed("push", headline(err))
}

// PushForceLease отправляет текущую ветку поверх upstream с
// `--force-with-lease` (#234): remote переписывается, только если он стоит
// там же, где его видел последний fetch. Иначе git отказывает, и чужие коммиты
// не теряются. Refspec задан явно: без него что отправлять, решает
// push.default, и при matching git переписал бы и другие ветки, о которых
// вопрос не спрашивал.
func (r Repo) PushForceLease(ctx context.Context) (target string, err error) {
	target, err = r.push(ctx, true)
	return target, failed("push", headline(err))
}

func (r Repo) push(ctx context.Context, force bool) (string, error) {
	branch, err := r.currentBranch()
	if err != nil {
		return "", err
	}

	upstream, err := r.inDir("rev-parse", "--abbrev-ref", "@{upstream}")
	if err != nil {
		return "origin/" + branch, r.network(ctx, r.dir, "push", "--set-upstream", "origin", "HEAD")
	}
	upstream = strings.TrimSpace(upstream)

	if force {
		where, err := r.inDir("for-each-ref", "--format=%(upstream:remotename)%00%(upstream:remoteref)", "refs/heads/"+branch)
		remote, ref, found := strings.Cut(strings.TrimSpace(where), "\x00")
		if err != nil || !found {
			return upstream, err
		}
		return upstream, r.network(ctx, r.dir, "push", "--force-with-lease="+ref, remote, "HEAD:"+ref)
	}
	if _, err := r.inDir("merge-base", "--is-ancestor", "@{upstream}", "HEAD"); err != nil {
		return upstream, i18n.New("git.pushDiverged", i18n.Args{"upstream": upstream})
	}
	return upstream, r.network(ctx, r.dir, "push")
}

// currentBranch возвращает имя ветки. При detached HEAD отказывает ошибкой из
// словаря: pull и push без ветки нечего делать, а git ответил бы своим
// текстом. symbolic-ref выходит с кодом 1 именно в этом случае, остальные
// отказы (нет репозитория) идут с другим кодом.
func (r Repo) currentBranch() (string, error) {
	ref, err := r.inDir("symbolic-ref", "--quiet", "HEAD")

	var refusal *gitError
	if errors.As(err, &refusal) && refusal.exit.ExitCode() == 1 {
		return "", i18n.New("git.detachedHead", nil)
	}

	return strings.TrimPrefix(strings.TrimSpace(ref), "refs/heads/"), err
}

// headCommit это хэш HEAD, а в репозитории без коммитов пустая строка.
func (r Repo) headCommit() string {
	head, _ := r.inDir("rev-parse", "--verify", "--quiet", "HEAD")
	return strings.TrimSpace(head)
}

// network запускает git, который ходит по сети, из каталога dir: pull и push
// идут из каталога запуска, подмодуль из корня. Он не читает терминал
// интерфейса: stdin пустой, пароль не спрашивается (GIT_TERMINAL_PROMPT=0 в
// run), а ssh работает в BatchMode. Отмена ctx приходит как ctx.Err(), а не
// как «signal: terminated» от убитого git.
func (r Repo) network(ctx context.Context, dir string, args ...string) error {
	_, err := run(call{ctx: ctx, dir: dir, env: r.sshEnv()}, args...)
	if err != nil && ctx.Err() != nil {
		return ctx.Err()
	}

	return err
}

// sshEnv заставляет ssh отказывать, а не спрашивать пароль и passphrase в
// терминале интерфейса. Свою команду ssh пользователь задаёт через
// GIT_SSH_COMMAND, core.sshCommand или GIT_SSH: тогда ничего не добавляется,
// иначе наша команда перекрыла бы его (GIT_SSH_COMMAND старше всех трёх).
func (r Repo) sshEnv() []string {
	for _, name := range []string{"GIT_SSH_COMMAND", "GIT_SSH"} {
		if _, set := os.LookupEnv(name); set {
			return nil
		}
	}

	if configured, err := r.inDir("config", "--get", "core.sshCommand"); err == nil && strings.TrimSpace(configured) != "" {
		return nil
	}

	return []string{"GIT_SSH_COMMAND=ssh -o BatchMode=yes"}
}

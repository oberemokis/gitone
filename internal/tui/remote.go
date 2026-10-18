package tui

import (
	"context"
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// remoteKind это то, что делает идущая сетевая операция.
type remoteKind int

const (
	noRemote remoteKind = iota
	pulling
	pushing
	fetching
	// push с --force-with-lease после вопроса (#234)
	forcePushing
	// отправка тега на origin (#238)
	pushingTag
)

// remote это идущий pull или push. Он идёт командой в фоне, интерфейс при этом
// отзывчив. id растёт с каждой операцией: шаг спиннера и ответ чужой операции
// отбрасываются. cancel отменяет контекст команды, и git убивается (esc).
type remote struct {
	kind   remoteKind
	id     int
	spin   int
	cancel context.CancelFunc
}

func (r remote) running() bool {
	return r.kind != noRemote
}

// labelKey это надпись индикатора в подвале, busyKey заметка об отказе.
func (r remote) labelKey() string {
	switch r.kind {
	case pushing, forcePushing, pushingTag:
		return "screen.pushing"
	case fetching:
		return "screen.fetching"
	}
	return "screen.pulling"
}

func (r remote) busyKey() string {
	switch r.kind {
	case pushing, forcePushing, pushingTag:
		return "remote.pushRunning"
	case fetching:
		return "remote.fetchRunning"
	}
	return "remote.pullRunning"
}

// remoteDoneMsg несёт итог pull или push: сдвинулся ли HEAD, куда ушла ветка
// или ошибку git. Отмена приходит как context.Canceled.
type remoteDoneMsg struct {
	id      int
	kind    remoteKind
	updated bool
	target  string
	err     error
}

// remoteSpinMsg это шаг спиннера операции id. У pull и push своя цепочка шагов:
// перечитывание диффа во время операции крутит свой спиннер, и её цепочка не
// должна на нём оборваться.
type remoteSpinMsg struct {
	id int
}

func remoteSpinTick(id int) tea.Cmd {
	return tea.Tick(spinnerStep, func(time.Time) tea.Msg { return remoteSpinMsg{id: id} })
}

// pullCmd и pushCmd запускают git в команде, а не в Update: сеть бывает
// медленной, и экран не должен висеть.
func pullCmd(ctx context.Context, dir string, id int) tea.Cmd {
	return func() tea.Msg {
		updated, err := git.Pull(ctx, dir)
		return remoteDoneMsg{id: id, kind: pulling, updated: updated, err: err}
	}
}

func pushCmd(ctx context.Context, dir string, id int, kind remoteKind) tea.Cmd {
	push := git.Push
	if kind == forcePushing {
		push = git.PushForceLease
	}
	return func() tea.Msg {
		target, err := push(ctx, dir)
		return remoteDoneMsg{id: id, kind: kind, target: target, err: err}
	}
}

func fetchCmd(ctx context.Context, dir string, id int) tea.Cmd {
	return func() tea.Msg {
		return remoteDoneMsg{id: id, kind: fetching, err: git.Fetch(ctx, dir)}
	}
}

// startRemote запускает pull, push или fetch. Вторая операция не стартует, пока
// идёт первая: это отказ из changesRepo, он решён выше по вызову.
func (m Model) startRemote(kind remoteKind) (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	m.remote = remote{kind: kind, id: m.remote.id + 1, cancel: cancel}

	run := pullCmd(ctx, m.dir, m.remote.id)
	switch kind {
	case pushing, forcePushing:
		run = pushCmd(ctx, m.dir, m.remote.id, kind)
	case fetching:
		run = fetchCmd(ctx, m.dir, m.remote.id)
	}

	return m, tea.Batch(remoteSpinTick(m.remote.id), run)
}

// stopRemote отменяет идущую операцию: git убит, ответ придёт сообщением с
// context.Canceled. Без операции ничего не делает.
func (m Model) stopRemote() Model {
	if m.remote.running() {
		m.remote.cancel()
	}
	return m
}

// remoteDone ставит итог в подвал и перечитывает дифф, ветку и списки: pull
// меняет дерево, а при ошибке видно, что на самом деле осталось, как у
// actionDone. Итог и отмена идут не цветом ошибки (sayCalm).
func (m Model) remoteDone(msg remoteDoneMsg) (tea.Model, tea.Cmd) {
	if !m.remote.running() || msg.id != m.remote.id {
		return m, nil
	}

	m.remote.cancel()
	m.remote = remote{id: m.remote.id}

	// разошедшаяся ветка ничего не поменяла: вопрос о принудительном push, и
	// перечитывать нечего, а перечитывание сняло бы вопрос из подвала
	var diverged *i18n.Error
	if errors.As(msg.err, &diverged) && diverged.Key == "git.pushDiverged" && m.active == noWindow {
		return m.askForcePush(diverged.Args["upstream"]), nil
	}

	switch {
	case errors.Is(msg.err, context.Canceled):
		m = m.sayCalm(m.text.Text("remote.cancelled", nil))
	case msg.err != nil:
		m = m.say(m.text.Error(msg.err))
	default:
		m = m.sayCalm(m.remoteResult(msg))
	}

	m.stale = true
	return m.startReload()
}

// askForcePush спрашивает в подвале, отправить ли разошедшуюся ветку с
// --force-with-lease (#234), как lazygit на P. Отдельной клавиши нет:
// принудительный push нужен только тогда, когда обычный отказал.
func (m Model) askForcePush(upstream any) Model {
	args := questionArgs(m.keys)
	args["upstream"] = upstream
	return m.ask(m.text.Text("remote.forceQuestion", args), func(m Model) (tea.Model, tea.Cmd) {
		if next, refused := m.refuseWhileRemote(); refused {
			return next, nil
		}
		return m.startRemote(forcePushing)
	})
}

// remoteResult это итог успешной операции на языке словаря, а не сырой вывод
// git.
func (m Model) remoteResult(msg remoteDoneMsg) string {
	switch {
	case msg.kind == pushing || msg.kind == forcePushing:
		return m.text.Text("remote.pushed", i18n.Args{"target": msg.target})
	case msg.kind == pushingTag:
		return m.text.Text("tags.pushed", i18n.Args{"name": visible(msg.target)})
	case msg.kind == fetching:
		return m.text.Text("remote.fetched", nil)
	case msg.updated:
		return m.text.Text("remote.fastForwarded", nil)
	}

	return m.text.Text("remote.upToDate", nil)
}

// changesRepo говорит, что глобальное действие меняет репозиторий или само
// запускает pull, push или fetch. Пока идёт сетевая операция, такое действие
// отказывает: git pull меняет дерево, и x посреди него стёр бы не те правки.
// Действия окон (переключение ветки и worktree, apply и drop у stash) проверяют
// то же сами. Просмотр, окна и перечитывание диффа работают.
func changesRepo(action keymap.Action) bool {
	switch action {
	case keymap.FilesStage, keymap.FilesDiscard, keymap.FilesCommit, keymap.FilesAmend, keymap.FilesIgnore, keymap.StashPush, keymap.GitPull, keymap.GitPush, keymap.GitFetch, keymap.GitUndo, keymap.GitRedo, keymap.GitOperation:
		return true
	}

	return false
}

// refuseWhileRemote ставит в подвал отказ («pull is running»), пока идёт pull
// или push. refused true значит, что действие делать нельзя.
func (m Model) refuseWhileRemote() (next Model, refused bool) {
	if !m.remote.running() {
		return m, false
	}

	return m.putNotice(m.text.Text(m.remote.busyKey(), nil), false), true
}

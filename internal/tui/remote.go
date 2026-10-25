package tui

import (
	"context"
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
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
	// клон и обновление подмодуля (#244)
	updatingSubmodule
)

// remote это идущая сетевая операция. Она идёт командой в фоне, интерфейс
// при этом отзывчив. id растёт с каждой операцией: шаг спиннера и ответ чужой
// операции отбрасываются. cancel отменяет контекст команды, и git убивается
// (esc).
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
	case updatingSubmodule:
		return "screen.updatingSubmodule"
	}
	return "screen.pulling"
}

func (r remote) busyKey() string {
	switch r.kind {
	case pushing, forcePushing, pushingTag:
		return "remote.pushRunning"
	case fetching:
		return "remote.fetchRunning"
	case updatingSubmodule:
		return "remote.submoduleRunning"
	}
	return "remote.pullRunning"
}

// remoteDoneMsg несёт итог сетевой операции: сдвинулся ли HEAD, куда ушла
// ветка или ошибку git. Отмена приходит как context.Canceled.
type remoteDoneMsg struct {
	id      int
	kind    remoteKind
	updated bool
	target  string
	err     error
}

func (msg remoteDoneMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.remoteDone(msg) }

// remoteSpinMsg это шаг спиннера операции id. У сетевой операции своя цепочка
// шагов: перечитывание диффа во время операции крутит свой спиннер, и её
// цепочка не должна на нём оборваться.
type remoteSpinMsg struct {
	id int
}

func (msg remoteSpinMsg) apply(m Model) (tea.Model, tea.Cmd) {
	if !m.remote.running() || msg.id != m.remote.id {
		return m, nil
	}
	m.remote.spin++
	return m, m.remoteSpinTick(msg.id)
}

func (m Model) remoteSpinTick(id int) tea.Cmd {
	return tea.Tick(m.spinStep, func(time.Time) tea.Msg { return remoteSpinMsg{id: id} })
}

// syncWork это git операций pull, push, принудительного push и fetch для
// startRemote.
func syncWork(repo git.Repo, kind remoteKind) func(context.Context) remoteDoneMsg {
	switch kind {
	case pushing, forcePushing:
		push := repo.Push
		if kind == forcePushing {
			push = repo.PushForceLease
		}
		return func(ctx context.Context) remoteDoneMsg {
			target, err := push(ctx)
			return remoteDoneMsg{target: target, err: err}
		}
	case fetching:
		return func(ctx context.Context) remoteDoneMsg { return remoteDoneMsg{err: repo.Fetch(ctx)} }
	}

	return func(ctx context.Context) remoteDoneMsg {
		updated, err := repo.Pull(ctx)
		return remoteDoneMsg{updated: updated, err: err}
	}
}

// startRemote запускает сетевую операцию kind со спиннером в подвале и отменой
// по esc. work это её git: он идёт в команде, а не в Update, потому что сеть
// бывает медленной и экран не должен висеть. Номер и вид операции в ответ
// ставит startRemote. Вторая операция не стартует, пока идёт первая: это
// отказ из changesRepo, он решён выше по вызову.
func (m Model) startRemote(kind remoteKind, work func(context.Context) remoteDoneMsg) (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	m.remote = remote{kind: kind, id: m.remote.id + 1, cancel: cancel}

	id := m.remote.id
	return m, tea.Batch(m.remoteSpinTick(id), func() tea.Msg {
		msg := work(ctx)
		msg.id, msg.kind = id, kind
		return msg
	})
}

// stopRemote отменяет идущую операцию: git убит, ответ придёт сообщением с
// context.Canceled. Без операции ничего не делает.
func (m Model) stopRemote() Model {
	if m.remote.running() {
		m.remote.cancel()
	}
	return m
}

// remoteDone ставит итог в тост и перечитывает дифф, ветку и списки: pull
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

	next, reload := m.startReload()
	// открытое окно подмодулей показывает новое состояние
	if msg.kind == updatingSubmodule && m.active == submoduleWindow {
		return next, tea.Batch(reload, submodulesCmd(m.repo))
	}
	return next, reload
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
		return m.startRemote(forcePushing, syncWork(m.repo, forcePushing))
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
	case msg.kind == updatingSubmodule:
		return m.text.Text("submodules.updated", i18n.Args{"path": visible(msg.target)})
	case msg.kind == fetching:
		return m.text.Text("remote.fetched", nil)
	case msg.updated:
		return m.text.Text("remote.fastForwarded", nil)
	}

	return m.text.Text("remote.upToDate", nil)
}

// refuseWhileRemote ставит тостом отказ («pull is running»), пока идёт pull
// или push. refused true значит, что действие делать нельзя.
func (m Model) refuseWhileRemote() (next Model, refused bool) {
	if !m.remote.running() {
		return m, false
	}

	return m.putNotice(m.text.Text(m.remote.busyKey(), nil)), true
}

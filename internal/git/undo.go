package git

import (
	"strings"

	"gitone/internal/i18n"
)

// Step это откат или повтор одного движения HEAD по reflog (#239), как z и
// ctrl+z в lazygit. Checkout значит переход на Target (ветку или коммит),
// иначе HEAD переставляется на Target через reset --soft: файлы и индекс не
// трогаются, изменения откаченного коммита остаются в индексе. Action и
// Subject это запись reflog, которую шаг откатывает или повторяет.
type Step struct {
	Redo     bool
	Checkout bool
	Target   string
	Action   string
	Subject  string
}

// Записи reflog, которые оставляют откат и повтор. По ним следующий откат
// знает, что уже откачено: без них второй откат вернул бы первый.
const (
	undoAction = "[gitone undo]"
	redoAction = "[gitone redo]"
)

// move это движение HEAD из reflog: откуда и куда, и запись, которая его
// сделала. Rebase от start до finish считается одним движением.
type move struct {
	checkout bool
	from, to string
	entry    ReflogEntry
}

// UndoStep находит последнее движение HEAD, которое ещё не откачено. Последнее
// изменение, не двигавшее HEAD (stash), откатить этим нельзя, и шаг не
// отменяет вместо него более старое движение.
func (r Repo) UndoStep() (Step, error) {
	entries, err := r.Reflog()
	if err != nil {
		return Step{}, err
	}
	if r.stashedLast(entries) {
		return Step{}, i18n.New("git.undoStash", nil)
	}

	var step Step
	err = walk(entries, func(counter int, found move) (bool, error) {
		if counter != 0 {
			return false, nil
		}
		if found.from == "" {
			return true, i18n.New("git.nothingToUndo", nil)
		}
		step = Step{Checkout: found.checkout, Target: found.from, Action: found.entry.Action, Subject: found.entry.Subject}
		return true, nil
	})
	if err == nil && step.Target == "" {
		err = i18n.New("git.nothingToUndo", nil)
	}
	return step, err
}

// RedoStep находит движение, откаченное последним. Новое движение HEAD после
// отката повтор сбрасывает: оно стоит в reflog раньше записи отката.
func (r Repo) RedoStep() (Step, error) {
	entries, err := r.Reflog()
	if err != nil {
		return Step{}, err
	}

	var step Step
	err = walk(entries, func(counter int, found move) (bool, error) {
		switch {
		case counter == 0:
			return true, i18n.New("git.nothingToRedo", nil)
		case counter > 1:
			return false, nil
		}
		step = Step{Redo: true, Checkout: found.checkout, Target: found.to, Action: found.entry.Action, Subject: found.entry.Subject}
		return true, nil
	})
	if err == nil && step.Target == "" {
		err = i18n.New("git.nothingToRedo", nil)
	}
	return step, err
}

// walk идёт по reflog от новых записей к старым, как parseReflogForActions в
// lazygit. counter это число откатов, ещё не погашенных движениями: запись
// отката его растит, запись повтора уменьшает, каждое пройденное движение
// гасит один откат. visit решает, остановиться ли на движении.
func walk(entries []ReflogEntry, visit func(counter int, found move) (bool, error)) error {
	counter, rebaseEnd := 0, ""
	for index, entry := range entries {
		previous := ""
		if index+1 < len(entries) {
			previous = entries[index+1].Hash
		}

		var found *move
		switch {
		case rebaseEnd != "":
			if rebaseStep(entry.Action, "(start)") {
				found = &move{from: previous, to: rebaseEnd, entry: entry}
				rebaseEnd = ""
			}
		case entry.Action == undoAction:
			counter++
		case entry.Action == redoAction:
			counter--
		case rebaseStep(entry.Action, "(finish)"), rebaseStep(entry.Action, "(abort)"):
			rebaseEnd = entry.Hash
		case rebaseStep(entry.Action, "(start)"):
			return i18n.New("git.undoRebasing", nil)
		case entry.Action == "checkout":
			if from, to, ok := checkoutTargets(entry.Subject); ok {
				found = &move{checkout: true, from: from, to: to, entry: entry}
			}
		case movesHead(entry.Action):
			found = &move{from: previous, to: entry.Hash, entry: entry}
		}

		// переход на то же место ничего не двигал: stash, reset --hard на HEAD
		if found == nil || found.from == found.to {
			continue
		}
		if stop, err := visit(counter, *found); stop {
			return err
		}
		counter--
	}
	return nil
}

// rebaseStep узнаёт стадию rebase в записи reflog. pull с rebase пишет свои:
// «pull --rebase (start)», а при pull.rebase=true «pull (start)».
func rebaseStep(action, stage string) bool {
	return (strings.HasPrefix(action, "rebase") || strings.HasPrefix(action, "pull")) && strings.HasSuffix(action, stage)
}

// checkoutTargets разбирает «moving from main to feature».
func checkoutTargets(subject string) (from, to string, ok bool) {
	rest, found := strings.CutPrefix(subject, "moving from ")
	if !found {
		return "", "", false
	}
	return strings.Cut(rest, " to ")
}

// movesHead говорит, что действие переставило HEAD на новый коммит. merge,
// cherry-pick и revert lazygit пропускает, и тогда откат молча отменил бы
// движение до них.
func movesHead(action string) bool {
	for _, prefix := range []string{"commit", "reset", "pull", "merge", "cherry-pick", "revert"} {
		if strings.HasPrefix(action, prefix) {
			return true
		}
	}
	return false
}

// stashedLast говорит, что последним изменением был stash: git stash пишет в
// reflog reset на тот же HEAD, а верхняя запись stash стоит на HEAD. После
// pop или drop запись уходит, и откат снова работает.
func (r Repo) stashedLast(entries []ReflogEntry) bool {
	if len(entries) < 2 || entries[0].Action != "reset" || entries[0].Hash != entries[1].Hash {
		return false
	}

	parent, err := r.inDir("rev-parse", "-q", "--verify", "refs/stash^1")
	return err == nil && strings.TrimSpace(parent) == entries[0].Hash
}

// ApplyStep делает шаг. Запись reflog получает метку отката или повтора через
// GIT_REFLOG_ACTION, как в lazygit. Переход с правками, которые мешают, git
// отвергает сам.
func (r Repo) ApplyStep(step Step) error {
	action := undoAction
	if step.Redo {
		action = redoAction
	}
	args := []string{"reset", "-q", "--soft", step.Target}
	if step.Checkout {
		args = []string{"checkout", "-q", step.Target, "--"}
	}

	_, err := run(call{dir: r.root, env: []string{"GIT_REFLOG_ACTION=" + action}}, args...)
	return failed(args[0], headline(err))
}

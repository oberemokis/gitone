package git

import "gitone/internal/i18n"

// operation это незавершённая операция git (#243): merge, rebase, am,
// cherry-pick или revert, пустая строка без неё. Имя совпадает с командой
// git: её --continue и --abort продолжают и прерывают операцию. bisect
// (#245) идёт последним: конфликт посреди поиска важнее самого поиска.
func operation(root string) string {
	switch {
	case rebasing(root):
		return "rebase"
	case applying(root):
		return "am"
	}
	for _, state := range []struct{ head, name string }{
		{"MERGE_HEAD", "merge"},
		{"CHERRY_PICK_HEAD", "cherry-pick"},
		{"REVERT_HEAD", "revert"},
	} {
		if _, err := run(call{dir: root}, "rev-parse", "--quiet", "--verify", state.head); err == nil {
			return state.name
		}
	}
	if bisecting(root) {
		return "bisect"
	}
	return ""
}

// Continue продолжает незавершённую операцию. Сообщение коммита git берёт
// приготовленное: редактор не открывается. Нерешённый конфликт git
// отвергает своей причиной. Если операция дошла до следующего конфликта,
// ошибка из словаря говорит, что она снова остановилась. Остальные отказы
// (упавший хук) идут строкой git: конфликта в них нет.
func (r Repo) Continue() error {
	op, err := r.runningOperation()
	if err != nil {
		return err
	}

	conflicted, err := r.ConflictedPaths()
	if err != nil {
		return err
	}
	_, err = run(call{dir: r.root, env: []string{"GIT_EDITOR=true"}}, op, "--continue")
	if err != nil && len(conflicted) == 0 && r.stoppedOnConflict() {
		return i18n.New("git.stoppedOnConflict", i18n.Args{"operation": op})
	}
	return failed(op, headline(err))
}

// stoppedOnConflict говорит, что операция остановилась и в индексе есть
// конфликтные пути.
func (r Repo) stoppedOnConflict() bool {
	conflicted, _ := r.ConflictedPaths()
	return len(conflicted) > 0 && unfinished(r.root)
}

// autostashConflict проверяет операцию, которую git закончил успехом: с
// autostash (--autostash, rebase.autoStash, merge.autoStash) он выходит с 0,
// даже когда отложенные правки не встали обратно. Они тогда лежат в stash, а
// в дереве маркеры конфликта (#388). Неслитые пути до операции git бы не
// пропустил, поэтому после успеха они значат именно это.
func (r Repo) autostashConflict(operation string) error {
	if conflicted, _ := r.ConflictedPaths(); len(conflicted) > 0 {
		return i18n.New("git.autostashConflict", i18n.Args{"operation": operation})
	}
	return nil
}

// Abort прерывает незавершённую операцию: HEAD, индекс и дерево
// возвращаются к состоянию до неё.
func (r Repo) Abort() error {
	op, err := r.runningOperation()
	if err != nil {
		return err
	}

	_, err = r.inRoot(op, "--abort")
	return failed(op, headline(err))
}

// runningOperation находит операцию для Continue и Abort. У bisect нет ни
// --continue, ни --abort: он идёт отметками и кончается bisect reset.
func (r Repo) runningOperation() (string, error) {
	switch op := operation(r.root); op {
	case "":
		return "", i18n.New("git.noOperation", nil)
	case "bisect":
		return "", i18n.New("git.bisectNoContinue", nil)
	default:
		return op, nil
	}
}

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
// ошибка из словаря говорит, что она снова остановилась.
func Continue(dir string) error {
	root, op, err := runningOperation(dir)
	if err != nil {
		return err
	}

	conflicted, err := ConflictedPaths(root)
	if err != nil {
		return err
	}
	_, err = run(call{dir: root, env: []string{"GIT_EDITOR=true"}}, op, "--continue")
	if err != nil && len(conflicted) == 0 && unfinished(root) {
		return i18n.New("git.stoppedOnConflict", i18n.Args{"operation": op})
	}
	return failed(op, headline(err))
}

// Abort прерывает незавершённую операцию: HEAD, индекс и дерево
// возвращаются к состоянию до неё.
func Abort(dir string) error {
	root, op, err := runningOperation(dir)
	if err != nil {
		return err
	}

	_, err = run(call{dir: root}, op, "--abort")
	return failed(op, headline(err))
}

func runningOperation(dir string) (root, op string, err error) {
	root, err = topLevel(dir)
	if err != nil {
		return "", "", failed("rev-parse", err)
	}
	if op = operation(root); op == "" {
		return "", "", i18n.New("git.noOperation", nil)
	}
	return root, op, nil
}

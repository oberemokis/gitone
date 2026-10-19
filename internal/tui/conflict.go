package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitone/internal/diff"
	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// startConflicts включает выбор конфликтов на файле с конфликтом слияния
// (#242), как панель merge conflicts в lazygit: курсор ходит по конфликтам,
// files.stage открывает меню сторон. У бинарного файла и у файла без
// маркеров конфликтов в тексте нет, меню сразу предлагает сторону для файла
// целиком.
func (m Model) startConflicts(file diff.File) Model {
	if !file.Binary {
		if view, found := m.view.SelectConflicts(); found {
			m.view = view
			return m.putNotice(m.text.Text("conflict.started", i18n.Args{
				"move":   firstKey(m.keys, keymap.DiffScrollDown) + "/" + firstKey(m.keys, keymap.DiffScrollUp),
				"choose": firstKey(m.keys, keymap.FilesStage),
				"close":  firstKey(m.keys, keymap.HelpClose),
			}), false)
		}
	}

	return m.openMenu(m.text.Text("conflict.fileTitle", i18n.Args{"file": fileTitle(file, m.text)}), m.fileSides(stagedKey(file), "", ""))
}

// chooseSide открывает меню сторон для конфликта под курсором: наша, их, обе
// и сторона для файла целиком. Метки сторон берутся из маркеров, «HEAD» и имя
// ветки понятнее слов ours и theirs, которые при rebase меняются местами.
func (m Model) chooseSide() (tea.Model, tea.Cmd) {
	start, found := m.view.ConflictStart()
	if !found {
		return m, nil
	}

	file := m.files[m.current]
	path, dir := stagedKey(file), m.dir
	return m, func() tea.Msg {
		text, err := git.WorktreeFile(dir, path)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		ours, theirs, found := diff.ConflictLabels(text, start)
		if !found {
			return actionDoneMsg{err: i18n.New("conflict.gone", nil)}
		}
		return conflictLabelsMsg{path: path, start: start, ours: ours, theirs: theirs}
	}
}

// conflictLabelsMsg несёт метки сторон конфликта, который начинается строкой
// start файла path.
type conflictLabelsMsg struct {
	path         string
	start        int
	ours, theirs string
}

// sidesArrived открывает меню сторон, если файл ещё открыт и выбор идёт.
func (m Model) sidesArrived(msg conflictLabelsMsg) Model {
	if len(m.files) == 0 || stagedKey(m.files[m.current]) != msg.path || !m.view.SelectingConflicts() || m.active != noWindow {
		return m
	}

	side := func(label string, chosen diff.ConflictSide) menuItem {
		return menuItem{label: label, run: func(m Model) (tea.Model, tea.Cmd) {
			return m, resolveConflictCmd(m.dir, msg.path, msg.start, chosen, m.conflictNotes(msg.path))
		}}
	}
	items := []menuItem{
		side(m.text.Text("conflict.useSide", i18n.Args{"side": visible(msg.ours)}), diff.SideOurs),
		side(m.text.Text("conflict.useSide", i18n.Args{"side": visible(msg.theirs)}), diff.SideTheirs),
		side(m.text.Text("conflict.useBoth", i18n.Args{"ours": visible(msg.ours), "theirs": visible(msg.theirs)}), diff.SideBoth),
	}
	items = append(items, m.fileSides(msg.path, msg.ours, msg.theirs)...)
	return m.openMenu(m.text.Text("conflict.title", nil), items)
}

// fileSides это строки меню со стороной для файла целиком. Без меток (у
// бинарного файла маркеров нет) стороны называются словами ours и theirs.
func (m Model) fileSides(path, ours, theirs string) []menuItem {
	if ours == "" {
		ours, theirs = m.text.Text("conflict.ours", nil), m.text.Text("conflict.theirs", nil)
	}

	whole := func(label string, useTheirs bool) menuItem {
		note := m.text.Text("conflict.fileResolved", i18n.Args{"file": visible(path)})
		return menuItem{label: label, run: func(m Model) (tea.Model, tea.Cmd) {
			dir := m.dir
			return m, func() tea.Msg {
				return actionDoneMsg{err: git.ResolveFile(dir, path, useTheirs), note: note}
			}
		}}
	}
	return []menuItem{
		whole(m.text.Text("conflict.useFile", i18n.Args{"side": visible(ours)}), false),
		whole(m.text.Text("conflict.useFile", i18n.Args{"side": visible(theirs)}), true),
	}
}

// conflictNotes это итоги решения: конфликт решён, или решён последний и файл
// поставлен в индекс.
func (m Model) conflictNotes(path string) [2]string {
	return [2]string{
		m.text.Text("conflict.resolved", nil),
		m.text.Text("conflict.fileResolved", i18n.Args{"file": visible(path)}),
	}
}

// resolveConflictCmd заменяет конфликт, начинающийся строкой start, на
// выбранную сторону. Когда в файле не осталось конфликтов, кроме тех, что уже
// лежали в HEAD, файл ставится в индекс: git считает конфликт решённым, и
// операция ждёт продолжения.
func resolveConflictCmd(dir, path string, start int, side diff.ConflictSide, notes [2]string) tea.Cmd {
	return func() tea.Msg {
		text, err := git.WorktreeFile(dir, path)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		resolved, found := diff.ResolveConflict(text, start, side)
		if !found {
			return actionDoneMsg{err: i18n.New("conflict.gone", nil)}
		}
		if err := git.WriteWorktreeFile(dir, path, resolved); err != nil {
			return actionDoneMsg{err: err}
		}

		if diff.ConflictCount(resolved) > diff.ConflictCount(git.HeadFile(dir, path)) {
			return actionDoneMsg{note: notes[0]}
		}
		return actionDoneMsg{err: git.Stage(dir, []string{path}), note: notes[1]}
	}
}

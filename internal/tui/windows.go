package tui

import (
	"cmp"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/keymap"
)

// windowKind это то, чем окна различаются (#352). Корень читает описание
// открытого окна, а не перебирает окна в каждом месте: новое окно это одна
// запись в windowKinds, а забытую ловит TestWindowKindsComplete.
type windowKind struct {
	// key разбирает нажатие, пока окно открыто: окно забирает его целиком
	key func(Model, tea.KeyPressMsg) (tea.Model, tea.Cmd)
	// box рисует окно; question это общий вопрос над ним, пусто без вопроса.
	// nil у окна, которое своего рисунка не имеет
	box func(m Model, p paint, question string) string
	// rows это число строк списка: колесо и клик ставят по ним курсор. nil у
	// окна без списка
	rows func(Model) int
	// enter это действие по клику на строку, пусто там, где клик только
	// ставит курсор
	enter keymap.Action
	// move и pick заменяют общий курсор m.cursor там, где курсор свой: у поиска
	// файлов и меню, а в логе строки графа курсор перешагивает. pick с false
	// значит, что на строке нечего выбрать
	move func(m Model, delta int) Model
	pick func(m Model, index int) (Model, bool)
	// close это клавиша закрытия: её жмёт клик мимо окна, как кнопку закрытия
	// в заголовке (#367). Пусто у окон-списков и меню, их закрывает list.close
	close keymap.Action
}

var windowKinds = map[window]windowKind{
	pickerWindow: {
		// поиск файлов гасит подвал, как главный экран
		key: func(m Model, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
			m.notice = ""
			return m.pickerKey(key)
		},
		box: func(m Model, p paint, _ string) string {
			return pickerBox(p, m.picker, len(m.files) == 0, m.width, m.bodyHeight())
		},
		enter: keymap.SearchChoose,
		close: keymap.SearchClose,
		move: func(m Model, delta int) Model {
			m.picker = m.picker.Move(delta)
			return m
		},
		pick: func(m Model, index int) (Model, bool) {
			m.picker = m.picker.Select(index)
			return m, true
		},
	},
	commitWindow: {
		key: Model.typeCommit,
		box: func(m Model, p paint, _ string) string {
			return commitBox(p, m.commitTitle(), m.subject, m.description, m.commitField, m.width)
		},
		close: keymap.CommitClose,
	},
	logWindow: {
		key: Model.logKey,
		box: func(m Model, p paint, question string) string {
			return logBox(p, m.logShown(), m.logFilter, m.cursor, m.query, m.searching, question, m.width, m.bodyHeight())
		},
		rows:  func(m Model) int { return len(m.logShown()) },
		enter: keymap.LogOpen,
		// шаг за шагом, как клавиши: строки соединения графа курсор перешагивает
		move: func(m Model, delta int) Model {
			shown, step := m.logShown(), 1
			if delta < 0 {
				step = -1
			}
			for range delta * step {
				m.cursor = commitRow(shown, m.cursor, m.moveCursor(step, len(shown)).cursor)
			}
			return m
		},
		// строка соединения графа коммитом не является
		pick: func(m Model, index int) (Model, bool) {
			if m.logShown()[index].Hash == "" {
				return m, false
			}
			m.cursor = index
			return m, true
		},
	},
	branchWindow: {
		key: Model.branchKey,
		box: func(m Model, p paint, question string) string {
			return branchBox(p, m.branchesShown(), m.cursor, m.query, m.searching, question, m.width, m.bodyHeight())
		},
		rows:  func(m Model) int { return len(m.branchesShown()) },
		enter: keymap.BranchesSwitch,
	},
	worktreeWindow: {
		key: Model.worktreeKey,
		box: func(m Model, p paint, _ string) string {
			return worktreeBox(p, m.worktreesShown(), m.cursor, m.query, m.searching, m.width, m.bodyHeight())
		},
		rows:  func(m Model) int { return len(m.worktreesShown()) },
		enter: keymap.WorktreesSwitch,
	},
	stashWindow: {
		key: Model.stashKey,
		box: func(m Model, p paint, _ string) string {
			return stashBox(p, m.stashes, m.cursor, git.Stash{}, m.width, m.bodyHeight())
		},
		rows:  func(m Model) int { return len(m.stashes) },
		enter: keymap.StashDiff,
	},
	tagWindow: {
		key: Model.tagKey,
		box: func(m Model, p paint, question string) string {
			return tagBox(p, m.tagsShown(), m.cursor, m.query, m.searching, question, m.width, m.bodyHeight())
		},
		rows:  func(m Model) int { return len(m.tagsShown()) },
		enter: keymap.TagsDiff,
	},
	// у remote, подмодулей и журнала нет действия на enter: клик ставит курсор
	remoteWindow: {
		key: Model.remoteKey,
		box: func(m Model, p paint, question string) string {
			return remoteBox(p, m.remotes, m.cursor, question, m.width, m.bodyHeight())
		},
		rows: func(m Model) int { return len(m.remotes) },
	},
	reflogWindow: {
		key: Model.reflogKey,
		box: func(m Model, p paint, _ string) string {
			return reflogBox(p, m.reflogShown(), m.cursor, m.query, m.searching, m.width, m.bodyHeight())
		},
		rows:  func(m Model) int { return len(m.reflogShown()) },
		enter: keymap.LogOpen,
	},
	submoduleWindow: {
		key: Model.submoduleKey,
		box: func(m Model, p paint, _ string) string {
			return submoduleBox(p, m.submodules, m.cursor, m.width, m.bodyHeight())
		},
		rows: func(m Model) int { return len(m.submodules) },
	},
	journalWindow: {
		key: Model.journalKey,
		box: func(m Model, p paint, _ string) string {
			return journalBox(p, m.journal, m.cursor, m.width, m.bodyHeight())
		},
		rows: func(m Model) int { return len(m.journal) },
	},
	recentWindow: {
		key: Model.recentKey,
		box: func(m Model, p paint, _ string) string {
			return recentBox(p, m.recent, m.cursor, m.width, m.bodyHeight())
		},
		rows:  func(m Model) int { return len(m.recent) },
		enter: keymap.RecentOpen,
	},
	promptWindow: {
		key:   Model.typePrompt,
		box:   func(m Model, p paint, _ string) string { return promptBox(p, m.prompt, m.width) },
		close: keymap.PromptClose,
	},
	menuWindow: {
		key: Model.menuKey,
		box: func(m Model, p paint, _ string) string {
			return menuBox(p, m.menu, m.width, m.bodyHeight())
		},
		enter: keymap.MenuChoose,
		move: func(m Model, delta int) Model {
			m.menu = m.menu.move(delta)
			return m
		},
		pick: func(m Model, index int) (Model, bool) {
			m.menu.cursor = index
			return m, true
		},
	},
	// вопрос об удалении stash курсора не имеет: колесо и клик его пропускают
	stashDropWindow: {
		key: Model.answerStashDrop,
		box: func(m Model, p paint, _ string) string {
			return stashBox(p, m.stashes, m.cursor, m.droppingNow(), m.width, m.bodyHeight())
		},
	},
	// общий вопрос своего рисунка не имеет: он стоит последней строкой окна под
	// ним (asking) или в подвале
	questionWindow: {key: Model.answerQuestion},
}

// moveBy сдвигает курсор окна на delta строк: своим move или общим курсором
// по rows. Окно без списка не двигается.
func (k windowKind) moveBy(m Model, delta int) Model {
	switch {
	case k.move != nil:
		return k.move(m, delta)
	case k.rows != nil:
		return m.moveCursor(delta, k.rows(m))
	}
	return m
}

// closeAction это клавиша закрытия окна (windowKind.close).
func (k windowKind) closeAction() keymap.Action {
	return cmp.Or(k.close, keymap.ListClose)
}

// pickRow ставит курсор на строку index: своим pick или общим курсором.
// false значит, что у окна нет строк или на строке нечего выбрать.
func (k windowKind) pickRow(m Model, index int) (Model, bool) {
	switch {
	case k.pick != nil:
		return k.pick(m, index)
	case k.rows != nil:
		m.cursor = index
		return m, true
	}
	return m, false
}

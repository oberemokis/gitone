package tui_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// Колесо в окнах-списках двигает курсор на три строки за щелчок, как три
// нажатия list.down или list.up, с упором в первую и последнюю строку (#197).

// wheel делает clicks щелчков колеса в одну сторону.
func wheel(model tea.Model, button tea.MouseButton, clicks int) tea.Model {
	for range clicks {
		model, _ = model.Update(tea.MouseWheelMsg{Button: button})
	}
	return model
}

// numbered это count имён prefix01, prefix02, … по порядку.
func numbered(prefix string, count int) []string {
	names := make([]string, count)
	for index := range names {
		names[index] = fmt.Sprintf("%s%02d", prefix, index+1)
	}
	return names
}

// listWindow это окно-список из двенадцати строк и подписи четырёх из них:
// первой, четвёртой, седьмой и последней. Курсор открытого окна на первой.
type listWindow struct {
	name                        string
	open                        func(t *testing.T) tea.Model
	first, fourth, seventh, end string
}

// openTwelveCommits открывает лог из коммитов item-01 … item-12, последний
// наверху, на экране 100 на 20: в окно помещаются пять строк.
func openTwelveCommits(t *testing.T) tea.Model {
	t.Helper()

	model := resize(open(t, options(commitSubjects(t, numbered("item-", 12)...), layout.Focus())), 100, 20)
	return openAsync(model, key('l'))
}

func listWindows() []listWindow {
	return []listWindow{
		{
			name: "лог",
			open: openTwelveCommits,
			// наверху лога последний коммит
			first: "item-12", fourth: "item-09", seventh: "item-06", end: "item-01",
		},
		{
			name: "ветки",
			open: func(t *testing.T) tea.Model {
				repo := newRepo(t)
				write(t, repo, "a.txt", "a\n")
				commitAll(t, repo)
				for _, name := range numbered("n", 11) {
					runGit(t, repo, "branch", name)
				}

				model := resize(open(t, options(repo, layout.Focus())), 100, 20)
				return openAsync(model, key('B'))
			},
			// git отдаёт ветки по алфавиту, main перед n01, курсор стоит на текущей
			first: "main", fourth: "n03", seventh: "n06", end: "n11",
		},
		{
			name: "worktree",
			open: func(t *testing.T) tea.Model {
				repo := newRepo(t)
				write(t, repo, "a.txt", "a\n")
				commitAll(t, repo)
				parent := t.TempDir()
				for _, name := range numbered("w", 11) {
					runGit(t, repo, "worktree", "add", "-q", "-b", name, filepath.Join(parent, name))
				}

				model := resize(open(t, options(repo, layout.Focus())), 100, 20)
				return openAsync(model, key('W'))
			},
			// основное дерево первое, дальше добавленные по порядку
			first: "main", fourth: "w03", seventh: "w06", end: "w11",
		},
		{
			name: "stash",
			open: func(t *testing.T) tea.Model {
				repo := newRepo(t)
				write(t, repo, "a.txt", "a\n")
				commitAll(t, repo)
				for _, name := range numbered("s", 12) {
					write(t, repo, "a.txt", name+"\n")
					runGit(t, repo, "stash", "push", "-q", "-m", name)
				}

				model := resize(open(t, options(repo, layout.Focus())), 100, 20)
				return openAsync(model, key('Z'))
			},
			// наверху списка последний stash
			first: "s12", fourth: "s09", seventh: "s06", end: "s01",
		},
		{
			name: "поиск файлов",
			open: func(t *testing.T) tea.Model {
				repo := newRepo(t)
				names := numbered("f", 12)
				for _, name := range names {
					write(t, repo, name+".txt", "old\n")
				}
				commitAll(t, repo)
				for _, name := range names {
					write(t, repo, name+".txt", "new\n")
				}

				model := resize(open(t, options(repo, layout.Focus())), 100, 20)
				model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
				return model
			},
			first: "f01.txt", fourth: "f04.txt", seventh: "f07.txt", end: "f12.txt",
		},
	}
}

// Один щелчок вниз уводит курсор на четвёртую строку, второй на седьмую.
func TestWheelDownMovesCursorThreeRows(t *testing.T) {
	for _, window := range listWindows() {
		t.Run(window.name, func(t *testing.T) {
			model := window.open(t)

			model = wheel(model, tea.MouseWheelDown, 1)
			if row := markedRow(model); !strings.Contains(row, window.fourth) {
				t.Fatalf("после щелчка курсор на %q, want %s:\n%s", row, window.fourth, screenText(model))
			}

			model = wheel(model, tea.MouseWheelDown, 1)
			if row := markedRow(model); !strings.Contains(row, window.seventh) {
				t.Fatalf("после второго щелчка курсор на %q, want %s:\n%s", row, window.seventh, screenText(model))
			}
		})
	}
}

// Колесо вверх возвращает курсор, а на первой строке он остаётся.
func TestWheelUpStopsAtFirstRow(t *testing.T) {
	for _, window := range listWindows() {
		t.Run(window.name, func(t *testing.T) {
			model := wheel(window.open(t), tea.MouseWheelDown, 2)

			model = wheel(model, tea.MouseWheelUp, 1)
			if row := markedRow(model); !strings.Contains(row, window.fourth) {
				t.Fatalf("после щелчка вверх курсор на %q, want %s:\n%s", row, window.fourth, screenText(model))
			}

			model = wheel(model, tea.MouseWheelUp, 3)
			if row := markedRow(model); !strings.Contains(row, window.first) {
				t.Fatalf("курсор на %q, want упор в %s:\n%s", row, window.first, screenText(model))
			}
		})
	}
}

// Пять щелчков вниз приводят курсор на двенадцатую строку, шестой на ней же
// и оставляет.
func TestWheelDownStopsAtLastRow(t *testing.T) {
	for _, window := range listWindows() {
		t.Run(window.name, func(t *testing.T) {
			model := wheel(window.open(t), tea.MouseWheelDown, 5)
			if row := markedRow(model); !strings.Contains(row, window.end) {
				t.Fatalf("курсор на %q, want %s:\n%s", row, window.end, screenText(model))
			}

			model = wheel(model, tea.MouseWheelDown, 1)
			if row := markedRow(model); !strings.Contains(row, window.end) {
				t.Fatalf("после шестого щелчка курсор на %q, want упор в %s:\n%s", row, window.end, screenText(model))
			}
		})
	}
}

// Лог из 40 коммитов, три щелчка вниз: курсор на десятой строке, окно
// прокрутилось за ним, верх списка ушёл с экрана.
func TestWheelScrollsLogWithCursor(t *testing.T) {
	subjects := numbered("item-", 40)
	model := resize(open(t, options(commitSubjects(t, subjects...), layout.Focus())), 100, 20)
	model = openAsync(model, key('l'))

	model = wheel(model, tea.MouseWheelDown, 3)

	// наверху лога последний коммит item-40, в десятой строке item-31
	if row := markedRow(model); !strings.Contains(row, "item-31") {
		t.Fatalf("под курсором %q, want item-31:\n%s", row, screenText(model))
	}
	if screen := screenText(model); strings.Contains(screen, "item-40") {
		t.Fatalf("верх списка остался на экране:\n%s", screen)
	}
}

// Поле поиска открыто, набрано «/fix»: колесо ходит по найденным строкам, а
// поле остаётся открытым.
func TestWheelMovesCursorOverFoundRows(t *testing.T) {
	model := searchLog(t, "fix-a", "note-1", "fix-b", "note-2", "fix-c", "fix-d", "note-3", "fix-e", "fix-f")
	model = typeText(model, "/fix")

	model = wheel(model, tea.MouseWheelDown, 1)

	// найденные сверху вниз: fix-f, fix-e, fix-d, fix-c, fix-b, fix-a
	if row := markedRow(model); !strings.Contains(row, "fix-c") {
		t.Fatalf("под курсором %q, want fix-c:\n%s", row, screenText(model))
	}
	if screen := screenText(model); !strings.Contains(screen, "/ fix") {
		t.Fatalf("поле поиска закрылось:\n%s", screen)
	}

	// упор в последнюю найденную строку, а не в последний коммит лога
	model = wheel(model, tea.MouseWheelDown, 2)
	if row := markedRow(model); !strings.Contains(row, "fix-a") {
		t.Fatalf("под курсором %q, want упор в fix-a:\n%s", row, screenText(model))
	}
}

// Три шага колеса идут как три j: строки соединения графа курсор
// перешагивает. В логе со слиянием щелчок вниз с первой строки ставит курсор
// на последний коммит, а не на строку соединения или main work.
func TestWheelStepsOverGraphConnectors(t *testing.T) {
	model := mergeLog(t)

	// сверху вниз: merge side, «|\», side work, main work, «|/», base
	model = wheel(model, tea.MouseWheelDown, 1)
	if row := markedRow(model); !strings.Contains(row, "base") {
		t.Fatalf("под курсором %q, want base:\n%s", row, screenText(model))
	}

	model = wheel(model, tea.MouseWheelDown, 1)
	if row := markedRow(model); !strings.Contains(row, "base") {
		t.Fatalf("под курсором %q, want упор в base:\n%s", row, screenText(model))
	}

	model = wheel(model, tea.MouseWheelUp, 1)
	if row := markedRow(model); !strings.Contains(row, "merge side") {
		t.Fatalf("под курсором %q, want merge side:\n%s", row, screenText(model))
	}
}

// Колесо вбок в логе курсор не двигает.
func TestWheelSidewaysKeepsCursor(t *testing.T) {
	model := openTwelveCommits(t)

	model = wheel(model, tea.MouseWheelRight, 2)
	model = wheel(model, tea.MouseWheelLeft, 2)

	if row := markedRow(model); !strings.Contains(row, "item-12") {
		t.Fatalf("под курсором %q, want item-12:\n%s", row, screenText(model))
	}
}

// Под вопросом об удалении stash курсора нет: колесо его не двигает, и ответ
// относится к тому stash, что был под курсором до вопроса.
func TestWheelUnderStashDropQuestionKeepsCursor(t *testing.T) {
	repo, model := openTwoStashes(t)
	model, _ = model.Update(key('d'))

	model = wheel(model, tea.MouseWheelDown, 1)
	model, cmd := model.Update(key('y'))
	settle(model, cmd, 4)

	// верхний stash хранит «three», нижний «two»: удалён верхний
	if got := runGit(t, repo, "show", "stash@{0}:a.txt"); got != "two" {
		t.Fatalf("остался stash с %q, want нижний с \"two\"", got)
	}
}

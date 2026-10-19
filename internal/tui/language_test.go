package tui_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/i18n"
	"gitone/internal/layout"
)

// pseudo это словарь, где каждый текст взят в ⟦⟧. Слово вне скобок на экране
// значит строку, которую забыли провести через словарь: со вторым языком она
// осталась бы английской.
func pseudo() i18n.Dictionary {
	texts := map[string]string{}
	for _, key := range i18n.Keys() {
		texts[key] = "⟦" + i18n.Dictionary{}.Text(key, nil) + "⟧"
	}

	return i18n.Custom("pseudo", texts)
}

var (
	marked = regexp.MustCompile(`⟦[^⟧]*⟧`)
	hash   = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
	word   = regexp.MustCompile(`\p{L}{2,}`)
)

// notOurs это слова, которые на экран кладём не мы и не переводим: имена
// клавиш, имена операций git в префиксе ошибки и ответы git (ссылка stash,
// подпись stash, относительная дата коммита).
var notOurs = map[string]bool{
	"ctrl": true, "space": true, "up": true, "down": true, "left": true, "right": true, "esc": true, "enter": true, "tab": true,
	"stash": true, "push": true, "WIP": true, "on": true,
	"year": true, "years": true, "month": true, "months": true, "ago": true,
}

func untranslated(screen []string) []string {
	var found []string
	for _, line := range screen {
		line = hash.ReplaceAllString(marked.ReplaceAllString(line, ""), "")
		for _, candidate := range word.FindAllString(line, -1) {
			if !notOurs[candidate] {
				found = append(found, candidate)
			}
		}
	}

	return found
}

// pseudoRepo это репозиторий, где всё, что пишем мы сами (имена файлов,
// содержимое, тема коммита, автор, ветки), без слов: на экране словами
// остаётся только интерфейс.
func pseudoRepo(t *testing.T) string {
	t.Helper()

	repo := newRepo(t)
	t.Setenv("GIT_AUTHOR_NAME", "0")
	t.Setenv("GIT_AUTHOR_DATE", "2020-01-01T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2020-01-01T00:00:00Z")

	write(t, repo, "1", "1\n")
	write(t, repo, "2", "2\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "1")
	runGit(t, repo, "branch", "-m", "0")
	runGit(t, repo, "branch", "1")
	return repo
}

func openPseudo(t *testing.T, repo string) tea.Model {
	t.Helper()

	opts := options(repo, layout.Default())
	opts.Text = pseudo()
	return resize(open(t, opts), 120, 40)
}

// withChanges это pseudoRepo с правкой в 1 и новым файлом 3.
func withChanges(t *testing.T) (string, tea.Model) {
	t.Helper()

	repo := pseudoRepo(t)
	write(t, repo, "1", "1\n3\n")
	write(t, repo, "3", "3\n")
	return repo, openPseudo(t, repo)
}

func TestScreensUseDictionary(t *testing.T) {
	ctrlP := tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}

	tests := []struct {
		name   string
		screen func(t *testing.T) tea.Model
		// shows это текст словаря, без которого окно могло не открыться
		shows string
		// gitRows узнаёт строки, которые пишет git: действие и сообщение reflog,
		// аргументы команд в журнале
		gitRows *regexp.Regexp
	}{
		{name: "основной экран", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			return model
		}},
		{name: "справка", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			model, _ = model.Update(key('?'))
			return model
		}},
		{name: "поиск", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			model, _ = model.Update(ctrlP)
			return model
		}},
		{name: "поиск без совпадений", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			model, _ = model.Update(ctrlP)
			return typeText(model, "9")
		}},
		{name: "коммит", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			model, _ = model.Update(key('c'))
			return model
		}},
		{name: "вопрос об отмене правок", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			model, _ = model.Update(key('x'))
			return model
		}},
		{name: "лог", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			return openAsync(model, key('l'))
		}},
		{name: "первый коммит в логе", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			model, _ = openAsync(model, key('l')).Update(enter)
			return model
		}},
		{name: "ветки", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			return openAsync(model, key('B'))
		}},
		{name: "пустой stash", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			return openAsync(model, key('Z'))
		}},
		{name: "вопрос об удалении stash", screen: func(t *testing.T) tea.Model {
			repo, _ := withChanges(t)
			runGit(t, repo, "stash", "push", "-q")
			model := openAsync(openPseudo(t, repo), key('Z'))
			model, _ = model.Update(key('d'))
			return model
		}},
		{name: "своя ошибка в подвале", screen: func(t *testing.T) tea.Model {
			model := openPseudo(t, pseudoRepo(t))
			model, cmd := model.Update(key('z'))
			return settle(model, cmd, 4)
		}},
		{name: "нет изменений", screen: func(t *testing.T) tea.Model {
			return openPseudo(t, pseudoRepo(t))
		}},
		{name: "загрузка", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			model, _ = model.Update(key('r'))
			return model
		}},
		{name: "теги", shows: "⟦No tags⟧", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			return openAsync(model, key('t'))
		}},
		{name: "remote", shows: "⟦No remotes⟧", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			return openAsync(model, key('R'))
		}},
		{name: "reflog", shows: "⟦Reflog⟧", gitRows: hash, screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			return openAsync(model, key('L'))
		}},
		{name: "подмодули", shows: "⟦No submodules⟧", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			return openAsync(model, key('S'))
		}},
		// журнал общий на процесс: запись есть всегда, а не только в одиночном прогоне
		{name: "журнал", shows: "⟦Git commands⟧", gitRows: regexp.MustCompile(`\d\d:\d\d:\d\d`), screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			model = pressAndSettle(model, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
			return openAsync(model, key('@'))
		}},
		{name: "недавние", shows: "⟦Recent repositories⟧", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			return openAsync(model, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
		}},
		{name: "меню bisect", shows: "is bad⟧", screen: func(t *testing.T) tea.Model {
			_, model := withChanges(t)
			model, _ = openAsync(model, key('l')).Update(key('b'))
			return model
		}},
		{name: "меню операции", shows: "⟦Reset bisect⟧", screen: func(t *testing.T) tea.Model {
			repo := pseudoRepo(t)
			runGit(t, repo, "bisect", "start")
			runGit(t, repo, "bisect", "bad")
			model, _ := openPseudo(t, repo).Update(key('M'))
			return model
		}},
		{name: "вопрос undo", shows: "⟦Undo commit", screen: func(t *testing.T) tea.Model {
			repo := pseudoRepo(t)
			runGit(t, repo, "commit", "-q", "--allow-empty", "-m", "2")
			return pressAndSettle(openPseudo(t, repo), key('u'))
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			screen := screenOf(tc.screen(t))
			if tc.gitRows != nil {
				screen = slices.DeleteFunc(screen, tc.gitRows.MatchString)
			}

			if body := strings.Join(screen, "\n"); !strings.Contains(body, tc.shows) {
				t.Errorf("на экране нет %q:\n%s", tc.shows, body)
			}

			if found := untranslated(screen); len(found) > 0 {
				t.Errorf("текст вне словаря %q:\n%s", found, strings.Join(screen, "\n"))
			}
		})
	}
}

// pathOnScreen это путь worktree: его пишем не мы, а временный каталог теста
// полон слов.
var pathOnScreen = regexp.MustCompile(`/\S+`)

// Окно worktree, поле поиска в списках и их надписи о пустом поиске берут тексты
// из словаря. Запрос из тильд не буквы, путь worktree убирается из экрана.
func TestListWindowsUseDictionary(t *testing.T) {
	repo := pseudoRepo(t)
	runGit(t, repo, "worktree", "add", "-q", "-b", "2", filepath.Join(t.TempDir(), "0"))

	for _, tc := range []struct {
		name  string
		open  rune
		typed string
		// shows это текст словаря, который стоит на экране: без него окно могло
		// не открыться, а экран под ним целиком из словаря
		shows string
	}{
		{name: "worktree", open: 'W', shows: "⟦Worktrees⟧"},
		{name: "поле поиска worktree", open: 'W', typed: "/", shows: "⟦done⟧"},
		{name: "worktree без совпадений", open: 'W', typed: "/~~", shows: "⟦No matches⟧"},
		{name: "поле поиска лога", open: 'l', typed: "/", shows: "⟦done⟧"},
		{name: "лог без совпадений", open: 'l', typed: "/~~", shows: "⟦No matching commits⟧"},
		{name: "поле поиска веток", open: 'B', typed: "/", shows: "⟦done⟧"},
		{name: "ветки без совпадений", open: 'B', typed: "/~~", shows: "⟦No matches⟧"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := typeText(openAsync(openPseudo(t, repo), key(tc.open)), tc.typed)

			text := pathOnScreen.ReplaceAllString(screenText(model), "")

			if !strings.Contains(text, tc.shows) {
				t.Errorf("на экране нет %q:\n%s", tc.shows, text)
			}
			if found := untranslated(strings.Split(text, "\n")); len(found) > 0 {
				t.Errorf("текст вне словаря %q:\n%s", found, text)
			}
		})
	}
}

// Служебные строки диффа: переименование без правок, бинарный файл, смена
// прав и строка без перевода в конце.
func TestDiffNoticesUseDictionary(t *testing.T) {
	repo := pseudoRepo(t)
	write(t, repo, "4", "4\n")
	write(t, repo, "5", string([]byte{0, 1, 2}))
	write(t, repo, "6", "6\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "2")

	runGit(t, repo, "mv", "4", "7")
	write(t, repo, "5", string([]byte{0, 1, 3}))
	if err := os.Chmod(filepath.Join(repo, "6"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, repo, "2", "2\n8")

	model := openPseudo(t, repo)
	next := key(']')
	for range 4 {
		screen := screenOf(model)
		if found := untranslated(screen); len(found) > 0 {
			t.Errorf("текст вне словаря %q:\n%s", found, strings.Join(screen, "\n"))
		}
		model, _ = model.Update(next)
	}
}

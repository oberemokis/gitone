package tui_test

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// cleanRepo это репозиторий с одним коммитом и без правок в рабочем дереве,
// открытый на экране width на height: «No changes» и клавиши окон под ним.
func cleanRepo(t *testing.T, keybinds string, width, height int) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "subject-clean")

	opts := options(repo, layout.Focus())
	if keybinds != "" {
		opts.Keys = loadKeys(t, keybinds)
	}

	return repo, resize(open(t, opts), width, height)
}

// Под «No changes» после пустой строки клавиши окон колонкой по центру, как
// подсказки подвала: клавиша и подпись. «No changes» стоит там же, где стоял
// без списка, в середине тела.
func TestNoChangesListsWindowKeys(t *testing.T) {
	_, model := cleanRepo(t, "", 100, 30)
	screen := screenOf(model)

	// тело идёт со строки 3 по 26, середина 14
	if got := strings.TrimSpace(screen[14]); got != "No changes" {
		t.Fatalf("в середине тела %q, want No changes:\n%s", got, screenText(model))
	}

	want := []string{"ctrl p files", "l log", "B branches", "W worktrees", "Z stash", "? help", "q quit"}
	var got []string
	for _, line := range screen[16:23] {
		got = append(got, strings.TrimSpace(line))
	}
	if !slices.Equal(got, want) {
		t.Fatalf("под текстом %q, want %q:\n%s", got, want, screenText(model))
	}

	// самая широкая строка «ctrl p files» стоит по центру, остальные по её левому краю
	for _, hint := range want {
		if left := column(screen[16+slices.Index(want, hint)], hint); left != 44 {
			t.Errorf("%q начинается в колонке %d, want 44", hint, left)
		}
	}
}

// Клик по строке «l log» открывает лог, как клавиша l.
func TestClickNoChangesHintOpensLog(t *testing.T) {
	_, model := cleanRepo(t, "", 100, 30)

	model = clickOn(t, model, "l log")

	if screen := screenText(model); !strings.Contains(screen, "subject-clean") {
		t.Fatalf("клик по «l log» не открыл лог:\n%s", screen)
	}
}

// Отключённое действие в списке отсутствует, остальные на месте.
func TestNoChangesSkipsDisabledAction(t *testing.T) {
	_, model := cleanRepo(t, `{"keybinds":{"app.log":"none"}}`, 100, 30)
	screen := screenOf(model)

	// без лога вторая строка списка ветки
	if got := strings.TrimSpace(screen[17]); got != "B branches" {
		t.Fatalf("вторая строка списка %q, want B branches:\n%s", got, screenText(model))
	}
	if strings.Contains(screenText(model), "log") {
		t.Fatalf("отключённый лог остался на экране:\n%s", screenText(model))
	}
}

// Тело высотой 5: надпись на месте, список обрезан по телу и не заходит в
// подвал.
func TestNoChangesListIsCutByBodyHeight(t *testing.T) {
	_, model := cleanRepo(t, "", 100, 11)
	screen := screenOf(model)

	if len(screen) != 11 {
		t.Fatalf("строк %d, want 11:\n%s", len(screen), screenText(model))
	}
	// тело идёт со строки 3 по 7, надпись в середине, под ней пустая строка и
	// единственная строка списка
	if got := strings.TrimSpace(screen[5]); got != "No changes" {
		t.Fatalf("в середине тела %q, want No changes:\n%s", got, screenText(model))
	}
	if got := strings.TrimSpace(screen[7]); got != "ctrl p files" {
		t.Fatalf("последняя строка тела %q, want ctrl p files:\n%s", got, screenText(model))
	}
	if strings.Contains(screenText(model), "l log") {
		t.Fatalf("список вышел за тело:\n%s", screenText(model))
	}
}

// Появились изменения: экран «No changes» исчезает вместе со списком.
func TestNoChangesListGoesWithChanges(t *testing.T) {
	repo, model := cleanRepo(t, "", 100, 30)
	if !strings.Contains(screenText(model), "ctrl p files") {
		t.Fatalf("списка нет до изменений:\n%s", screenText(model))
	}

	write(t, repo, "a.txt", "two\n")
	screen := screenText(pressReload(t, model))

	if strings.Contains(screen, "No changes") || strings.Contains(screen, "ctrl p files") {
		t.Fatalf("после изменений остались надпись или список:\n%s", screen)
	}
}

// Клавиши берутся из keymap: переназначенный лог показан новой клавишей.
func TestNoChangesShowsRebindKey(t *testing.T) {
	_, model := cleanRepo(t, `{"keybinds":{"app.log":"ctrl+l"}}`, 100, 30)

	// лог вторая строка списка
	if got := strings.TrimSpace(screenOf(model)[17]); got != "ctrl l log" {
		t.Fatalf("вторая строка списка %q, want ctrl l log:\n%s", got, screenText(model))
	}
}

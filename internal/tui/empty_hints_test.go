package tui_test

import (
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
)

// cleanRepo это репозиторий с одним коммитом и без правок в рабочем дереве,
// открытый на экране width на height: «No changes» и ряд клавиш окон под ним.
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

// Под «No changes» после двух пустых строк один ряд клавиш окон по центру
// тела, как подсказки подвала: клавиша и подпись, между ними три пробела.
// «No changes» стоит там же, где стоял без ряда, в середине тела. Клавиши,
// которые подвал уже показывает (B, W, ?, q), в ряд не идут: W виден на экране
// один раз (#400).
func TestNoChangesShowsWindowKeysInOneRow(t *testing.T) {
	_, model := cleanRepo(t, "", 100, 30)
	screen := screenOf(model)

	// тело идёт со строки 3 по 26, середина 14
	if got := strings.TrimSpace(screen[14]); got != "No changes" {
		t.Fatalf("в середине тела %q, want No changes:\n%s", got, screenText(model))
	}
	if got := strings.TrimSpace(strings.Join(screen[15:17], "")); got != "" {
		t.Fatalf("между надписью и рядом %q, want две пустые строки:\n%s", got, screenText(model))
	}

	want := "ctrl p files   l log   Z stash"
	if got := strings.TrimSpace(screen[17]); got != want {
		t.Fatalf("ряд %q, want %q:\n%s", got, want, screenText(model))
	}
	// ряд шириной 30 в теле шириной 100: свободно 70 колонок, слева 35
	if left := column(screen[17], "ctrl p files"); left != 35 {
		t.Errorf("ряд начинается в колонке %d, want 35", left)
	}

	if rest := strings.TrimSpace(strings.Join(screen[18:27], "")); rest != "" {
		t.Errorf("под рядом %q, want пусто:\n%s", rest, screenText(model))
	}
	body := strings.Join(screen[3:27], "\n")
	for _, absent := range []string{"B branches", "W worktrees", "? help", "q quit"} {
		if strings.Contains(body, absent) {
			t.Errorf("в теле осталась подсказка подвала %q:\n%s", absent, screenText(model))
		}
	}
}

// Клик по подсказке ряда нажимает её клавишу и открывает своё окно.
func TestClickNoChangesHintOpensWindow(t *testing.T) {
	tests := []struct{ hint, opens string }{
		{hint: "ctrl p files", opens: "Files"},
		{hint: "l log", opens: "subject-clean"},
		{hint: "Z stash", opens: "Stash"},
	}

	for _, tc := range tests {
		t.Run(tc.hint, func(t *testing.T) {
			_, model := cleanRepo(t, "", 100, 30)

			model = clickOn(t, model, tc.hint)

			if screen := screenText(model); !strings.Contains(screen, tc.opens) {
				t.Fatalf("клик по %q не открыл окно с %q:\n%s", tc.hint, tc.opens, screen)
			}
		})
	}
}

// Область нажатия шире текста (#134): клик на клетку вокруг подсказки, в
// пробел между подсказками и в пустую строку над рядом, тоже нажимает её.
func TestClickAroundNoChangesHintOpensWindow(t *testing.T) {
	tests := []struct {
		name   string
		dx, dy int
	}{
		{name: "слева", dx: -1},
		{name: "справа", dx: len("l log")},
		{name: "сверху", dy: -1},
		{name: "снизу", dy: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, model := cleanRepo(t, "", 100, 30)
			x, y := locate(t, model, "l log")

			model = clickAt(model, x+tc.dx, y+tc.dy)

			if screen := screenText(model); !strings.Contains(screen, "subject-clean") {
				t.Fatalf("клик рядом с «l log» не открыл лог:\n%s", screen)
			}
		})
	}
}

// Отключённое действие в ряду отсутствует, у остальных между подсказками те
// же три пробела.
func TestNoChangesSkipsDisabledAction(t *testing.T) {
	_, model := cleanRepo(t, `{"keybinds":{"app.log":"none"}}`, 100, 30)
	screen := screenOf(model)

	want := "ctrl p files   Z stash"
	if got := strings.TrimSpace(screen[17]); got != want {
		t.Fatalf("ряд %q, want %q:\n%s", got, want, screenText(model))
	}
	if strings.Contains(strings.Join(screen[3:27], "\n"), "log") {
		t.Fatalf("отключённый лог остался в теле:\n%s", screenText(model))
	}
}

// Клавиши берутся из keymap: переназначенный лог показан новой клавишей.
func TestNoChangesShowsRebindKey(t *testing.T) {
	_, model := cleanRepo(t, `{"keybinds":{"app.log":"ctrl+l"}}`, 100, 30)

	want := "ctrl p files   ctrl l log   Z stash"
	if got := strings.TrimSpace(screenOf(model)[17]); got != want {
		t.Fatalf("ряд %q, want %q:\n%s", got, want, screenText(model))
	}
}

// На узком теле подсказки отбрасываются с конца, пока ряд не поместится в
// тело, а надпись видна всегда.
func TestNoChangesRowDropsHintsFromTheEnd(t *testing.T) {
	tests := []struct {
		width int
		row   string
	}{
		{width: 30, row: "ctrl p files   l log   Z stash"},
		{width: 29, row: "ctrl p files   l log"},
		{width: 20, row: "ctrl p files   l log"},
		{width: 19, row: "ctrl p files"},
		{width: 11, row: ""},
	}

	for _, tc := range tests {
		t.Run("ширина "+strconv.Itoa(tc.width), func(t *testing.T) {
			_, model := cleanRepo(t, "", tc.width, 30)
			screen := screenOf(model)

			if got := strings.TrimSpace(screen[14]); got != "No changes" {
				t.Fatalf("в середине тела %q, want No changes:\n%s", got, screenText(model))
			}
			if got := strings.TrimSpace(screen[17]); got != tc.row {
				t.Fatalf("ряд %q, want %q:\n%s", got, tc.row, screenText(model))
			}
			if got := ansi.StringWidth(screen[17]); got != tc.width {
				t.Errorf("строка ряда шириной %d, want ширина тела %d", got, tc.width)
			}
		})
	}
}

// Тело высотой 6 строк вмещает надпись и ряд под ней, а на 5 строках ряд
// уходит за тело и не заходит в подвал. Надпись остаётся в середине.
func TestNoChangesRowIsCutByBodyHeight(t *testing.T) {
	tests := []struct {
		name   string
		height int
		row    string
	}{
		{name: "ряд помещается", height: 12, row: "ctrl p files   l log   Z stash"},
		{name: "ряд не помещается", height: 11, row: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, model := cleanRepo(t, "", 100, tc.height)
			screen := screenOf(model)

			if len(screen) != tc.height {
				t.Fatalf("строк %d, want %d:\n%s", len(screen), tc.height, screenText(model))
			}
			// тело идёт со строки 3, надпись в его середине, ряд на последней
			// строке шестистрочного тела
			if got := strings.TrimSpace(screen[5]); got != "No changes" {
				t.Fatalf("в середине тела %q, want No changes:\n%s", got, screenText(model))
			}
			// тело кончается за три строки до конца экрана: пустая строка, подвал
			// и ещё пустая
			if got := strings.TrimSpace(strings.Join(screen[6:tc.height-3], "\n")); got != tc.row {
				t.Fatalf("под надписью %q, want %q:\n%s", got, tc.row, screenText(model))
			}
		})
	}
}

// Появились изменения: экран «No changes» исчезает вместе с рядом.
func TestNoChangesRowGoesWithChanges(t *testing.T) {
	repo, model := cleanRepo(t, "", 100, 30)
	if !strings.Contains(screenText(model), "ctrl p files") {
		t.Fatalf("ряда нет до изменений:\n%s", screenText(model))
	}

	write(t, repo, "a.txt", "two\n")
	screen := screenText(pressReload(t, model))

	if strings.Contains(screen, "No changes") || strings.Contains(screen, "ctrl p files") {
		t.Fatalf("после изменений остались надпись или ряд:\n%s", screen)
	}
}

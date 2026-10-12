package tui_test

import (
	"strings"
	"testing"

	"gitone/internal/config"
	"gitone/internal/layout"
)

// uiSettings берёт встроенные настройки и меняет в них блок ui.
func uiSettings(change func(ui *config.UI)) config.Config {
	cfg := config.Default()
	change(&cfg.UI)
	return cfg
}

func openWithUI(t *testing.T, repo string, cfg config.Config, notice string) []string {
	t.Helper()

	opts := options(repo, layout.Focus())
	opts.Config, opts.Notice = cfg, notice
	return render(open(t, opts), 100, 20)
}

// Заголовок с путём скрыт, и тело начинается с первой строки диффа.
func TestPaneTitlesOffHidesHeader(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\ntwo\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\nTWO\n")

	cfg := uiSettings(func(ui *config.UI) { ui.PaneTitles = false })
	screen := openWithUI(t, repo, cfg, "")

	if body := strings.Join(screen, "\n"); strings.Contains(body, "a.txt +") {
		t.Fatalf("заголовок остался на экране:\n%s", body)
	}
	if !strings.Contains(screen[0], "one") {
		t.Fatalf("первая строка %q, ожидалась строка диффа", screen[0])
	}
}

// Без заголовка тело на два ряда выше: в нём помещаются две лишние строки
// диффа.
func TestPaneTitlesOffFreesTwoRows(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "f.txt", numberedLines(60))

	countBody := func(screen []string) int {
		count := 0
		for _, line := range screen[:len(screen)-1] {
			if strings.Contains(line, "l") {
				count++
			}
		}
		return count
	}

	with := countBody(openWithUI(t, repo, config.Default(), ""))
	without := countBody(openWithUI(t, repo, uiSettings(func(ui *config.UI) { ui.PaneTitles = false }), ""))

	if without-with != 2 {
		t.Fatalf("строк тела без заголовка %d, с заголовком %d, ожидалась разница 2", without, with)
	}
}

// Подсказок нет, но ошибка в подвале видна: без неё пользователь не узнает о
// сбое.
func TestKeyHintsOffKeepsStatusLine(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	cfg := uiSettings(func(ui *config.UI) { ui.KeyHints = false })

	screen := openWithUI(t, repo, cfg, "")
	if footer := screen[len(screen)-1]; strings.Contains(footer, "quit") {
		t.Fatalf("подсказки остались в подвале: %q", footer)
	}

	screen = openWithUI(t, repo, cfg, "config.json: ошибка в настройках")
	if footer := screen[len(screen)-1]; !strings.Contains(footer, "ошибка в настройках") {
		t.Fatalf("ошибка не видна в подвале: %q", footer)
	}
}

// Без номеров строка диффа начинается с текста, а не с номера.
func TestLineNumbersOffDropsColumn(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\ntwo\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\nTWO\n")

	with := openWithUI(t, repo, config.Default(), "")
	if got := strings.TrimSpace(with[2]); !strings.HasPrefix(got, "1") {
		t.Fatalf("с номерами строка %q, ожидался номер в начале", with[2])
	}

	without := openWithUI(t, repo, uiSettings(func(ui *config.UI) { ui.LineNumbers = false }), "")
	if got := strings.TrimSpace(without[2]); !strings.HasPrefix(got, "one") {
		t.Fatalf("без номеров строка %q, ожидался текст в начале", without[2])
	}
}

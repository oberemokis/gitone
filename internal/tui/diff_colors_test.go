package tui_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

// rawLine ищет строку сырого вывода с текстом text: screenOf цвета срезает.
func rawLine(t *testing.T, raw, text string) string {
	t.Helper()

	for _, line := range strings.Split(raw, "\n") {
		if strings.Contains(line, text) {
			return line
		}
	}

	t.Fatalf("строки с %q нет в выводе:\n%q", text, raw)
	return ""
}

// Номер и знак изменённой строки идут цветом добавления или удаления на своём
// фоне, светлее фона кода. У строки контекста номер приглушённый. Цвета
// темы заданы отличимыми: #010101 в ANSI это 48;2;1;1;1.
func TestChangedLineGutterColors(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "keep\nold\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "keep\nnew\n")

	custom := theme.Dark()
	custom.DiffAdded = lipgloss.Color("#020202")
	custom.DiffRemoved = lipgloss.Color("#040404")
	custom.DiffAddedBg = lipgloss.Color("#060606")
	custom.DiffRemovedBg = lipgloss.Color("#080808")
	custom.DiffAddedNumberBg = lipgloss.Color("#0a0a0a")
	custom.DiffRemovedNumberBg = lipgloss.Color("#0c0c0c")

	opts := options(repo, layout.Focus())
	opts.Theme = custom
	opts.Config.Diff.Mode = "unified"
	raw := resize(open(t, opts), 100, 20).View().Content

	tests := []struct {
		name, text, marker, fg, numberBg, lineBg string
	}{
		{name: "добавленная", text: "new", marker: "+", fg: "38;2;2;2;2", numberBg: "48;2;10;10;10", lineBg: "48;2;6;6;6"},
		{name: "удалённая", text: "old", marker: "-", fg: "38;2;4;4;4", numberBg: "48;2;12;12;12", lineBg: "48;2;8;8;8"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			line := rawLine(t, raw, tc.text)
			marker, code := strings.Index(line, tc.marker), strings.Index(line, tc.text)
			fg, numberBg, lineBg := strings.Index(line, tc.fg), strings.Index(line, tc.numberBg), strings.Index(line, tc.lineBg)

			if fg < 0 || fg > marker || numberBg < 0 || numberBg > marker {
				t.Errorf("номер и знак без цвета %s на фоне %s: %q", tc.fg, tc.numberBg, line)
			}
			if lineBg < marker || lineBg > code {
				t.Errorf("код без фона строки %s: %q", tc.lineBg, line)
			}
		})
	}

	if line := rawLine(t, raw, "keep"); strings.Contains(line, "38;2;2;2;2") || strings.Contains(line, "38;2;4;4;4") {
		t.Errorf("номер строки контекста окрашен как изменение: %q", line)
	}
}

// Встроенная тема: фон номеров светлее фона кода, у добавления и удаления.
func TestDarkGutterLighterThanLine(t *testing.T) {
	dark := theme.Dark()

	pairs := []struct {
		name         string
		number, line [3]uint32
	}{
		{name: "добавление", number: rgb(dark.DiffAddedNumberBg), line: rgb(dark.DiffAddedBg)},
		{name: "удаление", number: rgb(dark.DiffRemovedNumberBg), line: rgb(dark.DiffRemovedBg)},
	}

	for _, pair := range pairs {
		if sum(pair.number) <= sum(pair.line) {
			t.Errorf("%s: фон номеров %v не светлее фона строки %v", pair.name, pair.number, pair.line)
		}
	}
}

func rgb(c interface{ RGBA() (r, g, b, a uint32) }) [3]uint32 {
	r, g, b, _ := c.RGBA()
	return [3]uint32{r, g, b}
}

func sum(channels [3]uint32) uint32 {
	return channels[0] + channels[1] + channels[2]
}

// Над заголовком пустая строка, счётчики строк через пробел.
func TestHeaderSpacing(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "old\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "new\n")

	screen := render(open(t, options(repo, layout.Focus())), 100, 20)

	if strings.TrimSpace(screen[0]) != "" {
		t.Errorf("над заголовком не пусто: %q", screen[0])
	}
	if header := strings.TrimSpace(screen[1]); header != "a.txt +1 -1 (1/1)" {
		t.Errorf("заголовок %q, want %q", header, "a.txt +1 -1 (1/1)")
	}
}

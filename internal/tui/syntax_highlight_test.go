package tui_test

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"gitone/internal/config"
	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

// rawWith открывает файл с правкой и возвращает сырой вывод экрана.
func rawWith(t *testing.T, name, old, new string, palette theme.Theme) string {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, name, old)
	commitAll(t, repo)
	write(t, repo, name, new)

	opts := options(repo, layout.Focus())
	opts.Theme = palette
	return resize(open(t, opts), 100, 20).View().Content
}

// Ключевое слово в Go красится цветом syntaxKeyword прямо перед словом в строке
// диффа. #0a0b0c в ANSI это 38;2;10;11;12. Для файла без языка такого цвета в
// выводе быть не должно.
func TestKeywordUsesSyntaxColor(t *testing.T) {
	palette := theme.Dark()
	palette.SyntaxKeyword = lipgloss.Color("#0a0b0c")

	raw := rawWith(t, "main.go", "package old\n", "package fresh\n", palette)
	for _, marker := range []string{"old", "fresh"} {
		if got := styleBefore(t, raw, marker, "package"); !strings.Contains(got, "38;2;10;11;12") {
			t.Errorf("перед package в строке с %s стоит %q, want цвет ключевого слова", marker, got)
		}
	}
	if raw := rawWith(t, "notes.txt", "one\n", "two\n", palette); strings.Contains(raw, "38;2;10;11;12") {
		t.Fatalf("у файла без языка есть цвет синтаксиса")
	}
}

// Новые виды токенов красятся своими цветами темы, и подсветка не гасит фон
// изменённой строки: функция в добавленной строке идёт цветом syntaxFunction
// на фоне diffAddedBg. #0d0e0f в ANSI это 38;2;13;14;15, #010203 это
// 48;2;1;2;3.
func TestFunctionColorKeepsLineBackground(t *testing.T) {
	palette := theme.Dark()
	palette.SyntaxFunction = lipgloss.Color("#0d0e0f")
	palette.DiffAddedBg = lipgloss.Color("#010203")

	raw := rawWith(t, "main.go", "package main\n", "package main\n\nfunc readDiff() {}\n", palette)

	line := rawLine(t, raw, "readDiff")
	color := strings.Index(line, "38;2;13;14;15")
	if color < 0 || color > strings.Index(line, "readDiff") {
		t.Fatalf("имя функции без цвета syntaxFunction: %q", line)
	}
	if code := styleBefore(t, raw, "readDiff", "readDiff"); !strings.Contains(code, "48;2;1;2;3") {
		t.Fatalf("у функции пропал фон добавленной строки: %q", code)
	}
}

// Ключи новых видов читаются из файла темы.
func TestThemeFileSetsSyntaxKinds(t *testing.T) {
	file := config.ThemeFile{Colors: map[string]config.Color{
		"syntaxFunction": "#010101",
		"syntaxType":     "#020202",
		"syntaxConstant": "#030303",
		"syntaxProperty": "#040404",
		"syntaxOperator": "#050505",
		"syntaxTag":      "#060606",
	}}

	palette, err := theme.Build(file)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := map[string]color.Color{
		"syntaxFunction": palette.SyntaxFunction,
		"syntaxType":     palette.SyntaxType,
		"syntaxConstant": palette.SyntaxConstant,
		"syntaxProperty": palette.SyntaxProperty,
		"syntaxOperator": palette.SyntaxOperator,
		"syntaxTag":      palette.SyntaxTag,
	}
	for name, want := range file.Colors {
		if got[name] != lipgloss.Color(string(want)) {
			t.Errorf("%s = %v, ожидался %s из файла", name, got[name], want)
		}
	}
}

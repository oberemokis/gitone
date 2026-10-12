package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/config"
	"gitone/internal/keymap"
	"gitone/internal/layout"
	"gitone/internal/tui"
	"gitone/internal/tui/theme"
)

// rawWith открывает файл с правкой и возвращает сырой вывод экрана.
func rawWith(t *testing.T, name, old, new string, palette theme.Theme) string {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, name, old)
	commitAll(t, repo)
	write(t, repo, name, new)

	files, err := tui.Load(repo, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	opts := tui.Options{Config: config.Default(), Keys: keymap.Default(), Theme: palette, Layout: layout.Focus()}
	model, _ := tui.New(opts, files).Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	return model.View().Content
}

// Ключевое слово в Go красится цветом syntaxKeyword. #0a0b0c в ANSI это
// 38;2;10;11;12. Для файла без языка такого цвета в выводе быть не должно.
func TestKeywordUsesSyntaxColor(t *testing.T) {
	palette := theme.Dark()
	palette.SyntaxKeyword = lipgloss.Color("#0a0b0c")

	if raw := rawWith(t, "main.go", "package main\n", "package other\n", palette); !strings.Contains(raw, "38;2;10;11;12") {
		t.Fatalf("цвет ключевого слова не попал в вывод:\n%q", raw)
	}
	if raw := rawWith(t, "notes.txt", "one\n", "two\n", palette); strings.Contains(raw, "38;2;10;11;12") {
		t.Fatalf("у файла без языка есть цвет синтаксиса")
	}
}

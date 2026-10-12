package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/config"
	"gitone/internal/layout"
	"gitone/internal/tui"
)

// Настройки из config.json доходят до экрана: режим и порог ширины, с которой
// auto выбирает split.
func TestSettingsReachTheDiffScreen(t *testing.T) {
	withDiff := func(mode string, threshold int) config.Config {
		cfg := config.Default()
		cfg.Diff.Mode = mode
		cfg.Diff.AutoSplitMinWidth = threshold
		return cfg
	}

	tests := []struct {
		name      string
		settings  config.Config
		width     int
		wantSplit bool
	}{
		{name: "split из настроек на узком экране", settings: withDiff("split", 120), width: 80, wantSplit: true},
		{name: "unified из настроек на широком экране", settings: withDiff("unified", 120), width: 150, wantSplit: false},
		{name: "порог из настроек", settings: withDiff("auto", 80), width: 100, wantSplit: true},
		{name: "порог по умолчанию", settings: withDiff("auto", 120), width: 100, wantSplit: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			write(t, repo, "a.txt", "one\ntwo\n")
			commitAll(t, repo)
			write(t, repo, "a.txt", "one\nTWO\n")

			opts := options(repo, layout.Focus())
			opts.Config = tc.settings
			model := resize(open(t, opts), tc.width, 20)

			if got := isSplit(screenOf(model), "two", "TWO"); got != tc.wantSplit {
				t.Fatalf("split %v, ожидалось %v\n%s", got, tc.wantSplit, strings.Join(screenOf(model), "\n"))
			}
		})
	}
}

// Ошибка настроек стоит в подвале, как ошибка git при r, и уходит на первом
// нажатии. Приложение при этом запускается на встроенных значениях.
func TestSettingsErrorShowsInFooterUntilKey(t *testing.T) {
	const notice = "config.json: diff.mode: «wide» не режим"
	opts := options("", layout.Focus())
	opts.Notice = notice
	model := resize(tui.New(opts, nil), 100, 20)

	if footer := footerOf(model); !strings.Contains(footer, "«wide»") {
		t.Fatalf("в подвале %q, ожидался текст ошибки настроек", footer)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if footer := footerOf(model); strings.Contains(footer, "«wide»") || !strings.Contains(footer, "quit") {
		t.Fatalf("после нажатия подвал %q, ожидались подсказки", footer)
	}
}

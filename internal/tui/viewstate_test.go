package tui_test

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/config"
	"gitone/internal/layout"
)

// Вид, переключённый клавишами, переживает перезапуск: после w, m, f, ctrl+w
// и } новый запуск с тем же state.json рисует тот же экран (#369, случаи 1 и
// 2).
func TestViewSurvivesRestart(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "a b\n")
	write(t, repo, "b.txt", numberedLines(30))
	commitAll(t, repo)
	write(t, repo, "a.txt", "a  b\n")
	write(t, repo, "b.txt", strings.Replace(numberedLines(30), "l10\n", strings.Repeat("ten ", 40)+"\n", 1))
	opts := options(repo, layout.Default())
	opts.StateDir = t.TempDir()
	model := resize(open(t, opts), 120, 30)

	for _, press := range []tea.KeyPressMsg{key('w'), key('m'), key('f'), ctrlW, key('}')} {
		model = pressAndSettle(model, press)
	}
	opts.View = config.ReadView(opts.StateDir)
	restarted := resize(open(t, opts), 120, 30)

	want, got := screenOf(model), screenOf(restarted)
	if header := headerText(got); !strings.Contains(header, "whitespace hidden, context 4") {
		t.Fatalf("заголовок %q, ожидался запомненный вид", header)
	}
	// подвал после нажатий называет вид, а после запуска пуст
	if !slices.Equal(got[:footerRow(got)], want[:footerRow(want)]) {
		t.Fatalf("после перезапуска экран\n%s\nожидался\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

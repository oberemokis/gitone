package tui_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/config"
	"gitone/internal/layout"
)

// Запуск записывает репозиторий в недавние; ctrl+r открывает список, выбор
// другого репозитория строит дифф из него; пропавший каталог помечен и не
// открывается, из списка не пропадает (#248, случаи 1–3).
func TestRecentRepos(t *testing.T) {
	stateDir := t.TempDir()

	other := newRepo(t)
	write(t, other, "other.txt", "one\n")
	commitAll(t, other)
	write(t, other, "other.txt", "two\n")
	gone := filepath.Join(t.TempDir(), "gone")
	// корень как его отдаёт git: на macOS временный каталог это ссылка
	for _, path := range []string{gone, runGit(t, other, "rev-parse", "--show-toplevel")} {
		if err := config.RememberRepo(stateDir, path); err != nil {
			t.Fatal(err)
		}
	}

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	opts := options(repo, layout.Focus())
	opts.StateDir = stateDir
	model := resize(open(t, opts), 120, 20)
	model = settle(model, model.Init(), 2)

	root := runGit(t, repo, "rev-parse", "--show-toplevel")
	if got := config.ReadState(stateDir).RecentRepos; len(got) != 3 || got[0] != root {
		t.Fatalf("недавние %q, ожидался открытый первым", got)
	}

	model = openAsync(model, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	if body := screenText(model); !strings.Contains(body, "Recent repositories") || !strings.Contains(body, "missing") {
		t.Fatalf("окно без пометки пропавшего:\n%s", body)
	}

	// внизу пропавший: подвал говорит причину, список на месте
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	// длинный путь, конец подписи обрезан
	if footer := footerOf(model); !strings.Contains(footer, "gone is gone") {
		t.Fatalf("подвал %q, ожидался отказ", footer)
	}

	model, _ = model.Update(key('k'))
	model = enterKey(model)
	if body := screenText(model); !strings.Contains(body, "other.txt") {
		t.Fatalf("дифф не из другого репозитория:\n%s", body)
	}
	if _, err := os.Stat(gone); !os.IsNotExist(err) {
		t.Fatal("пропавший каталог появился")
	}
	got := config.ReadState(stateDir).RecentRepos
	if want := []string{runGit(t, other, "rev-parse", "--show-toplevel"), root, gone}; !slices.Equal(got, want) {
		t.Fatalf("недавние %q, ожидались %q: открытый первым, пропавший на месте", got, want)
	}
}

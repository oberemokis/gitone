package tui_test

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// branchesWith открывает окно веток репозитория, где кроме main есть ветки
// names.
func branchesWith(t *testing.T, names ...string) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "a\n")
	commitAll(t, repo)
	for _, name := range names {
		runGit(t, repo, "branch", name)
	}

	model := resize(open(t, options(repo, layout.Focus())), 120, 30)
	return repo, openAsync(model, key('B'))
}

// / и набор оставляют в окне веток только подходящие по имени, без учёта
// регистра.
func TestBranchSearchFilters(t *testing.T) {
	_, model := branchesWith(t, "feature-login", "fix-crash", "Feature-ui")

	screen := screenText(typeText(model, "/FEAT"))
	if !strings.Contains(screen, "feature-login") || !strings.Contains(screen, "Feature-ui") {
		t.Fatalf("найденных веток нет:\n%s", screen)
	}
	if strings.Contains(screen, "fix-crash") {
		t.Fatalf("неподходящая ветка осталась:\n%s", screen)
	}
	if !strings.Contains(screen, "/ FEAT") {
		t.Fatalf("строки поиска нет:\n%s", screen)
	}
}

// Пока поле открыто, q и j набираются в запрос.
func TestBranchSearchTakesKeysAsText(t *testing.T) {
	_, model := branchesWith(t, "qj-branch")

	model, _ = model.Update(key('/'))
	for _, char := range "qj" {
		var cmd tea.Cmd
		model, cmd = model.Update(key(char))
		if cmd != nil {
			if _, quits := cmd().(tea.QuitMsg); quits {
				t.Fatalf("%c в поле поиска вышел из приложения", char)
			}
		}
	}

	if screen := screenText(model); !strings.Contains(screen, "/ qj") {
		t.Fatalf("q и j не набрались в поиск:\n%s", screen)
	}
}

// enter заканчивает набор: j ходит по найденному, enter переключает на
// выбранную из найденных ветку.
func TestBranchSearchEnterSwitches(t *testing.T) {
	repo, model := branchesWith(t, "fix-one", "feat-two", "fix-three")

	model = typeText(model, "/fix")
	model, _ = model.Update(enter)
	model, _ = model.Update(key('j'))
	model, cmd := model.Update(enter)
	settle(model, cmd, 3)

	if current := runGit(t, repo, "branch", "--show-current"); current != "fix-three" {
		t.Fatalf("текущая ветка %q, want fix-three", current)
	}
}

// esc в поле снимает фильтр: видны все ветки.
func TestBranchSearchEscapeClears(t *testing.T) {
	_, model := branchesWith(t, "fix-one", "feat-two")

	model = typeText(model, "/fix")
	model, _ = model.Update(escape)

	if screen := screenText(model); !strings.Contains(screen, "feat-two") || strings.Contains(screen, "/ fix") {
		t.Fatalf("esc в поле не снял фильтр:\n%s", screen)
	}
}

// Ничего не нашлось: надпись вместо списка.
func TestBranchSearchNoMatches(t *testing.T) {
	_, model := branchesWith(t, "fix-one")

	if screen := screenText(typeText(model, "/zzz")); !strings.Contains(screen, "No matches") {
		t.Fatalf("нет надписи о пустом поиске:\n%s", screen)
	}
}

// В окне worktree ищется и по пути: часть имени каталога оставляет только его
// дерево, текущего в списке нет.
func TestWorktreeSearchFilters(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "a\n")
	commitAll(t, repo)
	runGit(t, repo, "worktree", "add", "-q", "-b", "alpha", filepath.Join(t.TempDir(), "wt-alpha"))

	model := openAsync(resize(open(t, options(repo, layout.Focus())), 120, 30), key('W'))
	screen := screenText(typeText(model, "/wt-al"))

	if !strings.Contains(screen, "alpha") || strings.Contains(screen, "(current)") {
		t.Fatalf("поиск по пути не оставил только wt-alpha:\n%s", screen)
	}
}

// list.search переназначается для всех списков сразу.
func TestRemappedListSearch(t *testing.T) {
	repo, _ := branchesWith(t, "fix-one", "feat-two")
	model := openWithKeybinds(t, repo, `{"keybinds":{"list.search":"s"}}`, 120)

	for _, window := range []rune{'l', 'B', 'W'} {
		opened := typeText(openAsync(model, key(window)), "s")
		if screen := screenText(opened); !strings.Contains(screen, "/ ") {
			t.Fatalf("s не открыл поиск в окне %c:\n%s", window, screen)
		}
	}
}

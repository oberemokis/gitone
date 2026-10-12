package tui_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// Лог, пришедший, когда уже открыт поиск, не перехватывает ввод: набранное
// уходит в поиск, и enter открывает найденный файл.
func TestLateLogDoesNotStealSearch(t *testing.T) {
	_, model := threeFileRepo(t)

	model, logCmd := model.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	model, _ = model.Update(logCmd())

	model = typeText(model, "c")
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	screen := screenOf(model)
	if got := openedFile(screen); got != "c.txt" {
		t.Fatalf("открыт %q, ожидался c.txt:\n%s", got, strings.Join(screen, "\n"))
	}
	if strings.Contains(strings.Join(screen, "\n"), "Log") {
		t.Fatalf("окно лога на экране:\n%s", strings.Join(screen, "\n"))
	}
}

// Список stash, пришедший при открытом коммите, отбрасывается: после esc из
// коммита окна stash нет.
func TestLateStashDoesNotSurfaceAfterCommit(t *testing.T) {
	_, model := threeFileRepo(t)

	model, stashCmd := model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	model, _ = model.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	model, _ = model.Update(stashCmd())
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if body := strings.Join(screenOf(model), "\n"); strings.Contains(body, "Stash") {
		t.Fatalf("после esc всплыло окно stash:\n%s", body)
	}
}

// Ошибка из ответа со списком уходит в подвал и при открытом окне, а окно
// коммита с набранной темой остаётся.
func TestLateLogErrorReachesFooter(t *testing.T) {
	repo, model := threeFileRepo(t)

	model, logCmd := model.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	model, _ = model.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	model = typeText(model, "тема")
	if err := os.RemoveAll(filepath.Join(repo, ".git")); err != nil {
		t.Fatal(err)
	}
	model, _ = model.Update(logCmd())

	screen := screenOf(model)
	if footer := screen[len(screen)-1]; !strings.Contains(footer, "not a git repository") {
		t.Fatalf("в подвале %q, ожидалась ошибка git", footer)
	}
	if !strings.Contains(strings.Join(screen, "\n"), "> тема") {
		t.Fatalf("окно коммита с темой пропало:\n%s", strings.Join(screen, "\n"))
	}
}

// Длинный список stash показывает столько же строк, сколько лог и ветки:
// высота тела минус шесть.
func TestLongStashListUsesCommonMargin(t *testing.T) {
	repo, _ := threeFileRepo(t)
	for index := range 15 {
		write(t, repo, "a.txt", fmt.Sprintf("stash %d\n", index))
		runGit(t, repo, "stash", "push", "-q")
	}

	model := resize(open(t, options(repo, layout.Focus())), 100, 20)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	model, _ = model.Update(cmd())

	shown := strings.Count(strings.Join(screenOf(model), "\n"), "stash@{")
	// высота 20: заголовок с пустой строкой и подвал оставляют телу 17 строк
	if want := 17 - 6; shown != want {
		t.Fatalf("строк stash на экране %d, ожидалось %d", shown, want)
	}
}

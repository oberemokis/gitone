package tui_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// pressReload нажимает r и доводит перечитывание до конца так же, как Bubble
// Tea: команда выполняется, её сообщение возвращается в модель.
func pressReload(t *testing.T, model tea.Model) tea.Model {
	t.Helper()

	model, cmd := model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if cmd == nil {
		t.Fatal("r не запустил перечитывание")
	}
	model, _ = model.Update(cmd())
	return model
}

// numberedLines это count строк l1, l2, … с переводом строки в конце.
func numberedLines(count int) string {
	var content strings.Builder
	for number := 1; number <= count; number++ {
		fmt.Fprintf(&content, "l%d\n", number)
	}
	return content.String()
}

func TestReloadShowsChangedLines(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\ntwo\n")
	model, _ := open(t, repo, nil).Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	write(t, repo, "a.txt", "one\ntwo\nthree\n")
	screen := screenOf(pressReload(t, model))

	if header := strings.TrimSpace(screen[0]); header != "a.txt +2-0" {
		t.Fatalf("заголовок %q, ожидался %q", header, "a.txt +2-0")
	}
	if body := strings.Join(screen, "\n"); !strings.Contains(body, "three") {
		t.Fatalf("новой строки нет на экране:\n%s", body)
	}
}

func TestReloadKeepsOpenFileAndScroll(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "seed\n")
	write(t, repo, "b.txt", "seed\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "a\n")
	write(t, repo, "b.txt", numberedLines(60))
	model, _ := open(t, repo, nil).Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	// открыт второй файл, а не первый: иначе возврат к первому ничего не докажет
	model, _ = model.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	model, _ = model.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	before := topNumber(t, screenOf(model))

	write(t, repo, "b.txt", numberedLines(61))
	screen := screenOf(pressReload(t, model))

	if got := openedFile(screen); got != "b.txt" {
		t.Fatalf("открыт %q, ожидался b.txt:\n%s", got, strings.Join(screen, "\n"))
	}
	if got := topNumber(t, screen); got != before {
		t.Fatalf("первая строка тела %d, ожидалась %d", got, before)
	}
}

func TestReloadOpensFirstFileWhenOpenOneIsGone(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "seed\n")
	write(t, repo, "b.txt", "seed\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "changed a\n")
	write(t, repo, "b.txt", "changed b\n")
	model, _ := open(t, repo, nil).Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	model, _ = model.Update(tea.KeyPressMsg{Code: ']', Text: "]"})

	// откат возвращает b.txt к HEAD, и он выпадает из диффа
	runGit(t, repo, "checkout", "--", "b.txt")
	screen := screenOf(pressReload(t, model))

	if got := openedFile(screen); got != "a.txt" {
		t.Fatalf("открыт %q, ожидался a.txt:\n%s", got, strings.Join(screen, "\n"))
	}
}

func TestSecondReloadWhileLoadingIsIgnored(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	model, _ := open(t, repo, nil).Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	reloadKey := tea.KeyPressMsg{Code: 'r', Text: "r"}

	model, first := model.Update(reloadKey)
	if first == nil {
		t.Fatal("первый r не запустил перечитывание")
	}
	if footer := screenOf(model)[len(screenOf(model))-1]; !strings.Contains(footer, "loading") {
		t.Fatalf("во время загрузки в подвале %q, ожидалось loading", footer)
	}

	_, second := model.Update(reloadKey)
	if second != nil {
		t.Fatal("второй r во время загрузки запустил ещё одно перечитывание")
	}

	model, _ = model.Update(first())
	if footer := screenOf(model)[len(screenOf(model))-1]; strings.Contains(footer, "loading") {
		t.Fatalf("после загрузки в подвале осталось loading: %q", footer)
	}
}

func TestReloadErrorKeepsOldDiffUntilNextKey(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	model, _ := open(t, repo, nil).Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	// без .git git не узнаёт репозиторий, и перечитывание падает
	if err := os.RemoveAll(filepath.Join(repo, ".git")); err != nil {
		t.Fatal(err)
	}
	model = pressReload(t, model)
	screen := screenOf(model)

	if body := strings.Join(screen, "\n"); !strings.Contains(body, "two") {
		t.Fatalf("старый дифф пропал с экрана:\n%s", body)
	}
	if footer := screen[len(screen)-1]; !strings.Contains(footer, "not a git repository") {
		t.Fatalf("в подвале %q, ожидался текст ошибки git", footer)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if footer := screenOf(model)[len(screen)-1]; strings.Contains(footer, "not a git repository") || !strings.Contains(footer, "quit") {
		t.Fatalf("после нажатия подвал %q, ожидались подсказки", footer)
	}
}

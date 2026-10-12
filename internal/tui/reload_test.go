package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

func TestReloadShowsChangedLines(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\ntwo\n")
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	write(t, repo, "a.txt", "one\ntwo\nthree\n")
	screen := screenOf(pressReload(t, model))

	if header := headerText(screen); header != "a.txt +2 -0 (1/1)" {
		t.Fatalf("заголовок %q, ожидался %q", header, "a.txt +2 -0 (1/1)")
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
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

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
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)
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
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)
	reloadKey := tea.KeyPressMsg{Code: 'r', Text: "r"}

	model, first := model.Update(reloadKey)
	if first == nil {
		t.Fatal("первый r не запустил перечитывание")
	}
	if footer := footerOf(model); !strings.Contains(footer, "loading") {
		t.Fatalf("во время загрузки в подвале %q, ожидалось loading", footer)
	}

	_, second := model.Update(reloadKey)
	if second != nil {
		t.Fatal("второй r во время загрузки запустил ещё одно перечитывание")
	}

	for _, msg := range runAll(first) {
		model, _ = model.Update(msg)
	}
	if footer := footerOf(model); strings.Contains(footer, "loading") {
		t.Fatalf("после загрузки в подвале осталось loading: %q", footer)
	}
}

func TestReloadErrorKeepsOldDiffUntilNextKey(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	// без .git git не узнаёт репозиторий, и перечитывание падает
	if err := os.RemoveAll(filepath.Join(repo, ".git")); err != nil {
		t.Fatal(err)
	}
	model = pressReload(t, model)
	screen := screenOf(model)

	if body := strings.Join(screen, "\n"); !strings.Contains(body, "two") {
		t.Fatalf("старый дифф пропал с экрана:\n%s", body)
	}
	if footer := screen[footerRow(screen)]; !strings.Contains(footer, "not a git repository") {
		t.Fatalf("в подвале %q, ожидался текст ошибки git", footer)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if footer := footerOf(model); strings.Contains(footer, "not a git repository") || !strings.Contains(footer, "quit") {
		t.Fatalf("после нажатия подвал %q, ожидались подсказки", footer)
	}
}

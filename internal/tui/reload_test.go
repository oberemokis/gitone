package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// Файл стал короче, пока был прокручен вниз: прокрутка обрезается по новой
// длине, и экран не уходит за конец файла (docs/diff.md, перечитывание).
func TestReloadClampsScrollToShorterFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "seed.txt", "seed\n")
	commitAll(t, repo)
	write(t, repo, "long.txt", numberedLines(60))
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)
	model, _ = model.Update(key('G'))
	if got := topNumber(t, screenOf(model)); got != 47 {
		t.Fatalf("до перечитывания наверху строка %d, want 47", got)
	}

	write(t, repo, "long.txt", numberedLines(30))
	screen := screenOf(pressReload(t, model))

	// тело в 14 строк: от последней строки 30 это строки 17-30
	if got := topNumber(t, screen); got != 17 {
		t.Errorf("после перечитывания наверху строка %d, want 17", got)
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

// lateReply доставляет модели ответ чтения reading (r, enter в логе или в
// stash), которое началось до смены каталога или аргументов. Возвращает экран
// сразу после ответа и модель, дочитавшую новый дифф до конца: пока новое
// чтение не пришло, запоздавший ответ экран менять не должен.
func lateReply(model tea.Model, reading tea.Cmd) (atOnce string, settled tea.Model) {
	// в пакете первым идёт шаг спиннера, вторым ответ git
	reply := runAll(reading)[1]
	model, restart := model.Update(reply)
	return screenText(model), settle(model, restart, 3)
}

// r ещё читает дифф основного дерева, а пользователь уже выбрал соседнее:
// запоздавший ответ про прежнее дерево не ложится на экран. x, y после этого
// трогают файл дерева, которое на экране, а не соседнее.
func TestStaleReloadAfterWorktreeSwitchShowsNewTree(t *testing.T) {
	repo, side, model := twoWorktrees(t)

	model, reading := model.Update(key('r'))
	model = openAsync(model, key('W'))
	model, _ = model.Update(key('j'))
	model, switching := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, switching, 2)

	// чтение успело увидеть правку, которой на экране ещё нет
	write(t, repo, "a.txt", "main-late\n")
	before := screenText(model)
	atOnce, model := lateReply(model, reading)

	if atOnce != before {
		t.Fatalf("запоздавший ответ изменил экран:\n%s", atOnce)
	}
	if screen := screenText(model); !strings.Contains(screen, "side-edit") || strings.Contains(screen, "main-") {
		t.Fatalf("на экране не дифф side-tree:\n%s", screen)
	}
	if footer := footerOf(model); !strings.HasPrefix(footer, " B side   W side-tree") {
		t.Fatalf("подвал %q, ожидался side-tree слева", footer)
	}

	model, _ = model.Update(key('x'))
	model, discarding := model.Update(key('y'))
	settle(model, discarding, 3)

	if got := readText(t, filepath.Join(repo, "a.txt")); got != "main-late\n" {
		t.Fatalf("файл основного дерева %q, ожидалась нетронутая правка main-late", got)
	}
	if got := readText(t, filepath.Join(side, "a.txt")); got != "one\n" {
		t.Fatalf("файл side-tree %q, ожидался откат к one", got)
	}
}

// Открыт коммит из лога, его дифф ещё читается, и пользователь жмёт b: ответ с
// диффом коммита на экран не идёт, там дифф рабочего дерева.
func TestStaleCommitDiffAfterBackShowsWorkingTree(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "commit-edit\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "work-edit\n")
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	model = openAsync(model, key('l'))
	model, reading := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model, _ = model.Update(key('b'))

	before := screenText(model)
	atOnce, model := lateReply(model, reading)

	if atOnce != before {
		t.Fatalf("запоздавший дифф коммита изменил экран:\n%s", atOnce)
	}
	if screen := screenText(model); !strings.Contains(screen, "work-edit") {
		t.Fatalf("на экране не дифф запуска:\n%s", screen)
	}
}

// Дифф stash открывают, пока идёт r: ответ с диффом рабочего дерева на экран не
// идёт, там дифф stash.
func TestStaleReloadAfterStashEnterShowsStash(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "stash-edit\n")
	runGit(t, repo, "stash", "push", "-q")
	write(t, repo, "a.txt", "work-edit\n")
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	model, reading := model.Update(key('r'))
	model = openAsync(model, key('Z'))
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	// чтение успело увидеть правку, которой на экране ещё нет
	write(t, repo, "a.txt", "work-late\n")
	before := screenText(model)
	atOnce, model := lateReply(model, reading)

	if atOnce != before {
		t.Fatalf("запоздавший дифф рабочего дерева изменил экран:\n%s", atOnce)
	}
	if screen := screenText(model); !strings.Contains(screen, "stash-edit") || strings.Contains(screen, "work-") {
		t.Fatalf("на экране не дифф stash:\n%s", screen)
	}
}

// r во время чтения не пропадает: git уже прочитал дифф, правка пришла после
// этого, и r показывает её вторым чтением, а не прячет за первым.
func TestReloadPressedWhileLoadingReadsAgain(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	model, first := model.Update(key('r'))
	replies := runAll(first)
	write(t, repo, "a.txt", "three\n")
	model, second := model.Update(key('r'))
	if second != nil {
		t.Fatal("второй r во время загрузки запустил ещё одно перечитывание")
	}
	for _, reply := range replies {
		var again tea.Cmd
		model, again = model.Update(reply)
		model = settle(model, again, 3)
	}

	if screen := screenText(model); !strings.Contains(screen, "three") {
		t.Fatalf("правка после первого чтения не на экране:\n%s", screen)
	}
}

// Шаг спиннера прежней загрузки приходит, когда пробел уже запустил новую: его
// цепочка обрывается, а цепочка новой загрузки идёт дальше. Две цепочки
// крутили бы спиннер вдвое быстрее.
func TestSpinnerTickOfFinishedLoadDoesNotStartSecondChain(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	model, first := model.Update(key('r'))
	// в пакете первым идёт шаг спиннера, вторым ответ git
	oldReply := runAll(first)
	oldTick, oldRead := oldReply[0], oldReply[1]
	model, _ = model.Update(oldRead)

	model, staging := model.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	model, restart := model.Update(runAll(staging)[0])
	newTick := runAll(restart)[0]

	if _, cmd := model.Update(oldTick); cmd != nil {
		t.Error("шаг прежней загрузки продолжил свою цепочку спиннера")
	}
	if _, cmd := model.Update(newTick); cmd == nil {
		t.Error("шаг идущей загрузки не продолжил цепочку спиннера")
	}
}

// openSecondLink открывает дифф, где файл link стал симлинком: две записи с
// одним путём, удалённая «content» и добавленная «target». Открыта вторая.
func openSecondLink(t *testing.T, ticks <-chan time.Time) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "link", "content\n")
	commitAll(t, repo)
	if err := os.Remove(filepath.Join(repo, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(repo, "link")); err != nil {
		t.Fatal(err)
	}

	opts := options(repo, layout.Focus())
	opts.Ticks = ticks
	model := resize(open(t, opts), 100, 20)
	model, _ = model.Update(key(']'))

	return repo, model
}

// Перечитывание ищет открытую запись по подписи и статусу, а не по одному
// пути: на второй записи link остаётся вторая, а не первая.
func TestReloadKeepsSecondEntryOfSamePath(t *testing.T) {
	_, model := openSecondLink(t, nil)

	screen := screenOf(pressReload(t, model))

	if header := headerText(screen); !strings.HasSuffix(header, "(2/2)") {
		t.Fatalf("заголовок %q, ожидалась вторая запись (2/2)", header)
	}
	if body := strings.Join(screen, "\n"); !strings.Contains(body, "target") || strings.Contains(body, "content") {
		t.Fatalf("на экране не добавленная запись link:\n%s", body)
	}
}

// То же для опроса: появился новый файл, дифф изменился, открыта по-прежнему
// вторая запись link.
func TestPollKeepsSecondEntryOfSamePath(t *testing.T) {
	repo, model := openSecondLink(t, oneTick())

	write(t, repo, "z.txt", "zzz\n")
	screen := screenOf(settle(model, model.Init(), 3))

	if header := headerText(screen); !strings.HasSuffix(header, "(2/3)") {
		t.Fatalf("заголовок %q, ожидалась вторая запись из трёх (2/3)", header)
	}
	if body := strings.Join(screen, "\n"); !strings.Contains(body, "target") || strings.Contains(body, "content") {
		t.Fatalf("на экране не добавленная запись link:\n%s", body)
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

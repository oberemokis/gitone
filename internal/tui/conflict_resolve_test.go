package tui_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// conflictRepo это слияние topic в main, остановленное на двух конфликтах в
// a.txt и на конфликте в бинарном b.bin.
func conflictRepo(t *testing.T) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\nx\nx\nx\nx\nx\ntwo\n")
	write(t, repo, "b.bin", "base\x00\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "topic")
	write(t, repo, "a.txt", "ONE topic\nx\nx\nx\nx\nx\nTWO topic\n")
	write(t, repo, "b.bin", "topic\x00\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	write(t, repo, "a.txt", "ONE main\nx\nx\nx\nx\nx\nTWO main\n")
	write(t, repo, "b.bin", "main\x00\n")
	commitAll(t, repo)
	merge := exec.Command("git", "merge", "topic")
	merge.Dir = repo
	if out, err := merge.CombinedOutput(); err == nil {
		t.Fatalf("слияние прошло без конфликта:\n%s", out)
	}

	return repo, resize(open(t, options(repo, layout.Focus())), 100, 30)
}

// chooseMenu выбирает строку меню с текстом label.
func chooseMenu(t *testing.T, model tea.Model, label string) tea.Model {
	t.Helper()

	for range 10 {
		if strings.Contains(markedRow(model), label) {
			return enterKey(model)
		}
		model, _ = model.Update(key('j'))
	}
	t.Fatalf("в меню нет %q:\n%s", label, screenText(model))
	return model
}

// Конфликтные файлы помечены в панели (#242, случай 1). v включает выбор
// конфликтов, space на втором открывает меню, «main» берёт нашу сторону
// этого конфликта (случай 2); после последнего конфликта файл попадает в
// индекс, а слияние ждёт продолжения (случай 4).
func TestResolveConflictHunks(t *testing.T) {
	repo, model := conflictRepo(t)

	if body := screenText(toggleFiles(model)); !strings.Contains(body, "a.txt conflict") {
		t.Fatalf("конфликт не помечен в панели:\n%s", body)
	}

	model = pressAndSettle(model, key('v'))
	model, _ = model.Update(key('j'))
	model = pressAndSettle(model, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	model = chooseMenu(t, model, "HEAD")

	got := readText(t, filepath.Join(repo, "a.txt"))
	if !strings.HasSuffix(got, "x\nTWO main\n") || !strings.HasPrefix(got, "<<<<<<< HEAD\n") {
		t.Fatalf("a.txt = %q, ожидалась решённой только вторая", got)
	}

	model = pressAndSettle(model, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	model = chooseMenu(t, model, "topic")
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "ONE topic\nx\nx\nx\nx\nx\nTWO main\n" {
		t.Fatalf("a.txt = %q после обоих конфликтов", got)
	}
	if unmerged := runGit(t, repo, "ls-files", "--unmerged", "--", "a.txt"); unmerged != "" {
		t.Fatalf("a.txt не в индексе:\n%s", unmerged)
	}
	if footer := footerOf(model); !strings.Contains(footer, "Conflicts in a.txt resolved") {
		t.Fatalf("подвал %q, ожидался итог", footer)
	}
	if head := runGit(t, repo, "rev-parse", "-q", "--verify", "MERGE_HEAD"); head == "" {
		t.Fatal("слияние закончилось само, а должно ждать продолжения")
	}
}

// Наша сторона для файла целиком делает его равным HEAD, и дифф пустеет:
// выбор конфликтов снимается, space после этого не падает (#287, случай 2).
func TestConflictsGoneWithDiff(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "base\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "topic")
	write(t, repo, "a.txt", "topic\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	write(t, repo, "a.txt", "main\n")
	commitAll(t, repo)
	merge := exec.Command("git", "merge", "topic")
	merge.Dir = repo
	if out, err := merge.CombinedOutput(); err == nil {
		t.Fatalf("слияние прошло без конфликта:\n%s", out)
	}
	model := resize(open(t, options(repo, layout.Focus())), 100, 30)

	model = pressAndSettle(model, key('v'))
	model = pressAndSettle(model, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	model = chooseMenu(t, model, "Whole file: HEAD")
	model = pressAndSettle(model, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})

	if body := screenText(model); strings.Contains(body, "▌") {
		t.Fatalf("выбор конфликтов пережил пустой дифф:\n%s", body)
	}
}

// textConflict это слияние topic в main, остановленное на конфликте в a.txt:
// base общий предок, main и topic правки сторон. setup готовит репозиторий до
// первого коммита.
func textConflict(t *testing.T, base, main, topic string, setup func(repo string)) (string, tea.Model) {
	t.Helper()

	repo := newRepo(t)
	if setup != nil {
		setup(repo)
	}
	write(t, repo, "a.txt", base)
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "topic")
	write(t, repo, "a.txt", topic)
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	write(t, repo, "a.txt", main)
	commitAll(t, repo)
	merge := exec.Command("git", "merge", "topic")
	merge.Dir = repo
	if out, err := merge.CombinedOutput(); err == nil {
		t.Fatalf("слияние прошло без конфликта:\n%s", out)
	}

	return repo, resize(open(t, options(repo, layout.Focus())), 100, 30)
}

// markedText это строки экрана, отмеченные выделением.
func markedText(model tea.Model) string {
	var rows []string
	for _, row := range screenOf(model) {
		if strings.Contains(row, "▌") {
			rows = append(rows, row)
		}
	}
	return strings.Join(rows, "\n")
}

// resolvedWith решает конфликт под курсором стороной label и проверяет, что
// файл в индексе с текстом want.
func resolvedWith(t *testing.T, repo string, model tea.Model, label, want string) tea.Model {
	t.Helper()

	model = pressAndSettle(model, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	model = chooseMenu(t, model, label)
	if got := readText(t, filepath.Join(repo, "a.txt")); got != want {
		t.Fatalf("a.txt = %q, ожидалось %q", got, want)
	}
	if unmerged := runGit(t, repo, "ls-files", "--unmerged", "--", "a.txt"); unmerged != "" {
		t.Fatalf("a.txt не в индексе:\n%s", unmerged)
	}
	return model
}

// Маркеры, которые уже лежали в HEAD, файл неслитым не держат: после решения
// настоящего конфликта он в индексе (#302, случай 1).
func TestConflictIgnoresMarkersFromHead(t *testing.T) {
	block := "<<<<<<< HEAD\nold\n=======\nnew\n>>>>>>> doc\n"
	padding := "x\nx\nx\nx\nx\nx\n"
	repo, model := textConflict(t, block+padding+"line\n", block+padding+"main\n", block+padding+"topic\n", nil)

	model = pressAndSettle(model, key('v'))
	resolvedWith(t, repo, model, "HEAD", block+padding+"main\n")
}

// Маркеры длины conflict-marker-size узнаются, выбор по хунку работает
// (случай 2).
func TestConflictMarkerSize(t *testing.T) {
	repo, model := textConflict(t, "one\n", "main\n", "topic\n", func(repo string) {
		write(t, repo, ".gitattributes", "a.txt conflict-marker-size=10\n")
	})

	model = pressAndSettle(model, key('v'))
	if marked := markedText(model); !strings.Contains(marked, "<<<<<<<<<< HEAD") {
		t.Fatalf("выбор конфликтов не включился:\n%s", screenText(model))
	}
	resolvedWith(t, repo, model, "HEAD", "main\n")
}

// Блок без ======= конфликтом не считается: курсор встаёт на настоящий
// конфликт ниже (случай 3).
func TestConflictWithoutMiddleIsSkipped(t *testing.T) {
	repo, model := conflictRepo(t)
	text := readText(t, filepath.Join(repo, "a.txt"))
	write(t, repo, "a.txt", strings.Replace(text, "=======\n", "", 1))
	model = pressReload(t, model)

	model = pressAndSettle(model, key('v'))
	if marked := markedText(model); !strings.Contains(marked, "TWO main") || strings.Contains(marked, "ONE main") {
		t.Fatalf("курсор не на настоящем конфликте:\n%s", marked)
	}
}

// Конфликт в файле с CRLF решается (случай 5); решение первого из двух
// ставит курсор на второй (случай 6).
func TestConflictCRLFAndNext(t *testing.T) {
	repo, model := textConflict(t, "one\r\n", "main\r\n", "topic\r\n", nil)
	model = pressAndSettle(model, key('v'))
	resolvedWith(t, repo, model, "HEAD", "main\r\n")

	_, model = conflictRepo(t)
	model = pressAndSettle(model, key('v'))
	model = pressAndSettle(model, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	model = chooseMenu(t, model, "HEAD")
	if marked := markedText(model); !strings.Contains(marked, "TWO main") {
		t.Fatalf("курсор не на втором конфликте:\n%s", screenText(model))
	}
}

// Бинарный конфликт решается стороной для файла целиком (случаи 3 и 5).
func TestResolveBinaryConflictWhole(t *testing.T) {
	repo, model := conflictRepo(t)

	model = pressAndSettle(model, key(']'))
	model = pressAndSettle(model, key('v'))
	model = chooseMenu(t, model, "Whole file: theirs")

	if got := readText(t, filepath.Join(repo, "b.bin")); got != "topic\x00\n" {
		t.Fatalf("b.bin = %q, ожидалась их сторона", got)
	}
	if unmerged := runGit(t, repo, "ls-files", "--unmerged", "--", "b.bin"); unmerged != "" {
		t.Fatalf("b.bin не в индексе:\n%s", unmerged)
	}
}

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

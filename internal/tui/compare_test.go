package tui_test

import (
	"strings"
	"testing"

	"gitone/internal/layout"
)

// compareRepo это main с правкой a.txt в рабочем дереве и веткой feature, где
// в f.txt свой коммит; открыт дифф рабочего дерева.
func compareRepo(t *testing.T) (string, string) {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "feature")
	write(t, repo, "f.txt", "feature\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	write(t, repo, "a.txt", "two\n")
	return repo, runGit(t, repo, "rev-parse", "--short=7", "HEAD")
}

// W на ветке показывает дифф от её вершины до рабочего дерева, заголовок
// называет точку сравнения; b возвращает к диффу рабочего дерева (#216,
// случаи 1 и 4).
func TestCompareWithBranch(t *testing.T) {
	repo, _ := compareRepo(t)
	model := openAsync(resize(open(t, options(repo, layout.Focus())), 120, 24), key('B'))
	model, _ = model.Update(key('k'))

	// второй файл f.txt есть в feature и нет в рабочем дереве
	model = pressAndSettle(model, key('W'))
	if got := headerText(screenOf(model)); !strings.Contains(got, "(1/2) vs feature") {
		t.Fatalf("заголовок %q, ожидались два файла и сравнение с feature", got)
	}

	model = pressAndSettle(model, key('b'))
	if got := headerText(screenOf(model)); strings.Contains(got, "vs feature") || !strings.Contains(got, "a.txt") {
		t.Fatalf("b не вернул дифф рабочего дерева: %q", got)
	}
}

// W на коммите лога сравнивает с ним (случай 2); вид только для просмотра.
func TestCompareWithCommit(t *testing.T) {
	repo, head := compareRepo(t)
	model := openAsync(resize(open(t, options(repo, layout.Focus())), 120, 24), key('l'))

	model = pressAndSettle(model, key('W'))
	if got := headerText(screenOf(model)); !strings.Contains(got, "a.txt") || !strings.Contains(got, "vs "+head) {
		t.Fatalf("заголовок %q, ожидалось сравнение с %s", got, head)
	}

	model = pressAndSettle(model, space)
	if footer := footerOf(model); !strings.Contains(footer, "read-only") {
		t.Fatalf("подвал %q, ожидался отказ правки в виде для просмотра", footer)
	}
}

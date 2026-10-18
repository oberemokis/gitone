package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

var ctrlW = tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl}

// ctrl+w прячет файл, где поменялись одни пробелы, заголовок говорит о виде;
// второе нажатие возвращает его (#217, случай 1).
func TestToggleWhitespace(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "a b\n")
	write(t, repo, "b.txt", "x\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "a  b\n")
	write(t, repo, "b.txt", "y\n")
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	model = pressAndSettle(model, ctrlW)
	screen := screenOf(model)
	if got := headerText(screen); !strings.Contains(got, "b.txt") || !strings.Contains(got, "(1/1)") || !strings.Contains(got, "whitespace hidden") {
		t.Fatalf("заголовок %q, ожидался один b.txt и вид «whitespace hidden»", got)
	}

	model = pressAndSettle(model, ctrlW)
	// открытый b.txt остаётся открытым, теперь вторым из двух
	if got := headerText(screenOf(model)); !strings.Contains(got, "(2/2)") || strings.Contains(got, "whitespace") {
		t.Fatalf("заголовок %q, ожидались оба файла без вида", got)
	}
}

// } добавляет строку контекста к трём строкам git, { убирает их до нуля: хунки
// без окружения, разрыв между ними на месте (случаи 2 и 3).
func TestChangeContext(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", numberedLines(30))
	commitAll(t, repo)
	write(t, repo, "a.txt", strings.NewReplacer("l10\n", "ten\n", "l25\n", "twenty five\n").Replace(numberedLines(30)))
	model := resize(open(t, options(repo, layout.Focus())), 120, 40)

	if body := screenText(model); strings.Contains(body, " l6 ") {
		t.Fatalf("при трёх строках контекста видна l6:\n%s", body)
	}
	model = pressAndSettle(model, key('}'))
	if body := screenText(model); !strings.Contains(body, " l6 ") || !strings.Contains(headerText(screenOf(model)), "context 4") {
		t.Fatalf("} не добавил строку контекста:\n%s", body)
	}

	for range 5 {
		model = pressAndSettle(model, key('{'))
	}
	body := screenText(model)
	if strings.Contains(body, " l9 ") || !strings.Contains(body, "ten") || !strings.Contains(body, "twenty five") {
		t.Fatalf("при нуле строк контекста видно окружение или пропал хунк:\n%s", body)
	}
	if got := headerText(screenOf(model)); !strings.Contains(got, "context 0") {
		t.Fatalf("заголовок %q, ожидался «context 0»", got)
	}
}

package tui_test

import (
	"fmt"
	"strings"
	"testing"

	"gitone/internal/layout"
)

// Отметки в логе начинают поиск и ведут к первому плохому коммиту, подвал
// говорит, что проверять и что найдено; M во время поиска отмечает HEAD, а
// сброс возвращает main (#245, случаи 1–4).
func TestBisectFromLog(t *testing.T) {
	repo := newRepo(t)
	for number := 1; number <= 8; number++ {
		write(t, repo, "a.txt", fmt.Sprintf("%d\n", number))
		runGit(t, repo, "add", "-A")
		runGit(t, repo, "commit", "-q", "-m", fmt.Sprintf("c%d", number))
	}
	c4 := runGit(t, repo, "rev-parse", "HEAD~4")
	model := resize(open(t, options(repo, layout.Focus())), 120, 40)

	// HEAD плохой
	model = openAsync(model, key('l'))
	model, _ = model.Update(key('b'))
	model = chooseMenu(t, model, "is bad")
	if footer := footerOf(model); !strings.Contains(footer, "mark a commit on the other side") || !strings.Contains(footer, "M bisecting") {
		t.Fatalf("подвал %q, ожидалось ожидание второй отметки", footer)
	}

	// c1 хороший: git переходит на c4
	model = openAsync(model, key('l'))
	for range 7 {
		model, _ = model.Update(key('j'))
	}
	model, _ = model.Update(key('b'))
	model = chooseMenu(t, model, "is good")
	if footer := footerOf(model); !strings.Contains(footer, "Testing "+c4[:7]+" c4, 3 left") {
		t.Fatalf("подвал %q, ожидался коммит для проверки", footer)
	}

	// c4 плохой, c3 хороший: найден c4
	for _, label := range []string{"HEAD is bad", "HEAD is good"} {
		model, _ = model.Update(key('M'))
		model = chooseMenu(t, model, label)
	}
	if footer := footerOf(model); !strings.Contains(footer, "First bad commit: "+c4[:7]+" c4") {
		t.Fatalf("подвал %q, ожидался найденный коммит", footer)
	}

	model, _ = model.Update(key('M'))
	model = chooseMenu(t, model, "Reset bisect")
	if got := runGit(t, repo, "branch", "--show-current"); got != "main" {
		t.Fatalf("после сброса ветка %q", got)
	}
	if footer := footerOf(model); strings.Contains(footer, "bisecting") {
		t.Fatalf("подвал %q, поиск не закончился", footer)
	}
}

package tui_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// hardReset выбирает в меню reset строку hard.
func hardReset(model tea.Model) tea.Model {
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return model
}

// g на текущей ветке ведёт к её upstream: вопрос hard называет, сколько
// локальных коммитов пропадёт, y совмещает ветку и дерево с remote (#231,
// случаи 1 и 3).
func TestResetToUpstream(t *testing.T) {
	origin, repo := remoteRepo(t)
	runGit(t, repo, "commit", "-q", "-a", "-m", "local")

	model := openAsync(resize(open(t, options(repo, layout.Focus())), 120, 24), key('B'))
	model = pressAndSettle(model, key('g'))
	if body := screenText(model); !strings.Contains(body, "Reset the branch to origin/main") {
		t.Fatalf("g не открыл меню reset к upstream:\n%s", body)
	}

	model = hardReset(model)
	if body := screenText(model); !strings.Contains(body, "1 local commit and uncommitted changes are lost") {
		t.Fatalf("вопрос не назвал число коммитов:\n%s", body)
	}
	model = pressAndSettle(model, key('n'))
	if got := runGit(t, repo, "log", "-1", "--format=%s"); got != "local" {
		t.Fatal("reset выполнен без y")
	}

	answerYes(hardReset(pressAndSettle(model, key('g'))))
	if got, want := runGit(t, repo, "rev-parse", "HEAD"), runGit(t, origin, "rev-parse", "main"); got != want {
		t.Fatalf("HEAD %s, ожидался main с origin %s", got, want)
	}
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "one\n" {
		t.Fatalf("a.txt = %q, ожидалось содержимое remote", got)
	}
}

// Вопрос hard называет коммиты в нужном числе, а без них о коммитах молчит
// (#399).
func TestHardResetQuestionCountsCommits(t *testing.T) {
	cases := []struct {
		commits int
		want    string
	}{
		{commits: 0, want: "Hard reset to origin/main? Uncommitted changes are lost."},
		{commits: 1, want: "1 local commit and uncommitted changes are lost"},
		{commits: 2, want: "2 local commits and uncommitted changes are lost"},
	}

	for _, tc := range cases {
		t.Run(strconv.Itoa(tc.commits), func(t *testing.T) {
			_, repo := remoteRepo(t)
			for number := range tc.commits {
				write(t, repo, "a.txt", "local "+strconv.Itoa(number)+"\n")
				commitAll(t, repo)
			}

			model := openAsync(resize(open(t, options(repo, layout.Focus())), 120, 24), key('B'))
			model = hardReset(pressAndSettle(model, key('g')))
			if body := screenText(model); !strings.Contains(body, tc.want) {
				t.Fatalf("вопрос без %q:\n%s", tc.want, body)
			}
		})
	}
}

// g на другой ветке ведёт к ней (случай 2), на текущей без upstream подвал
// объясняет, почему нельзя (случай 4).
func TestResetToOtherBranch(t *testing.T) {
	repo, model := branchRepo(t)

	model = pressAndSettle(model, key('g'))
	if footer := footerOf(model); !strings.Contains(footer, "has no upstream") {
		t.Fatalf("подвал %q, ожидался отказ без upstream", footer)
	}

	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))
	model = pressAndSettle(model, key('g'))
	enterKey(model)

	if got, want := runGit(t, repo, "rev-parse", "main"), runGit(t, repo, "rev-parse", "unmerged"); got != want {
		t.Fatalf("main %s, ожидалась вершина unmerged %s", got, want)
	}
}

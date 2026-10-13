package tui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// mergeLog открывает лог, где ветка side слита в main. Сверху вниз: слияние,
// соединение «|\», коммит side, коммит main, соединение «|/», корень.
func mergeLog(t *testing.T) tea.Model {
	t.Helper()

	repo := newRepo(t)
	commit := func(name, subject string) {
		write(t, repo, name, subject+"\n")
		runGit(t, repo, "add", "-A")
		runGit(t, repo, "commit", "-q", "-m", subject)
	}
	commit("a.txt", "base")
	runGit(t, repo, "switch", "-q", "-c", "side")
	commit("b.txt", "side work")
	runGit(t, repo, "switch", "-q", "main")
	commit("c.txt", "main work")
	runGit(t, repo, "merge", "-q", "--no-ff", "-m", "merge side", "side")

	model := resize(open(t, options(repo, layout.Focus())), 120, 30)
	return openAsync(model, key('l'))
}

// Дорожка main и дорожка side разного цвета, а дорожка main одного цвета во
// всех строках: «|» в строке side и «*» в строке main стоят в первой дорожке.
func TestLogGraphLanesHaveOwnColors(t *testing.T) {
	raw := mergeLog(t).View().Content

	mainLane := styleBefore(t, raw, "side work", "|")
	sideLane := styleBefore(t, raw, "side work", "*")
	if mainLane == sideLane {
		t.Fatalf("дорожки main и side одного цвета %q", mainLane)
	}
	if other := styleBefore(t, raw, "main work", "*"); other != mainLane {
		t.Fatalf("дорожка main в строке main %q, в строке side %q", other, mainLane)
	}
}

// Под фильтром поиска графа нет: ни соединений, ни рисунка у коммитов. esc в
// поле снимает фильтр и возвращает граф.
func TestLogSearchHidesGraph(t *testing.T) {
	model := typeText(mergeLog(t), "/work")

	screen := screenOf(model)
	if text := strings.Join(screen, "\n"); strings.Contains(text, `|\`) || strings.Contains(text, "|/") {
		t.Fatalf("соединения графа под фильтром:\n%s", text)
	}
	if row := rowWith(t, screen, "side work"); strings.ContainsAny(row, "|*") {
		t.Fatalf("рисунок графа под фильтром: %q", row)
	}

	model, _ = model.Update(escape)
	if row := rowWith(t, screenOf(model), "side work"); !strings.Contains(row, "| *") {
		t.Fatalf("esc не вернул граф: %q", row)
	}
}

// enter на коммите ветки под графом открывает дифф этого коммита.
func TestLogGraphEnterOpensCommit(t *testing.T) {
	model := mergeLog(t)

	model, _ = model.Update(key('j'))
	model, cmd := model.Update(enter)
	model = settle(model, cmd, 3)

	if screen := screenText(model); strings.Contains(screen, "Log") || !strings.Contains(screen, "b.txt") || strings.Contains(screen, "c.txt") {
		t.Fatalf("enter не открыл коммит side:\n%s", screen)
	}
}

// Строки соединения видны между коммитами, j и k ставят курсор только на
// коммиты, а на последнем j курсор не уходит на соединение.
func TestLogGraphCursorSkipsConnectors(t *testing.T) {
	model := mergeLog(t)

	screen := screenText(model)
	for _, connector := range []string{`|\`, "|/"} {
		if !strings.Contains(screen, connector) {
			t.Fatalf("соединения %q нет:\n%s", connector, screen)
		}
	}

	for _, step := range []struct {
		key     rune
		subject string
	}{
		{'j', "side work"}, {'j', "main work"}, {'j', "base"}, {'j', "base"},
		{'k', "main work"}, {'k', "side work"}, {'k', "merge side"},
	} {
		model, _ = model.Update(key(step.key))
		if row := markedRow(model); !strings.Contains(row, step.subject) {
			t.Fatalf("после %c курсор на %q, ожидался коммит %q", step.key, row, step.subject)
		}
	}
}

// Граф шире 24 колонок обрезается справа: у слияния 13 веток в одной строке
// «| » стоит 12 раз и «*» 13-й ветки уходит в 25-ю колонку, за край. Столбец
// графа остаётся в 24 колонки и отделён от хэша промежутком в две.
func TestLogGraphIsCutAtMaxColumns(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "base.txt", "base\n")
	commitAll(t, repo)

	branches := make([]string, 13)
	for index := range branches {
		branches[index] = fmt.Sprintf("b%d", index+1)
		runGit(t, repo, "switch", "-q", "-c", branches[index], "main")
		write(t, repo, branches[index]+".txt", "x\n")
		runGit(t, repo, "add", "-A")
		runGit(t, repo, "commit", "-q", "-m", fmt.Sprintf("branch %d", index+1))
	}
	runGit(t, repo, "switch", "-q", "main")
	runGit(t, repo, append([]string{"merge", "-q", "-m", "octopus"}, branches...)...)

	model := resize(open(t, options(repo, layout.Focus())), 140, 40)
	screen := screenOf(openAsync(model, key('l')))

	last := rowWith(t, screen, "branch 13")
	if strings.Contains(last, "*") {
		t.Errorf("«*» 13-й ветки не обрезан: %q", last)
	}
	if gap := column(rowWith(t, screen, "Message"), "Commit") - column(last, "|"); gap != 24+2 {
		t.Errorf("от начала графа до столбца хэша %d колонок, want 26: %q", gap, last)
	}
}

// В линейной истории у каждого коммита «*» в первом столбце, перед столбцом
// хэша с промежутком в две колонки.
func TestLogGraphMarksEachCommit(t *testing.T) {
	screen := screenOf(searchLog(t, "init", "two", "three"))

	commit := column(rowWith(t, screen, "Message"), "Commit")
	for _, subject := range []string{"init", "two", "three"} {
		row := rowWith(t, screen, subject)
		if star := column(row, "*"); star != commit-3 {
			t.Errorf("«*» коммита %q в колонке %d, want %d: %q", subject, star, commit-3, row)
		}
	}
}

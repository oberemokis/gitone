package tui_test

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// clickOn кликает левой кнопкой по первому месту экрана, где написан text, и
// доводит команды клика до конца, как Bubble Tea.
func clickOn(t *testing.T, model tea.Model, text string) tea.Model {
	t.Helper()

	x, y := locate(t, model, text)
	return clickAt(model, x, y)
}

// locate находит клетку первого места экрана, где написан text.
func locate(t *testing.T, model tea.Model, text string) (x, y int) {
	t.Helper()

	for y, line := range screenOf(model) {
		if x := column(line, text); x >= 0 {
			return x, y
		}
	}

	t.Fatalf("на экране нет %q:\n%s", text, screenText(model))
	return 0, 0
}

func clickAt(model tea.Model, x, y int) tea.Model {
	model, cmd := model.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	return settle(model, cmd, 3)
}

// Клик по коммиту в логе сразу открывает его дифф, как enter.
func TestClickLogCommitOpensDiff(t *testing.T) {
	model := clickOn(t, searchLog(t, "base", "subject-one", "subject-two"), "subject-one")

	if screen := screenText(model); strings.Contains(screen, "Log") || !strings.Contains(screen, "subject-one") || strings.Contains(screen, "subject-two") {
		t.Fatalf("клик не открыл коммит subject-one:\n%s", screen)
	}
}

// Клик по ветке переключает на неё.
func TestClickBranchSwitches(t *testing.T) {
	repo, model := branchesWith(t, "feature-x")

	clickOn(t, model, "feature-x")

	if current := runGit(t, repo, "branch", "--show-current"); current != "feature-x" {
		t.Fatalf("текущая ветка %q, want feature-x", current)
	}
}

// Клик по файлу в панели открывает его.
func TestClickFileInPanelOpensIt(t *testing.T) {
	_, model := threeFileRepo(t)
	model, _ = model.Update(key('f'))

	model = clickOn(t, model, "b.txt")

	if header := screenOf(model)[headerRow]; !strings.Contains(header, "b.txt") || !strings.Contains(header, "(2/3)") {
		t.Fatalf("клик не открыл b.txt: %q", header)
	}
}

// Клик по строке поиска файлов выбирает файл.
func TestClickPickerRowChooses(t *testing.T) {
	_, model := threeFileRepo(t)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})

	model = clickOn(t, model, "c.txt")

	if header := screenOf(model)[headerRow]; !strings.Contains(header, "c.txt") || !strings.Contains(header, "(3/3)") {
		t.Fatalf("клик не выбрал c.txt: %q", header)
	}
}

// Клик по «? help» в подвале открывает справку, по «esc close» закрывает её,
// а в окне «esc close» закрывает окно.
func TestClickHintsOpenAndClose(t *testing.T) {
	_, model := threeFileRepo(t)

	model = clickOn(t, model, "? help")
	if !strings.Contains(screenText(model), "Hotkeys") {
		t.Fatalf("клик по ? help не открыл справку:\n%s", screenText(model))
	}
	model = clickOn(t, model, "esc close")
	if strings.Contains(screenText(model), "Hotkeys") {
		t.Fatalf("клик по esc close не закрыл справку:\n%s", screenText(model))
	}

	model = clickOn(t, searchLog(t, "base", "subject-one"), "esc close")
	if strings.Contains(screenText(model), "Log") {
		t.Fatalf("клик по esc close не закрыл лог:\n%s", screenText(model))
	}
}

// Клик по «B main» в подвале открывает окно веток.
func TestClickBranchLabelOpensBranches(t *testing.T) {
	_, model := threeFileRepo(t)

	if screen := screenText(clickOn(t, model, "B main")); !strings.Contains(screen, "Branches") {
		t.Fatalf("клик по B main не открыл ветки:\n%s", screen)
	}
}

// Клик по пустому месту окна, мимо окна и по диффу ничего не меняет.
func TestClickEmptyPlaceDoesNothing(t *testing.T) {
	model := searchLog(t, "base", "subject-one")
	before := screenText(model)
	screen := screenOf(model)
	title := slices.IndexFunc(screen, func(line string) bool { return strings.Contains(line, "Log") })

	for _, point := range [][2]int{{column(screen[title], "Log") + 10, title}, {1, title}} {
		model = clickAt(model, point[0], point[1])
		if after := screenText(model); after != before {
			t.Fatalf("клик в (%d, %d) изменил экран:\n%s", point[0], point[1], after)
		}
	}

	_, diff := threeFileRepo(t)
	before = screenText(diff)
	if after := screenText(clickAt(diff, 50, 8)); after != before {
		t.Fatalf("клик по диффу изменил экран:\n%s", after)
	}
}

// Переназначенная клавиша: подсказка показывает её, и клик делает действие.
func TestClickRemappedHint(t *testing.T) {
	repo, _ := threeFileRepo(t)
	model := openWithKeybinds(t, repo, `{"keybinds":{"app.help":"h"}}`, 100)

	if screen := screenText(clickOn(t, model, "h help")); !strings.Contains(screen, "Hotkeys") {
		t.Fatalf("клик по h help не открыл справку:\n%s", screen)
	}
}

// Клик по строке справки закрывает её и делает действие строки.
func TestClickHelpRowRunsAction(t *testing.T) {
	_, model := threeFileRepo(t)
	model, _ = model.Update(key('?'))

	screen := screenText(clickOn(t, model, "Branches"))
	if strings.Contains(screen, "Hotkeys") || !strings.Contains(screen, "(current)") {
		t.Fatalf("клик по строке Branches не закрыл справку и не открыл ветки:\n%s", screen)
	}
}

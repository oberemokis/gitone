package tui_test

import (
	"strings"
	"testing"
)

// Клик по пустой строке под «? help» в подвале и на колонку левее «?»
// открывает справку (#134).
func TestClickNearFooterHint(t *testing.T) {
	_, model := threeFileRepo(t)
	screen := screenOf(model)
	footer := footerRow(screen)
	help := column(screen[footer], "? help")

	for _, point := range [][2]int{{help + 2, footer + 1}, {help - 1, footer}} {
		if screen := screenText(clickAt(model, point[0], point[1])); !strings.Contains(screen, "Hotkeys") {
			t.Errorf("клик в (%d, %d) рядом с ? help не открыл справку:\n%s", point[0], point[1], screen)
		}
	}
}

// Клик на колонку правее стрелки › открывает следующий файл.
func TestClickNearHeaderArrow(t *testing.T) {
	_, model := threeFileRepo(t)

	if header := screenOf(clickAt(model, 99, headerRow))[headerRow]; !strings.Contains(header, "(2/3)") {
		t.Fatalf("клик правее › не открыл второй файл: %q", header)
	}
}

// Клик по пустой строке над подсказкой окна делает её действие: кнопки окна
// расширяются и внутри него.
func TestClickNearWindowHint(t *testing.T) {
	model := searchLog(t, "base", "subject-one", "subject-two")
	screen := screenOf(model)
	hint := rowOf(screen, "commit diff")

	model = clickAt(model, column(screen[hint], "enter")+1, hint-1)
	if screen := screenText(model); strings.Contains(screen, "Log") || !strings.Contains(screen, "subject-two") {
		t.Fatalf("клик над enter commit diff не открыл коммит:\n%s", screen)
	}
}

// Строки списка не расширяются: клик над первой строкой лога ничего не
// открывает. Клик посередине между подсказками подвала ничего не делает.
func TestClickBesideRowsAndBetweenHints(t *testing.T) {
	model := searchLog(t, "base", "subject-one", "subject-two")
	screen := screenOf(model)
	first := rowOf(screen, "subject-two")
	if after := screenText(clickAt(model, column(screen[first], "subject-two"), first-1)); after != screenText(model) {
		t.Errorf("клик над первой строкой лога изменил экран:\n%s", after)
	}

	_, diff := threeFileRepo(t)
	screen = screenOf(diff)
	footer := footerRow(screen)
	if after := screenText(clickAt(diff, column(screen[footer], "? help")-2, footer)); after != screenText(diff) {
		t.Errorf("клик между подсказками подвала изменил экран:\n%s", after)
	}
}

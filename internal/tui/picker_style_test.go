package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// openedPicker открывает поиск файлов в репозитории с правками в a.txt,
// b.txt, c.txt и новым d.txt.
func openedPicker(t *testing.T) tea.Model {
	t.Helper()

	repo, model := threeFileRepo(t)
	write(t, repo, "d.txt", "new\n")
	model = pressReload(t, model)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	return model
}

// Выбранный файл помечен «›», как в остальных списках, и строка без
// заливки цветом (#137).
func TestPickerMarksSelectedRow(t *testing.T) {
	model := openedPicker(t)

	row := markedRow(model)
	if !strings.Contains(row, "› a.txt") {
		t.Fatalf("выбранный файл не помечен «›»: %q", row)
	}
	line := strings.Split(model.View().Content, "\n")[rowOf(screenOf(model), row)]
	if cells := cellBackgrounds(line); cells[column(row, "a.txt")] != "20;20;20" {
		t.Fatalf("выбранная строка залита цветом %q", cells[column(row, "a.txt")])
	}
}

// Строка поиска «/ » с курсором и пустой строкой под ней, внизу подсказка
// enter open.
func TestPickerSearchLineAndHint(t *testing.T) {
	screen := screenOf(openedPicker(t))

	search := rowOf(screen, "/ ")
	if search != rowOf(screen, "Files")+2 || windowText(screen, search+1, "Files") != "" {
		t.Fatalf("строка поиска не «/ » с пустыми строками вокруг:\n%s", strings.Join(screen, "\n"))
	}
	if strings.Contains(strings.Join(screen, "\n"), "Search") {
		t.Fatalf("в пустом поле подсказка Search:\n%s", strings.Join(screen, "\n"))
	}
	if rowOf(screen, "enter open") < 0 {
		t.Fatalf("нет подсказки enter open:\n%s", strings.Join(screen, "\n"))
	}
}

// windowPadding это отступ заголовка title от левого края окна: от первой
// клетки с фоном окна до начала заголовка.
func windowPadding(t *testing.T, model tea.Model, title string) int {
	t.Helper()

	screen := screenOf(model)
	row := rowOf(screen, title)
	cells := cellBackgrounds(strings.Split(model.View().Content, "\n")[row])
	for x, background := range cells {
		if background == "20;20;20" {
			return column(screen[row], title) - x
		}
	}
	t.Fatalf("у строки заголовка %s нет фона окна", title)
	return 0
}

// Отступ заголовка от края окна тот же, что у окна веток.
func TestPickerPaddingMatchesBranches(t *testing.T) {
	_, branches := threeFileRepo(t)
	branches = openAsync(branches, key('B'))

	if got, want := windowPadding(t, openedPicker(t), "Files"), windowPadding(t, branches, "Branches"); got != want {
		t.Fatalf("отступ заголовка окна поиска %d, у окна веток %d", got, want)
	}
}

// Статус нового файла стоит справа, у края окна, приглушённым.
func TestPickerStatusOnTheRight(t *testing.T) {
	model := openedPicker(t)
	screen := screenOf(model)

	end := column(rowWith(t, screen, "close"), "close") + len("close")
	row := rowWith(t, screen, "d.txt")
	if column(row, "added")+len("added") != end {
		t.Errorf("статус не у правого края: %q", row)
	}
	if code := styleBefore(t, model.View().Content, "d.txt", "added"); !strings.Contains(code, "90") {
		t.Errorf("статус не приглушён: %q", code)
	}
}

// Клик по esc close закрывает окно поиска.
func TestClickPickerClose(t *testing.T) {
	if screen := screenText(clickOn(t, openedPicker(t), "esc close")); strings.Contains(screen, "Files") {
		t.Fatalf("клик по esc close не закрыл поиск:\n%s", screen)
	}
}

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// Экран 80 на 24: тело в 18 строк (3..20), в окне справки отступ сверху в
// строке 3, заголовок в 4, строки справки с 5 по 19, подвал в 22. Справка
// без прокрутки занимает 35 строк, в ней 32 строки под заголовком, поэтому
// сдвинуть её можно на 17.
const (
	helpTitleRow = 4
	helpLastRow  = 19
	helpFooter   = 22
)

// helpOnSmallScreen открывает справку на экране 80 на 24.
func helpOnSmallScreen(t *testing.T) tea.Model {
	t.Helper()

	_, model := threeFileRepo(t)
	model, _ = model.Update(key('?'))
	return resize(model, 80, 24)
}

// Справка выше экрана не выталкивает подвал: на терминале 80 на 24 экран
// остаётся в 24 строки, подвал стоит на предпоследней.
func TestHelpFitsSmallScreen(t *testing.T) {
	screen := screenOf(helpOnSmallScreen(t))

	if len(screen) != 24 {
		t.Fatalf("экран в %d строк, want 24:\n%s", len(screen), strings.Join(screen, "\n"))
	}
	if footer := screen[helpFooter]; !strings.Contains(footer, "q quit") || !strings.Contains(footer, "? help") {
		t.Fatalf("подвал не на строке %d:\n%s", helpFooter, strings.Join(screen, "\n"))
	}
	if row := rowOf(screen, "Hotkeys"); row != helpTitleRow {
		t.Fatalf("заголовок справки в строке %d, want %d:\n%s", row, helpTitleRow, strings.Join(screen, "\n"))
	}
}

// j и k двигают справку на строку, у краёв она стоит. Заголовок остаётся на
// месте, как подпись «esc close» рядом с ним.
func TestHelpScrollsByListKeys(t *testing.T) {
	model := helpOnSmallScreen(t)
	if row := rowOf(screenOf(model), "Quit"); row != 7 {
		t.Fatalf("строка Quit в начале: %d, want 7", row)
	}

	steps := []struct {
		name     string
		press    tea.KeyPressMsg
		wantQuit int
	}{
		{name: "j", press: key('j'), wantQuit: 6},
		{name: "стрелка вниз", press: tea.KeyPressMsg{Code: tea.KeyDown}, wantQuit: 5},
		{name: "k", press: key('k'), wantQuit: 6},
		{name: "стрелка вверх", press: tea.KeyPressMsg{Code: tea.KeyUp}, wantQuit: 7},
		{name: "k в начале", press: key('k'), wantQuit: 7},
	}

	for _, step := range steps {
		model, _ = model.Update(step.press)
		screen := screenOf(model)

		if row := rowOf(screen, "Quit"); row != step.wantQuit {
			t.Fatalf("%s: строка Quit %d, want %d:\n%s", step.name, row, step.wantQuit, strings.Join(screen, "\n"))
		}
		if title := screen[helpTitleRow]; !strings.Contains(title, "Hotkeys") || !strings.Contains(title, "esc close") {
			t.Fatalf("%s: заголовок справки сдвинулся:\n%s", step.name, strings.Join(screen, "\n"))
		}
	}
}

// Колесо двигает справку на 3 строки, как дифф.
func TestHelpScrollsByWheel(t *testing.T) {
	model := helpOnSmallScreen(t)

	steps := []struct {
		name         string
		wheel        tea.MouseButton
		wantBranches int
	}{
		{name: "вниз", wheel: tea.MouseWheelDown, wantBranches: 6},
		{name: "вверх", wheel: tea.MouseWheelUp, wantBranches: 9},
		{name: "вверх в начале", wheel: tea.MouseWheelUp, wantBranches: 9},
	}

	for _, step := range steps {
		model, _ = model.Update(tea.MouseWheelMsg{Button: step.wheel})
		screen := screenOf(model)

		if row := rowOf(screen, "Branches"); row != step.wantBranches {
			t.Fatalf("колесо %s: строка Branches %d, want %d:\n%s", step.name, row, step.wantBranches, strings.Join(screen, "\n"))
		}
	}
}

// Внизу справка стоит на последней строке: лишние нажатия не копятся, и k
// сразу двигает её назад. Группа «Diff» при этом видна целиком, заголовок и
// подвал на местях.
func TestHelpStopsAtBottom(t *testing.T) {
	model := typeText(helpOnSmallScreen(t), strings.Repeat("j", 60))
	screen := screenOf(model)

	if row := rowOf(screen, "Reload diff"); row != helpLastRow {
		t.Fatalf("Reload diff в строке %d, want %d:\n%s", row, helpLastRow, strings.Join(screen, "\n"))
	}
	if title := screen[helpTitleRow]; !strings.Contains(title, "Hotkeys") || !strings.Contains(title, "esc close") {
		t.Fatalf("заголовок справки сдвинулся:\n%s", strings.Join(screen, "\n"))
	}
	if !strings.Contains(screen[helpFooter], "q quit") {
		t.Fatalf("подвал не на месте:\n%s", strings.Join(screen, "\n"))
	}

	model, _ = model.Update(key('k'))
	if screen = screenOf(model); strings.Contains(strings.Join(screen, "\n"), "Reload diff") || rowOf(screen, "Scroll right") != helpLastRow {
		t.Fatalf("k не сдвинул справку на строку от низа:\n%s", strings.Join(screen, "\n"))
	}

	for range 10 {
		model, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	}
	if row := rowOf(screenOf(model), "Reload diff"); row != helpLastRow {
		t.Fatalf("колесо внизу: Reload diff в строке %d, want %d", row, helpLastRow)
	}
}

// На высоком экране справка помещается и не прокручивается: j и колесо
// двигают дифф, как без неё.
func TestHelpWithoutScrollLeavesKeysToDiff(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "seed.txt", "seed\n")
	commitAll(t, repo)
	write(t, repo, "long.txt", numberedLines(100))

	// справка растёт с каждым глобальным действием: экран взят с запасом
	model, _ := resize(open(t, options(repo, layout.Focus())), 100, 80).Update(key('?'))
	screen := screenOf(model)
	last := rowOf(screen, "Reload diff")
	if last < 0 || len(screen) != 80 {
		t.Fatalf("справка на экране 100 на 80 обрезана, экран в %d строк:\n%s", len(screen), strings.Join(screen, "\n"))
	}

	steps := []struct {
		name    string
		msg     tea.Msg
		wantTop int
	}{
		{name: "j", msg: key('j'), wantTop: 2},
		{name: "колесо вниз", msg: tea.MouseWheelMsg{Button: tea.MouseWheelDown}, wantTop: 5},
	}

	for _, step := range steps {
		model, _ = model.Update(step.msg)
		screen = screenOf(model)

		if got := topNumber(t, screen); got != step.wantTop {
			t.Fatalf("%s: первая строка тела %d, want %d", step.name, got, step.wantTop)
		}
		if row := rowOf(screen, "Reload diff"); row != last {
			t.Fatalf("%s: справка сдвинулась, Reload diff в строке %d, было %d", step.name, row, last)
		}
	}
}

// Справка клавиш не забирает, пока прокручивается: q выходит, ? закрывает.
func TestScrollingHelpKeepsOtherKeys(t *testing.T) {
	model := helpOnSmallScreen(t)

	if _, cmd := model.Update(key('q')); cmd == nil {
		t.Fatal("q не дал команду выхода при открытой справке")
	} else if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Fatal("q при открытой справке не вышел из приложения")
	}

	model, _ = model.Update(key('?'))
	if strings.Contains(screenText(model), "Hotkeys") {
		t.Fatalf("? не закрыл прокручиваемую справку:\n%s", screenText(model))
	}
}

// Места клика подвала лежат там, где подвал нарисован: клик по «q quit»
// выходит, клик по пустой клетке рядом ничего не делает. Раньше справка
// тянула тело вниз, подвал уходил за край, а клики оставались на старой
// строке: пустая клетка справа от окна закрывала приложение.
func TestHelpFooterClicksFollowDrawing(t *testing.T) {
	model := helpOnSmallScreen(t)
	screen := screenOf(model)
	row, quit := footerRow(screen), column(screen[footerRow(screen)], "q quit")

	_, cmd := model.Update(tea.MouseClickMsg{X: quit, Y: row, Button: tea.MouseLeft})
	if cmd == nil {
		t.Fatalf("клик по q quit в (%d, %d) ничего не сделал", quit, row)
	}
	if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Fatalf("клик по q quit в (%d, %d) не вышел из приложения", quit, row)
	}

	next, cmd := model.Update(tea.MouseClickMsg{X: quit - 2, Y: row, Button: tea.MouseLeft})
	if cmd != nil || screenText(next) != screenText(model) {
		t.Fatalf("клик по пустой клетке (%d, %d) подвала что-то сделал:\n%s", quit-2, row, screenText(next))
	}
}

// Прокрутка идёт по list.down и list.up из keybinds.json, а не по j и k.
func TestHelpScrollFollowsRebinding(t *testing.T) {
	repo, _ := threeFileRepo(t)
	model := resize(openWithKeybinds(t, repo, `{"keybinds":{"list.down":"J"}}`, 80), 80, 24)
	model, _ = model.Update(key('?'))

	if row := rowOf(screenOf(typeText(model, "j")), "Quit"); row != 7 {
		t.Fatalf("j сдвинул справку, хотя list.down это J: строка Quit %d", row)
	}
	if row := rowOf(screenOf(typeText(model, "J")), "Quit"); row != 6 {
		t.Fatalf("J не сдвинул справку: строка Quit %d, want 6", row)
	}
}

package tui_test

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
)

// Клавиши окон берутся из keybinds.json, как глобальные. Встроенные клавиши
// проверяют остальные тесты окон.

func openWithKeybinds(t *testing.T, repo, keybinds string, width int) tea.Model {
	t.Helper()

	opts := options(repo, layout.Focus())
	opts.Keys = loadKeys(t, keybinds)
	return resize(open(t, opts), width, 20)
}

// markedRow это строка списка под курсором.
func markedRow(model tea.Model) string {
	for _, line := range screenOf(model) {
		if strings.Contains(line, "›") {
			return line
		}
	}
	return ""
}

func TestRemappedListNavigation(t *testing.T) {
	repo, _ := threeFileRepo(t)
	runGit(t, repo, "commit", "-q", "--allow-empty", "-m", "zeta")
	model := openAsync(openWithKeybinds(t, repo, `{"keybinds":{"list.down":"n","list.close":"q"}}`, 100), key('l'))

	// тема стоит между двумя пробелами: так её не спутать с датой коммита
	model, _ = model.Update(key('j'))
	if row := markedRow(model); !strings.Contains(row, "  zeta  ") {
		t.Fatalf("после j курсор на %q, want на месте (zeta)", row)
	}
	model, _ = model.Update(key('n'))
	if row := markedRow(model); !strings.Contains(row, "  commit  ") {
		t.Fatalf("после n курсор на %q, want на commit", row)
	}

	if !strings.Contains(screenText(model), "q close") {
		t.Fatalf("подсказка без новой клавиши закрытия:\n%s", screenText(model))
	}
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(screenText(model), "Log") {
		t.Fatalf("esc закрыл лог, хотя закрытие на q:\n%s", screenText(model))
	}
	model, _ = model.Update(key('q'))
	if strings.Contains(screenText(model), "Log") {
		t.Fatalf("q не закрыл лог:\n%s", screenText(model))
	}
}

func TestDisabledStashApply(t *testing.T) {
	repo, _ := threeFileRepo(t)
	runGit(t, repo, "stash", "push", "-q")
	model := openAsync(openWithKeybinds(t, repo, `{"keybinds":{"stash.apply":"none"}}`, 100), key('Z'))

	if strings.Contains(screenText(model), "apply") {
		t.Fatalf("подсказка stash показывает отключённое apply:\n%s", screenText(model))
	}

	model, cmd := model.Update(key('a'))
	if cmd != nil {
		t.Fatal("a дал команду, хотя stash.apply отключён")
	}
	if !strings.Contains(screenText(model), "stash@{0}") {
		t.Fatalf("окно stash закрылось от a:\n%s", screenText(model))
	}
}

func TestRemappedQuestionYes(t *testing.T) {
	repo, _ := threeFileRepo(t)
	model := openWithKeybinds(t, repo, `{"keybinds":{"question.yes":"Y"}}`, 100)

	model, _ = model.Update(key('x'))
	if !strings.Contains(footerOf(model), "Y yes") {
		t.Fatalf("подвал = %q, want вопрос с клавишей Y", footerOf(model))
	}
	model, cmd := model.Update(key('y'))
	model = settle(model, cmd, 4)
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "new a.txt\n" {
		t.Fatalf("a.txt = %q после y, want правка на месте", got)
	}

	model, _ = model.Update(key('x'))
	model, cmd = model.Update(key('Y'))
	settle(model, cmd, 4)
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "old a.txt\n" {
		t.Fatalf("a.txt = %q после Y, want правка отменена", got)
	}
}

func TestRemappedSearchKeepsTyping(t *testing.T) {
	repo, _ := threeFileRepo(t)
	model := openWithKeybinds(t, repo, `{"keybinds":{"search.down":"ctrl+j"}}`, 100)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})

	model = typeText(model, "j")
	if !strings.Contains(screenText(model), "No matching files") {
		t.Fatalf("j не набралась в поиск:\n%s", screenText(model))
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	model, _ = model.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := openedFile(screenOf(model)); got != "b.txt" {
		t.Fatalf("открыт %q, want b.txt после ctrl+j", got)
	}
}

// enter без совпадений ничего не выбирает и окно поиска не закрывает.
func TestSearchChooseWithoutMatchesKeepsSearch(t *testing.T) {
	_, model := threeFileRepo(t)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	model = typeText(model, "zz")

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(screenText(model), "No matching files") {
		t.Fatalf("окно поиска закрылось без выбора:\n%s", screenText(model))
	}
}

// ctrl+c в окне выходит и тогда, когда выход переназначен.
func TestCtrlCQuitsWindowWithRemappedQuit(t *testing.T) {
	repo, _ := threeFileRepo(t)
	model := openAsync(openWithKeybinds(t, repo, `{"keybinds":{"app.quit":"w"}}`, 100), key('l'))

	if _, cmd := model.Update(key('w')); cmd != nil {
		t.Fatal("w в логе дал команду, want ничего: окно забирает нажатия")
	}
	_, cmd := model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c в логе не дал команды, want выход")
	}
	if _, quits := cmd().(tea.QuitMsg); !quits {
		t.Fatal("ctrl+c в логе не выходит")
	}
}

// Длинные клавиши в подсказке не раздвигают окно за край экрана.
func TestLongHintStaysOnNarrowScreen(t *testing.T) {
	repo, _ := threeFileRepo(t)
	runGit(t, repo, "stash", "push", "-q")
	keybinds := `{"keybinds":{"list.close":"ctrl+alt+shift+f12","stash.apply":"ctrl+alt+shift+f11"}}`
	model := openAsync(openWithKeybinds(t, repo, keybinds, 40), key('Z'))

	screen := screenOf(model)
	if len(screen) != 20 {
		t.Fatalf("экран %d строк, want 20:\n%s", len(screen), strings.Join(screen, "\n"))
	}
	for index, line := range screen {
		if width := ansi.StringWidth(line); width > 40 {
			t.Fatalf("строка %d шириной %d вылезла за экран в 40 колонок: %q", index, width, line)
		}
	}

	// перенос подсказки или строки списка выдаёт себя продолжением на
	// следующей строке окна: экран при этом не растёт, лишнее срезает наложение
	for index, line := range screen[:len(screen)-1] {
		wrapped := strings.TrimSpace(screen[index+1]) != ""
		if (strings.Contains(line, "enter diff") || strings.Contains(line, "stash@{0}")) && wrapped {
			t.Fatalf("строка окна перенеслась:\n%s", strings.Join(screen, "\n"))
		}
	}
}

package tui_test

import (
	"fmt"
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
	for row, line := range screenOf(model) {
		// в заголовке › это стрелка к следующему файлу (#120)
		if row != headerRow && strings.Contains(line, "›") {
			return line
		}
	}
	return ""
}

func TestRemappedListNavigation(t *testing.T) {
	repo, _ := threeFileRepo(t)
	runGit(t, repo, "commit", "-q", "--allow-empty", "-m", "zeta")
	model := openAsync(openWithKeybinds(t, repo, `{"keybinds":{"list.down":"J","list.close":"q"}}`, 100), key('l'))

	// тема стоит между двумя пробелами: так её не спутать с датой коммита
	model, _ = model.Update(key('j'))
	if row := markedRow(model); !strings.Contains(row, "  zeta  ") {
		t.Fatalf("после j курсор на %q, want на месте (zeta)", row)
	}
	model, _ = model.Update(key('J'))
	if row := markedRow(model); !strings.Contains(row, "  commit  ") {
		t.Fatalf("после J курсор на %q, want на commit", row)
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
	model := openAsync(openWithKeybinds(t, repo, `{"keybinds":{"app.quit":"e"}}`, 100), key('l'))

	if _, cmd := model.Update(key('e')); cmd != nil {
		t.Fatal("e в логе дал команду, want ничего: окно забирает нажатия")
	}
	_, cmd := model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c в логе не дал команды, want выход")
	}
	if _, quits := cmd().(tea.QuitMsg); !quits {
		t.Fatal("ctrl+c в логе не выходит")
	}
}

func ctrl(char rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: char, Mod: tea.ModCtrl}
}

// typed это клавиши набора текста по одной.
func typed(text string) []tea.KeyPressMsg {
	var presses []tea.KeyPressMsg
	for _, char := range text {
		presses = append(presses, key(char))
	}
	return presses
}

// opened нажимает клавиши по одной и доставляет ответ git после каждой, если он
// есть: так открывается окно со списком.
func opened(model tea.Model, presses ...tea.KeyPressMsg) tea.Model {
	for _, press := range presses {
		var cmd tea.Cmd
		model, cmd = model.Update(press)
		model = settle(model, cmd, 3)
	}
	return model
}

// remapRepo это репозиторий, где в каждом окне есть что выбрать: коммиты base и
// zeta (zeta меняет a.txt с init на zeta), ветки main, other и side, соседний
// worktree side-tree, stash с правкой stash-edit в a.txt и правки b.txt и c.txt
// в дереве, поэтому поиск файлов не пуст.
func remapRepo(t *testing.T) string {
	t.Helper()

	repo := newRepo(t)
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		write(t, repo, name, "init\n")
	}
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "base")

	write(t, repo, "a.txt", "zeta\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "zeta")

	runGit(t, repo, "branch", "other")
	runGit(t, repo, "worktree", "add", "-q", "-b", "side", filepath.Join(t.TempDir(), "side-tree"))

	write(t, repo, "a.txt", "stash-edit\n")
	runGit(t, repo, "stash", "push", "-q")
	write(t, repo, "b.txt", "work\n")
	write(t, repo, "c.txt", "work\n")
	return repo
}

// Переназначенное действие окна работает на новой клавише и не работает на
// прежней: прежняя ничего не меняет на экране и не даёт команды, новая даёт
// результат из таблицы. Подсказка показывает новую клавишу, а работать на
// зашитой могла бы старая.
func TestRemappedWindowActions(t *testing.T) {
	pressDown := tea.KeyPressMsg{Code: tea.KeyDown}

	for _, tc := range []struct {
		name   string
		action string
		newKey tea.KeyPressMsg
		// steps открывают окно и ставят его в состояние, где работает действие
		steps  []tea.KeyPressMsg
		oldKey tea.KeyPressMsg
		// shows и hides это то, что есть и чего нет на экране после новой клавиши
		shows, hides string
	}{
		{name: "log.open", action: "log.open", newKey: key('o'), steps: typed("l"), oldKey: enter, shows: "+ zeta", hides: "Log"},
		{name: "branches.switch", action: "branches.switch", newKey: key('o'), steps: typed("Bj"), oldKey: enter, shows: "B other", hides: "Branches"},
		{name: "worktrees.switch", action: "worktrees.switch", newKey: key('o'), steps: typed("Wj"), oldKey: enter, shows: "W side-tree", hides: "Worktrees"},
		{name: "stash.diff", action: "stash.diff", newKey: key('o'), steps: typed("Z"), oldKey: enter, shows: "+ stash-edit", hides: "Stash"},
		{name: "stash.drop", action: "stash.drop", newKey: key('x'), steps: typed("Z"), oldKey: key('d'), shows: "Drop stash@{0}?", hides: "apply"},
		{name: "list.up", action: "list.up", newKey: key('u'), steps: typed("Bj"), oldKey: key('k'), shows: "› main", hides: "› other"},
		{name: "listSearch.done", action: "listSearch.done", newKey: ctrl('g'), steps: typed("l/ze"), oldKey: enter, shows: "/ ze", hides: "done"},
		{name: "listSearch.cancel", action: "listSearch.cancel", newKey: ctrl('g'), steps: typed("l/ze"), oldKey: escape, shows: "base", hides: "/ ze"},
		{name: "commit.close", action: "commit.close", newKey: ctrl('g'), steps: typed("cdraft"), oldKey: escape, shows: "b.txt", hides: "> draft"},
		{name: "search.choose", action: "search.choose", newKey: ctrl('o'), steps: []tea.KeyPressMsg{ctrl('p'), key('c')}, oldKey: enter, shows: "c.txt +1 -1", hides: "Files"},
		{name: "search.close", action: "search.close", newKey: ctrl('o'), steps: []tea.KeyPressMsg{ctrl('p')}, oldKey: escape, shows: "b.txt", hides: "Files"},
		{name: "search.up", action: "search.up", newKey: ctrl('k'), steps: []tea.KeyPressMsg{ctrl('p'), pressDown}, oldKey: tea.KeyPressMsg{Code: tea.KeyUp}, shows: "› b.txt", hides: "› c.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			keybinds := fmt.Sprintf(`{"keybinds":{%q:%q}}`, tc.action, tc.newKey.String())
			model := opened(openWithKeybinds(t, remapRepo(t), keybinds, 120), tc.steps...)

			// hides должно быть видно до новой клавиши, иначе проверка пуста
			before := screenText(model)
			if !strings.Contains(before, tc.hides) {
				t.Fatalf("до нажатия на экране нет %q:\n%s", tc.hides, before)
			}

			model, cmd := model.Update(tc.oldKey)
			if cmd != nil || screenText(model) != before {
				t.Fatalf("прежняя клавиша %s сработала:\n%s", tc.oldKey, screenText(model))
			}

			model, cmd = model.Update(tc.newKey)
			model = settle(model, cmd, 4)

			screen := screenText(model)
			if !strings.Contains(screen, tc.shows) || strings.Contains(screen, tc.hides) {
				t.Fatalf("после %s на экране нет %q или есть %q:\n%s", tc.newKey, tc.shows, tc.hides, screen)
			}
		})
	}
}

// stash.apply на новой клавише применяет stash, на прежней нет.
func TestRemappedStashApply(t *testing.T) {
	repo := remapRepo(t)
	model := opened(openWithKeybinds(t, repo, `{"keybinds":{"stash.apply":"p"}}`, 120), key('Z'))

	if _, cmd := model.Update(key('a')); cmd != nil {
		t.Fatal("прежняя клавиша a дала команду")
	}

	model, cmd := model.Update(key('p'))
	settle(model, cmd, 4)
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "stash-edit\n" {
		t.Fatalf("a.txt = %q после p, want правка из stash", got)
	}
}

// commit.submit на новой клавише коммитит тему, на прежней нет.
func TestRemappedCommitSubmit(t *testing.T) {
	repo := remapRepo(t)
	runGit(t, repo, "add", "b.txt")
	before := runGit(t, repo, "rev-parse", "HEAD")
	model := openWithKeybinds(t, repo, `{"keybinds":{"commit.submit":"ctrl+s"}}`, 120)

	model, _ = model.Update(key('c'))
	model = typeText(model, "тема")
	model, cmd := model.Update(enter)
	if cmd != nil || runGit(t, repo, "rev-parse", "HEAD") != before {
		t.Fatal("прежняя клавиша enter отправила коммит")
	}

	model, cmd = model.Update(ctrl('s'))
	settle(model, cmd, 4)
	if got := runGit(t, repo, "log", "-1", "--format=%s"); got != "тема" {
		t.Fatalf("тема коммита %q, want тема", got)
	}
}

// backspace в поле поиска списка стирает символ целиком, а не байт: клавиша
// зашита и от keybinds.json не зависит.
func TestListSearchBackspaceErasesRune(t *testing.T) {
	model := opened(openWithKeybinds(t, remapRepo(t), `{}`, 120), key('l'))

	model = typeText(model, "/зета")
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})

	// после запроса стоит пробел курсора: обломок байта буквы его бы сдвинул
	if screen := screenText(model); !strings.Contains(screen, "/ зет ") {
		t.Fatalf("backspace не стёр последнюю букву запроса:\n%s", screen)
	}
}

// ctrl+c в поле поиска списка выходит из приложения.
func TestCtrlCQuitsListSearch(t *testing.T) {
	model := opened(openWithKeybinds(t, remapRepo(t), `{}`, 120), key('l'))
	model = typeText(model, "/")

	_, cmd := model.Update(ctrl('c'))
	if cmd == nil {
		t.Fatal("ctrl+c в поле поиска не дал команды, want выход")
	}
	if _, quits := cmd().(tea.QuitMsg); !quits {
		t.Fatal("ctrl+c в поле поиска не выходит")
	}
}

// backspace в поле темы коммита стирает символ целиком.
func TestCommitSubjectBackspaceErasesRune(t *testing.T) {
	model := openWithKeybinds(t, remapRepo(t), `{}`, 120)

	model, _ = model.Update(key('c'))
	model = typeText(model, "тест")
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})

	if screen := screenText(model); !strings.Contains(screen, "> тес_") || strings.Contains(screen, "> тест_") {
		t.Fatalf("backspace не стёр последнюю букву темы:\n%s", screen)
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
	// следующей строке окна: экран при этом не растёт, лишнее срезает наложение.
	// Смотрится только внутри окна: по краям виден ряд клавиш пустого экрана
	for index, line := range screen[:len(screen)-1] {
		wrapped := windowText(screen, index+1, "St…") != ""
		if (strings.Contains(line, "enter diff") || strings.Contains(line, "stash@{0}")) && wrapped {
			t.Fatalf("строка окна перенеслась:\n%s", strings.Join(screen, "\n"))
		}
	}
}

package tui_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// Мышь делает то, что видно под курсором (#150): вопрос discard не кликается,
// строка справки не жмёт чужую клавишу, поле поиска не съедает клик, двойной
// клик не попадает в панель под окном, колесо не крутит дифф под окном.

// discardAsked открывает справку и вопрос discard для a.txt на высоком экране:
// все строки справки на нём видны.
func discardAsked(t *testing.T, keybinds string) (string, tea.Model) {
	t.Helper()

	repo, _ := threeFileRepo(t)
	model := resize(openWithKeybinds(t, repo, keybinds, 100), 100, 60)
	model, _ = model.Update(key('?'))
	model, _ = model.Update(key('x'))
	return repo, model
}

// Клик по строке справки под вопросом discard ничего не делает, даже когда
// клавиша строки совпадает с ответом на вопрос.
func TestClickHelpRowUnderDiscardQuestionIsIgnored(t *testing.T) {
	repo, model := discardAsked(t, `{"keybinds":{"diff.toggleWrap":"y"}}`)
	before := screenText(model)

	model = clickOn(t, model, "Wrap long lines")

	if after := screenText(model); after != before {
		t.Fatalf("клик изменил экран:\n%s", after)
	}
	if got := readText(t, filepath.Join(repo, "a.txt")); got != "new a.txt\n" {
		t.Fatalf("a.txt = %q, want правка на месте", got)
	}
}

// Клик по «esc close» справки тоже не снимает вопрос.
func TestClickHelpCloseUnderDiscardQuestionIsIgnored(t *testing.T) {
	_, model := discardAsked(t, `{}`)

	model = clickOn(t, model, "esc close")

	if !strings.Contains(footerOf(model), "Discard changes") {
		t.Fatalf("подвал = %q, want вопрос discard", footerOf(model))
	}
}

// Строка «Toggle this help» закрывает справку: нажатая клавиша app.help
// открыла бы её снова.
func TestClickHelpToggleRowClosesHelp(t *testing.T) {
	_, model := threeFileRepo(t)
	model, _ = model.Update(key('?'))

	model = clickOn(t, model, "Toggle this help")

	if screen := screenText(model); strings.Contains(screen, "Hotkeys") {
		t.Fatalf("справка осталась открытой:\n%s", screen)
	}
}

// Клик по строке коммита при наборе «/subj» заканчивает набор и открывает
// коммит, а не просто закрывает поле.
func TestClickLogRowWhileSearchingOpensCommit(t *testing.T) {
	model := typeText(searchLog(t, "base", "subject-one", "subject-two"), "/subj")

	model = clickOn(t, model, "subject-one")

	if screen := screenText(model); strings.Contains(screen, "Log") || !strings.Contains(screen, "subject-one") || strings.Contains(screen, "subject-two") {
		t.Fatalf("клик не открыл коммит subject-one:\n%s", screen)
	}
}

// Имя клавиши в запрос не попадает: с listSearch.done на tab клик по строке
// не дописывает «enter» к «z».
func TestClickLogRowWhileSearchingTypesNothing(t *testing.T) {
	repo := commitSubjects(t, "base", "zeta", "other")
	model := openAsync(openWithKeybinds(t, repo, `{"keybinds":{"listSearch.done":"tab"}}`, 120), key('l'))
	model = typeText(model, "/z")

	model = clickOn(t, model, "zeta")

	if screen := screenText(model); strings.Contains(screen, "Log") || strings.Contains(screen, "zenter") {
		t.Fatalf("клик не открыл коммит zeta:\n%s", screen)
	}
}

// Клик по «q close» при наборе закрывает лог, а не дописывает «q» к запросу.
func TestClickCloseWhileSearchingClosesLog(t *testing.T) {
	repo := commitSubjects(t, "base", "subject-one")
	model := openAsync(openWithKeybinds(t, repo, `{"keybinds":{"list.close":"q"}}`, 120), key('l'))
	model = typeText(model, "/subj")

	model = clickOn(t, model, "q close")

	if screen := screenText(model); strings.Contains(screen, "Log") || strings.Contains(screen, "subjq") {
		t.Fatalf("клик по q close не закрыл лог:\n%s", screen)
	}
}

// Колесо при открытом окне не крутит дифф под ним.
func TestWheelUnderWindowDoesNotScrollDiff(t *testing.T) {
	repo := commitSubjects(t, "base")
	write(t, repo, "long.txt", numberedLines(60))
	model := openAsync(resize(open(t, options(repo, layout.Focus())), 120, 30), key('l'))

	for range 3 {
		model, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	}

	if top := topNumber(t, screenOf(model)); top != 1 {
		t.Fatalf("верхняя строка диффа %d, want 1", top)
	}
}

// Вопрос discard окна не рисует: дифф под ним виден и крутится колесом.
func TestWheelUnderDiscardQuestionScrollsDiff(t *testing.T) {
	repo := commitSubjects(t, "base")
	write(t, repo, "long.txt", numberedLines(60))
	model, _ := resize(open(t, options(repo, layout.Focus())), 120, 30).Update(key('x'))

	model, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})

	if top := topNumber(t, screenOf(model)); top != 4 {
		t.Fatalf("верхняя строка диффа %d, want 4", top)
	}
}

// clock это управляемые часы: время стоит, пока тест его не сдвинул.
type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

// searchOverPanel открывает поиск файлов ctrl+p поверх панели файлов в
// репозитории из файлов a1.txt … a8.txt на экране 100 на 20. Окно закрывает
// край панели: строка a3.txt в окне и седьмая строка панели, a7.txt, стоят в
// одной клетке экрана.
func searchOverPanel(t *testing.T) (tea.Model, *clock) {
	t.Helper()

	repo := newRepo(t)
	names := []string{"a1.txt", "a2.txt", "a3.txt", "a4.txt", "a5.txt", "a6.txt", "a7.txt", "a8.txt"}
	for _, name := range names {
		write(t, repo, name, "old\n")
	}
	commitAll(t, repo)
	for _, name := range names {
		write(t, repo, name, "new\n")
	}

	ticking := &clock{now: time.Date(2026, time.October, 13, 12, 0, 0, 0, time.UTC)}
	opts := options(repo, layout.Default())
	opts.Now = ticking.Now
	model := resize(open(t, opts), 100, 20)
	model, _ = model.Update(key('f'))
	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	return model, ticking
}

// lastLocate находит клетку последнего места экрана, где написан text: строка
// окна стоит ниже одноимённой строки панели.
func lastLocate(t *testing.T, model tea.Model, text string) (x, y int) {
	t.Helper()

	screen := screenOf(model)
	for y := len(screen) - 1; y >= 0; y-- {
		if x := column(screen[y], text); x >= 0 {
			return x, y
		}
	}

	t.Fatalf("на экране нет %q:\n%s", text, screenText(model))
	return 0, 0
}

// Второй клик двойного клика попадает в панель файлов под закрывшимся окном
// и открыл бы другой файл: клик в ту же клетку в течение 400 мс отбрасывается.
func TestDoubleClickAfterWindowClosesIsIgnored(t *testing.T) {
	model, ticking := searchOverPanel(t)
	x, y := lastLocate(t, model, "a3.txt")

	model = clickAt(model, x, y)
	ticking.now = ticking.now.Add(100 * time.Millisecond)
	model = clickAt(model, x, y)

	if opened := openedFile(screenOf(model)); opened != "a3.txt" {
		t.Fatalf("открыт %s, want a3.txt", opened)
	}
}

// Через секунду клик в ту же клетку уже обычный и открывает файл панели.
func TestClickInSameCellLaterWorks(t *testing.T) {
	model, ticking := searchOverPanel(t)
	x, y := lastLocate(t, model, "a3.txt")

	model = clickAt(model, x, y)
	ticking.now = ticking.now.Add(time.Second)
	model = clickAt(model, x, y)

	if opened := openedFile(screenOf(model)); opened != "a7.txt" {
		t.Fatalf("открыт %s, want a7.txt", opened)
	}
}

// Подсказки самого поля работают при наборе: «done» оставляет фильтр,
// «clear» снимает его.
func TestClickSearchFieldHints(t *testing.T) {
	model := typeText(searchLog(t, "base", "zeta"), "/z")

	done := clickOn(t, model, "enter done")
	if screen := screenText(done); !strings.Contains(screen, "/ z") || strings.Contains(screen, "base") {
		t.Fatalf("done не оставил фильтр:\n%s", screen)
	}

	cleared := clickOn(t, model, "esc clear")
	if screen := screenText(cleared); strings.Contains(screen, "/ z") || !strings.Contains(screen, "base") {
		t.Fatalf("clear не снял фильтр:\n%s", screen)
	}
}

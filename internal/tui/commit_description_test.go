package tui_test

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
)

// left и right лежат в wrap_test.go.
var (
	tab       = tea.KeyPressMsg{Code: tea.KeyTab}
	up        = tea.KeyPressMsg{Code: tea.KeyUp}
	down      = tea.KeyPressMsg{Code: tea.KeyDown}
	home      = tea.KeyPressMsg{Code: tea.KeyHome}
	end       = tea.KeyPressMsg{Code: tea.KeyEnd}
	backspace = tea.KeyPressMsg{Code: tea.KeyBackspace}
	del       = tea.KeyPressMsg{Code: tea.KeyDelete}
)

// stagedRepo это репозиторий с коммитом и застейдженной правкой a.txt.
func stagedRepo(t *testing.T) string {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "add", "a.txt")

	return repo
}

// commitWindow открывает окно коммита на экране width на height.
func commitWindow(t *testing.T, repo string, width, height int) tea.Model {
	t.Helper()

	model, _ := resize(open(t, options(repo, layout.Focus())), width, height).Update(key('c'))
	return model
}

// press нажимает клавиши по одной, как с клавиатуры.
func press(model tea.Model, presses ...tea.KeyPressMsg) tea.Model {
	for _, next := range presses {
		model, _ = model.Update(next)
	}

	return model
}

// commitMessage это тема и тело последнего коммита.
func commitMessage(t *testing.T, repo string) (subject, body string) {
	t.Helper()
	return runGit(t, repo, "log", "-1", "--format=%s"), runGit(t, repo, "log", "-1", "--format=%b")
}

// submit нажимает enter и доводит коммит до конца.
func submit(model tea.Model) tea.Model {
	model, cmd := model.Update(enter)
	return settle(model, cmd, 4)
}

// Тема, tab, описание в две строки, tab, enter: коммит с темой и телом.
func TestCommitSubjectAndDescription(t *testing.T) {
	repo := stagedRepo(t)
	model := commitWindow(t, repo, 100, 30)

	model = typeText(model, "fix")
	model = press(model, tab)
	model = typeText(model, "line 1")
	model = press(model, enter)
	model = typeText(model, "line 2")
	model = press(model, tab)
	submit(model)

	subject, body := commitMessage(t, repo)
	if subject != "fix" || body != "line 1\nline 2" {
		t.Fatalf("коммит с темой %q и телом %q, ожидались fix и «line 1\\nline 2»", subject, body)
	}
}

// enter в теме без описания коммитит, как раньше: тела нет.
func TestCommitSubjectWithoutDescription(t *testing.T) {
	repo := stagedRepo(t)
	model := commitWindow(t, repo, 100, 30)

	model = typeText(model, "fix")
	submit(model)

	subject, body := commitMessage(t, repo)
	if subject != "fix" || body != "" {
		t.Fatalf("коммит с темой %q и телом %q, ожидались fix и пустое тело", subject, body)
	}
}

// Окно подписывает оба поля, подсказка внизу называет поле, куда ведёт tab, а
// enter в описании ставит строку, поэтому коммит там не предлагается.
func TestCommitWindowLabelsFieldsAndHint(t *testing.T) {
	model := commitWindow(t, stagedRepo(t), 100, 30)

	screen := screenText(model)
	for _, want := range []string{"Subject", "Description", "enter commit   tab description"} {
		if !strings.Contains(screen, want) {
			t.Fatalf("в окне нет %q:\n%s", want, screen)
		}
	}

	screen = screenText(press(model, tab))
	if !strings.Contains(screen, "tab subject") || strings.Contains(screen, "enter commit") {
		t.Fatalf("в описании подсказка не «tab subject» без «enter commit»:\n%s", screen)
	}
}

// fieldRows это строки поля описания на экране: между подписью «Description» и
// пустой строкой над подсказкой, без пробелов справа.
func fieldRows(t *testing.T, model tea.Model) []string {
	t.Helper()

	screen := screenOf(model)
	label := rowOf(screen, "Description")
	hint := rowOf(screen, "tab ")
	if label < 0 || hint < 0 {
		t.Fatalf("в окне нет подписи или подсказки:\n%s", strings.Join(screen, "\n"))
	}

	// поле стоит под подписью с её левого края и шире окна не бывает; колонка
	// в клетках, и строка режется по клеткам: эмодзи шире одной
	start := column(screen[label], "Description")
	rows := make([]string, 0, hint-label-2)
	for _, line := range screen[label+1 : hint-1] {
		rows = append(rows, strings.TrimRight(ansi.Cut(line, start, start+56), " "))
	}

	return rows
}

// descriptionRows это непустая часть поля: пустые строки внизу отброшены, их
// число зависит от высоты поля.
func descriptionRows(t *testing.T, model tea.Model) []string {
	t.Helper()
	return strings.Split(strings.TrimRight(strings.Join(fieldRows(t, model), "\n"), "\n"), "\n")
}

// descriptionAfter набирает тему «s», переходит в описание и нажимает клавиши.
func descriptionAfter(t *testing.T, presses ...tea.KeyPressMsg) []string {
	t.Helper()

	model := commitWindow(t, stagedRepo(t), 100, 30)
	model = press(model, key('s'), tab)

	return descriptionRows(t, press(model, presses...))
}

// twoLines это нажатия, набирающие в описании «ab» и «cd» двумя строками.
func twoLines() []tea.KeyPressMsg {
	return slices.Concat(typed("ab"), []tea.KeyPressMsg{enter}, typed("cd"))
}

// Курсор движется стрелками, ввод идёт в его позицию, backspace и delete
// склеивают строки, home и end работают по строке.
func TestDescriptionCursorEditing(t *testing.T) {
	tests := []struct {
		name    string
		presses []tea.KeyPressMsg
		want    []string
	}{
		{name: "влево и вверх", presses: slices.Concat(twoLines(), []tea.KeyPressMsg{left, up}, typed("X")), want: []string{"aXb", "cd"}},
		{name: "вправо", presses: slices.Concat(typed("ab"), []tea.KeyPressMsg{left, left, right}, typed("X")), want: []string{"aXb"}},
		{name: "вниз", presses: slices.Concat(twoLines(), []tea.KeyPressMsg{up, left, down}, typed("X")), want: []string{"ab", "cXd"}},
		{name: "backspace в середине строки", presses: slices.Concat(typed("abc"), []tea.KeyPressMsg{left, backspace}), want: []string{"ac"}},
		{name: "backspace в начале строки", presses: slices.Concat(twoLines(), []tea.KeyPressMsg{left, left, backspace}), want: []string{"abcd"}},
		{name: "delete в середине строки", presses: slices.Concat(typed("abc"), []tea.KeyPressMsg{left, left, del}), want: []string{"ac"}},
		{name: "delete в конце строки", presses: slices.Concat(twoLines(), []tea.KeyPressMsg{up, del}), want: []string{"abcd"}},
		{name: "home и end", presses: slices.Concat(twoLines(), []tea.KeyPressMsg{home}, typed("X"), []tea.KeyPressMsg{end}, typed("Y")), want: []string{"ab", "XcdY"}},
		{name: "край текста", presses: slices.Concat(typed("a"), []tea.KeyPressMsg{right, del, up, left, left, backspace, down}, typed("X")), want: []string{"Xa"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := descriptionAfter(t, tc.presses...); !slices.Equal(got, tc.want) {
				t.Fatalf("описание %q, ожидалось %q", got, tc.want)
			}
		})
	}
}

// Вверх и вниз курсор помнит колонку, с которой начал: короткая строка по пути
// её не сбрасывает.
func TestDescriptionKeepsColumnOverShortLine(t *testing.T) {
	presses := slices.Concat(typed("abcd"), []tea.KeyPressMsg{enter}, typed("x"), []tea.KeyPressMsg{enter}, typed("1234"), []tea.KeyPressMsg{up, up}, typed("Z"))

	if got, want := descriptionAfter(t, presses...), []string{"abcdZ", "x", "1234"}; !slices.Equal(got, want) {
		t.Fatalf("описание %q, ожидалось %q", got, want)
	}
}

// Вставка в описание кладёт в поле все строки и коммит не отправляет: конец
// строки любого вида становится переводом строки, таб четырьмя пробелами, а
// управляющие символы терминала отбрасываются.
func TestPasteIntoDescription(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{name: "три строки", content: "one\ntwo\nthree", want: []string{"one", "two", "three"}},
		{name: "концы строк", content: "a\r\nb\rc", want: []string{"a", "b", "c"}},
		{name: "управляющие символы", content: "a\x1b[31mb", want: []string{"a[31mb"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model := press(commitWindow(t, stagedRepo(t), 100, 30), tab)

			model, cmd := paste(model, tc.content)

			if cmd != nil {
				t.Fatal("вставка запустила команду")
			}
			if got := descriptionRows(t, model); !slices.Equal(got, tc.want) {
				t.Fatalf("описание %q, ожидалось %q", got, tc.want)
			}
		})
	}
}

// Таб из вставки попадает в коммит четырьмя пробелами: ширину таба курсор не
// знает. Экран таб раскрывает сам, поэтому проверяется сообщение коммита.
func TestPasteTabBecomesSpaces(t *testing.T) {
	repo := stagedRepo(t)
	model := press(commitWindow(t, repo, 100, 30), key('s'), tab)

	model, _ = paste(model, "a\tb")
	submit(press(model, tab))

	if _, body := commitMessage(t, repo); body != "a    b" {
		t.Fatalf("тело коммита %q, ожидалось «a    b»", body)
	}
}

// cursorText это текст строки поля, в которой стоит курсор: клетка цветом
// primary есть только у него. Строка берётся из сырого вывода, потому что
// после снятия цвета курсор от текста не отличить.
func cursorText(t *testing.T, model tea.Model) string {
	t.Helper()

	var found []string
	for _, line := range strings.Split(model.View().Content, "\n") {
		if strings.Contains(line, "48;2;250;178;131") {
			found = append(found, strings.TrimSpace(ansi.Strip(line)))
		}
	}
	if len(found) != 1 {
		t.Fatalf("курсор на %d строках экрана, ожидалась одна: %q", len(found), found)
	}

	return found[0]
}

// lines это строки l1, l2, … без перевода строки в конце.
func lines(count int) string {
	return strings.TrimSuffix(numberedLines(count), "\n")
}

// Поле описания в шесть строк, пока на экране хватает места.
func TestDescriptionFieldHasSixRows(t *testing.T) {
	model := press(commitWindow(t, stagedRepo(t), 100, 30), tab)

	if got := len(fieldRows(t, model)); got != 6 {
		t.Fatalf("в поле %d строк, ожидалось 6", got)
	}
}

// Описание в 20 строк: видны шесть, поле идёт за курсором стрелками, а курсор
// остаётся на экране.
func TestDescriptionScrollsWithArrows(t *testing.T) {
	model := press(commitWindow(t, stagedRepo(t), 100, 30), tab)
	model, _ = paste(model, lines(20))

	if got, want := fieldRows(t, model), []string{"l15", "l16", "l17", "l18", "l19", "l20"}; !slices.Equal(got, want) {
		t.Fatalf("после вставки видно %q, ожидалось %q", got, want)
	}

	model = press(model, up, up, up, up, up)
	if got, want := fieldRows(t, model)[0], "l15"; got != want || cursorText(t, model) != "l15" {
		t.Fatalf("курсор на верхней строке: первая строка %q, курсор на %q, ожидалась %q", got, cursorText(t, model), want)
	}

	model = press(model, up)
	if got, want := fieldRows(t, model), []string{"l14", "l15", "l16", "l17", "l18", "l19"}; !slices.Equal(got, want) || cursorText(t, model) != "l14" {
		t.Fatalf("после шестого up видно %q с курсором на %q, ожидалось %q и l14", got, cursorText(t, model), want)
	}
}

// Колесо мыши прокручивает поле на три строки, курсор остаётся на экране.
func TestDescriptionScrollsWithWheel(t *testing.T) {
	model := press(commitWindow(t, stagedRepo(t), 100, 30), tab)
	model, _ = paste(model, lines(20))

	model, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})

	if got, want := fieldRows(t, model), []string{"l12", "l13", "l14", "l15", "l16", "l17"}; !slices.Equal(got, want) || cursorText(t, model) != "l17" {
		t.Fatalf("после колеса видно %q с курсором на %q, ожидалось %q и l17", got, cursorText(t, model), want)
	}

	for range 10 {
		model, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	}
	if got, want := fieldRows(t, model), []string{"l1", "l2", "l3", "l4", "l5", "l6"}; !slices.Equal(got, want) || cursorText(t, model) != "l6" {
		t.Fatalf("у верхнего края видно %q с курсором на %q, ожидалось %q и l6", got, cursorText(t, model), want)
	}

	for range 10 {
		model, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	}
	if got, want := fieldRows(t, model), []string{"l15", "l16", "l17", "l18", "l19", "l20"}; !slices.Equal(got, want) || cursorText(t, model) != "l15" {
		t.Fatalf("у нижнего края видно %q с курсором на %q, ожидалось %q и l15", got, cursorText(t, model), want)
	}
}

// Длинная строка без пробелов переносится по ширине поля: поле в 56 клеток, а
// последняя остаётся курсору, поэтому в строке поля 55 символов.
func TestDescriptionWrapsLongLine(t *testing.T) {
	model := press(commitWindow(t, stagedRepo(t), 100, 30), tab)

	model, _ = paste(model, strings.Repeat("x", 120))

	want := []string{strings.Repeat("x", 55), strings.Repeat("x", 55), strings.Repeat("x", 10)}
	if got := descriptionRows(t, model); !slices.Equal(got, want) {
		t.Fatalf("описание %q, ожидалось %q", got, want)
	}
}

// Перенос идёт по пробелам, слово не рвётся, пока оно помещается в строку.
func TestDescriptionWrapsAtSpaces(t *testing.T) {
	model := press(commitWindow(t, stagedRepo(t), 100, 30), tab)

	model, _ = paste(model, strings.Repeat("abcdefghi ", 8))

	want := []string{strings.TrimSpace(strings.Repeat("abcdefghi ", 5)), strings.TrimSpace(strings.Repeat("abcdefghi ", 3))}
	if got := descriptionRows(t, model); !slices.Equal(got, want) {
		t.Fatalf("описание %q, ожидалось %q", got, want)
	}
}

// Вверх и вниз курсор идёт по строкам поля, а не по строкам текста, и держит
// колонку.
func TestDescriptionArrowsFollowWrappedRows(t *testing.T) {
	model := press(commitWindow(t, stagedRepo(t), 100, 30), tab)
	model, _ = paste(model, strings.Repeat("x", 120))

	model = press(model, up)
	model = press(model, typed("Y")...)

	want := []string{strings.Repeat("x", 55), strings.Repeat("x", 10) + "Y" + strings.Repeat("x", 44), strings.Repeat("x", 11)}
	if got := descriptionRows(t, model); !slices.Equal(got, want) {
		t.Fatalf("описание %q, ожидалось %q", got, want)
	}
}

// Широкий символ занимает две клетки: строка поля делится по клеткам, а не по
// рунам.
func TestDescriptionWrapsWideCharacters(t *testing.T) {
	model := press(commitWindow(t, stagedRepo(t), 100, 30), tab)

	model, _ = paste(model, strings.Repeat("世", 40))

	want := []string{strings.Repeat("世", 27), strings.Repeat("世", 13)}
	if got := descriptionRows(t, model); !slices.Equal(got, want) {
		t.Fatalf("описание %q, ожидалось %q", got, want)
	}
}

// Низкий экран: тело в 11 строк, поле сжато до одной, окно целиком между
// заголовком и подвалом, оба поля доступны.
func TestCommitWindowFitsLowScreen(t *testing.T) {
	model := commitWindow(t, stagedRepo(t), 100, 17)

	model = typeText(model, "s")
	model = press(model, tab)
	model = typeText(model, "x")
	model = press(model, enter)
	model = typeText(model, "y")

	screen := screenOf(model)
	title, subject, description, hint := rowOf(screen, "Commit"), rowOf(screen, "Subject"), rowOf(screen, "Description"), rowOf(screen, "tab subject")
	if len(screen) != 17 || title < bodyRow || hint >= footerRow(screen)-1 || subject <= title || description <= subject {
		t.Fatalf("окно не поместилось между заголовком и подвалом (заголовок %d, подпись темы %d, подпись описания %d, подсказка %d):\n%s", title, subject, description, hint, strings.Join(screen, "\n"))
	}
	if got := fieldRows(t, model); !slices.Equal(got, []string{"y"}) {
		t.Fatalf("в сжатом поле %q, ожидалась одна строка с курсором", got)
	}

	if got := descriptionRows(t, press(model, up)); !slices.Equal(got, []string{"x"}) {
		t.Fatalf("после up в сжатом поле %q, ожидалась строка x", got)
	}
}

// Длинная тема остаётся в одной строке и не растит окно.
func TestLongSubjectKeepsWindowHeight(t *testing.T) {
	model := commitWindow(t, stagedRepo(t), 100, 30)
	before := rowOf(screenOf(model), "Description")

	model = typeText(model, strings.Repeat("a", 100)+"END")

	screen := screenOf(model)
	if after := rowOf(screen, "Description"); after != before {
		t.Fatalf("подпись описания съехала с строки %d на %d:\n%s", before, after, strings.Join(screen, "\n"))
	}
	if !strings.Contains(screenText(model), "…aaa") || !strings.Contains(screenText(model), "aaEND_") {
		t.Fatalf("в теме не виден конец с курсором:\n%s", screenText(model))
	}
}

// commitWindowWith открывает окно коммита с клавишами из keybinds.json.
func commitWindowWith(t *testing.T, repo, keybinds string) tea.Model {
	t.Helper()

	opts := options(repo, layout.Focus())
	opts.Keys = loadKeys(t, keybinds)
	model, _ := resize(open(t, opts), 100, 30).Update(key('c'))
	return model
}

// commit.field на новой клавише переключает поле, на прежней нет, подсказка
// называет новую клавишу.
func TestRemappedCommitField(t *testing.T) {
	model := commitWindowWith(t, stagedRepo(t), `{"keybinds":{"commit.field":"ctrl+t"}}`)
	before := screenText(model)
	if !strings.Contains(before, "ctrl t description") {
		t.Fatalf("в подсказке нет «ctrl t description»:\n%s", before)
	}

	model, cmd := model.Update(tab)
	if cmd != nil || screenText(model) != before {
		t.Fatalf("прежняя клавиша tab сработала:\n%s", screenText(model))
	}

	model, _ = model.Update(ctrl('t'))
	if screen := screenText(model); !strings.Contains(screen, "ctrl t subject") {
		t.Fatalf("после ctrl+t поле не переключилось:\n%s", screen)
	}
}

// commit.field со значением none отключает переключение: описание недоступно,
// набор идёт в тему, а подсказка о переключении пропадает.
func TestDisabledCommitFieldKeepsSubject(t *testing.T) {
	repo := stagedRepo(t)
	model := commitWindowWith(t, repo, `{"keybinds":{"commit.field":"none"}}`)

	model = press(model, tab)
	model = typeText(model, "fix")

	screen := screenText(model)
	if !strings.Contains(screen, "> fix_") || strings.Contains(screen, "tab") || strings.Contains(screen, "description") {
		t.Fatalf("описание доступно или подсказка осталась при отключённом commit.field:\n%s", screen)
	}

	submit(model)
	if subject, body := commitMessage(t, repo); subject != "fix" || body != "" {
		t.Fatalf("коммит с темой %q и телом %q, ожидались fix и пустое тело", subject, body)
	}
}

// esc в описании закрывает окно без коммита, а новое окно открывается пустым.
func TestEscInDescriptionClosesWithoutCommit(t *testing.T) {
	repo := stagedRepo(t)
	before := runGit(t, repo, "rev-parse", "HEAD")
	model := commitWindow(t, repo, 100, 30)

	model = typeText(model, "draft")
	model = press(model, tab)
	model = typeText(model, "body")
	model, cmd := model.Update(escape)

	if cmd != nil || strings.Contains(screenText(model), "Description") {
		t.Fatalf("esc в описании не закрыл окно:\n%s", screenText(model))
	}
	if after := runGit(t, repo, "rev-parse", "HEAD"); after != before {
		t.Fatal("коммит прошёл после esc")
	}

	model, _ = model.Update(key('c'))
	if screen := screenText(model); !strings.Contains(screen, "> _") || strings.Contains(screen, "draft") || strings.Contains(screen, "body") {
		t.Fatalf("новое окно открылось не пустым:\n%s", screen)
	}
}

// ctrl+c в описании выходит из приложения, q и ? набираются как текст.
func TestDescriptionQuitKeys(t *testing.T) {
	model := press(commitWindow(t, stagedRepo(t), 100, 30), tab)

	model = typeText(model, "q?")
	if got := descriptionRows(t, model); !slices.Equal(got, []string{"q?"}) {
		t.Fatalf("описание %q, ожидалось q?", got)
	}

	_, cmd := model.Update(ctrl('c'))
	if cmd == nil {
		t.Fatal("ctrl+c в описании не дал команды, want выход")
	}
	if _, quits := cmd().(tea.QuitMsg); !quits {
		t.Fatal("ctrl+c в описании не выходит")
	}
}

// Описание из одних пробелов и переводов строки коммит без тела.
func TestBlankDescriptionGivesNoBody(t *testing.T) {
	repo := stagedRepo(t)
	model := press(commitWindow(t, repo, 100, 30), key('s'), tab)

	model, _ = paste(model, "   \n  \n")
	submit(press(model, tab))

	subject, body := commitMessage(t, repo)
	if subject != "s" || body != "" {
		t.Fatalf("коммит с темой %q и телом %q, ожидались s и пустое тело", subject, body)
	}
}

// Тело коммита берётся как набрано: пустые строки между абзацами остаются.
func TestDescriptionKeepsBlankLinesBetweenParagraphs(t *testing.T) {
	repo := stagedRepo(t)
	model := press(commitWindow(t, repo, 100, 30), key('s'), tab)

	model, _ = paste(model, "first\n\nsecond")
	submit(press(model, tab))

	if _, body := commitMessage(t, repo); body != "first\n\nsecond" {
		t.Fatalf("тело коммита %q, ожидалось «first\\n\\nsecond»", body)
	}
}

// Подсказки окна кликабельны: tab открывает описание, enter коммитит.
func TestCommitHintsAreClickable(t *testing.T) {
	repo := stagedRepo(t)
	model := press(commitWindow(t, repo, 100, 30), key('s'))

	model = clickOn(t, model, "tab description")
	if screen := screenText(model); !strings.Contains(screen, "tab subject") {
		t.Fatalf("клик по «tab description» не открыл описание:\n%s", screen)
	}

	model = clickOn(t, model, "tab subject")
	clickOn(t, model, "enter commit")

	if subject, _ := commitMessage(t, repo); subject != "s" {
		t.Fatalf("клик по «enter commit» не сделал коммит, тема %q", subject)
	}
}

// Вставка в тему оставляет первую строку, описание остаётся пустым.
func TestPasteThreeLinesIntoSubject(t *testing.T) {
	model := commitWindow(t, stagedRepo(t), 100, 30)

	model, _ = paste(model, "alpha\nbeta\ngamma")

	if screen := screenText(model); !strings.Contains(screen, "> alpha_") || strings.Contains(screen, "beta") {
		t.Fatalf("в теме не первая строка вставки:\n%s", screen)
	}
	if got := descriptionRows(t, press(model, tab)); !slices.Equal(got, []string{""}) {
		t.Fatalf("описание %q после вставки в тему, ожидалось пустое", got)
	}
}

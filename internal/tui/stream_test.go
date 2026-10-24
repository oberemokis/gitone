package tui_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/config"
	"gitone/internal/layout"
	"gitone/internal/tui"
)

// Лента всех файлов (#373): файлы подряд на одной странице, у каждого
// заголовок «путь +N -M», текущий файл тот, чья строка наверху.

var ctrlD = tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}

// streamFiles правит три файла: в a.txt одну строку из трёх, в b.txt из 40
// строк l5 и l35 (два хунка), в c.txt добавляет строку (правка с одной
// стороны).
func streamFiles(t testing.TB) string {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "a1\na2\na3\n")
	write(t, repo, "b.txt", numberedLines(40))
	write(t, repo, "c.txt", "c1\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "a1\nA2\na3\n")
	write(t, repo, "b.txt", strings.NewReplacer("l5\n", "L5\n", "l35\n", "L35\n").Replace(numberedLines(40)))
	write(t, repo, "c.txt", "c1\nc2\n")
	return repo
}

// allFiles это настройки с лентой из config.json.
func allFiles(repo string) tui.Options {
	opts := options(repo, layout.Focus())
	opts.Config.Diff.AllFiles = true
	return opts
}

// streamRepo это streamFiles, открытый лентой на экране width на height.
func streamRepo(t *testing.T, width, height int) (string, tea.Model) {
	t.Helper()

	repo := streamFiles(t)
	return repo, resize(open(t, allFiles(repo)), width, height)
}

// topFile это заголовок файла, стоящий наверху тела, без пробелов по краям:
// пустая строка, «путь +N -M», пустая строка. Пусто, если наверху не он.
func topFile(screen []string) string {
	if strings.TrimSpace(screen[bodyRow]) != "" {
		return ""
	}
	return strings.TrimSpace(screen[bodyRow+1])
}

// fileHeaders это заголовки файлов ленты на экране по порядку.
func fileHeaders(screen []string) []string {
	var headers []string
	for _, row := range screen[bodyRow:] {
		fields := strings.Fields(row)
		if len(fields) == 3 && strings.HasPrefix(fields[1], "+") && strings.HasPrefix(fields[2], "-") && strings.HasSuffix(fields[0], ".txt") {
			headers = append(headers, strings.Join(fields, " "))
		}
	}
	return headers
}

func staged(t *testing.T, repo string) string {
	t.Helper()
	return runGit(t, repo, "diff", "--cached", "--name-only")
}

// Без настройки дифф показывает один файл (случай 1), a выводит все подряд с
// заголовком у каждого (случай 2).
func TestAllFilesKeyShowsEveryFile(t *testing.T) {
	repo := streamFiles(t)
	model := resize(open(t, options(repo, layout.Focus())), 100, 40)

	if headers := fileHeaders(screenOf(model)); len(headers) > 0 {
		t.Fatalf("без ленты на экране заголовки файлов %q", headers)
	}

	model, _ = model.Update(key('a'))
	want := []string{"a.txt +1 -1", "b.txt +2 -2", "c.txt +1 -0"}
	if headers := fileHeaders(screenOf(model)); !slices.Equal(headers, want) {
		t.Fatalf("заголовки ленты %q, ожидались %q:\n%s", headers, want, screenText(model))
	}
}

// ] ставит наверх заголовок следующего файла, короткий последний файл тоже
// доезжает до верха, а на нём ] ничего не делает (случаи 3 и 13).
func TestNextFileHeaderOnTop(t *testing.T) {
	_, model := streamRepo(t, 100, 30)

	for _, want := range []string{"b.txt +2 -2", "c.txt +1 -0", "c.txt +1 -0"} {
		model, _ = model.Update(key(']'))
		screen := screenOf(model)
		if got := topFile(screen); got != want {
			t.Fatalf("наверху %q, ожидался заголовок %q:\n%s", got, want, strings.Join(screen, "\n"))
		}
	}
	if header := headerText(screenOf(model)); !strings.Contains(header, "c.txt") || !strings.Contains(header, "(3/3)") {
		t.Fatalf("заголовок экрана %q, ожидался последний файл", header)
	}
}

// [ из середины файла ставит наверх его заголовок, второе нажатие заголовок
// предыдущего (случай 14). g и G ведут к краям ленты (случай 15).
func TestPreviousFileAndEdges(t *testing.T) {
	_, model := streamRepo(t, 100, 30)

	model, _ = model.Update(key(']'))
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))
	model, _ = model.Update(key('j'))
	if got := topFile(screenOf(model)); got != "" {
		t.Fatalf("после j наверху всё ещё заголовок %q", got)
	}

	for _, want := range []string{"b.txt +2 -2", "a.txt +1 -1"} {
		model, _ = model.Update(key('['))
		if got := topFile(screenOf(model)); got != want {
			t.Fatalf("после [ наверху %q, ожидался %q", got, want)
		}
	}

	model, _ = model.Update(key('G'))
	if got := topFile(screenOf(model)); got != "c.txt +1 -0" {
		t.Fatalf("после G наверху %q, ожидался последний файл", got)
	}
	model, _ = model.Update(key('g'))
	if got := topFile(screenOf(model)); got != "a.txt +1 -1" {
		t.Fatalf("после g наверху %q, ожидался первый файл", got)
	}
}

// Заголовок экрана идёт за файлом наверху, и space ставит в индекс его
// (случай 5).
func TestTopFileIsCurrent(t *testing.T) {
	repo, model := streamRepo(t, 100, 30)

	// блок a.txt: три строки заголовка и четыре строки файла
	for range 7 {
		model, _ = model.Update(key('j'))
	}
	if header := headerText(screenOf(model)); !strings.Contains(header, "b.txt") || !strings.Contains(header, "(2/3)") {
		t.Fatalf("заголовок экрана %q, ожидался b.txt наверху ленты", header)
	}

	pressAndSettle(model, space)
	if got := staged(t, repo); got != "b.txt" {
		t.Fatalf("в индексе %q, ожидался файл наверху b.txt", got)
	}
}

// Выбор ленты переживает перезапуск (случай 4).
func TestAllFilesSurvivesRestart(t *testing.T) {
	repo := streamFiles(t)
	opts := options(repo, layout.Focus())
	opts.StateDir = t.TempDir()
	model := resize(open(t, opts), 100, 40)

	model = pressAndSettle(model, key('a'))
	opts.View = config.ReadView(opts.StateDir)
	restarted := resize(open(t, opts), 100, 40)

	if !opts.View.AllFiles {
		t.Fatalf("в state.json вид %+v, ожидалась лента", opts.View)
	}
	if got, want := screenOf(restarted), screenOf(model); !slices.Equal(got, want) {
		t.Fatalf("после перезапуска экран\n%s\nожидался\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// В split файл с правкой на одной стороне идёт unified во всю ширину, файл с
// двумя сторонами остаётся split (случай 6).
func TestOneSidedFileUnifiedInSplit(t *testing.T) {
	_, model := streamRepo(t, 160, 40)
	screen := screenOf(model)

	if !isSplit(screen, "a2", "A2") {
		t.Fatalf("a.txt не в split:\n%s", strings.Join(screen, "\n"))
	}
	for _, row := range screen {
		if at := column(row, "c2"); at >= 0 && at > 40 {
			t.Fatalf("строка c2 в колонке %d, ожидался unified во всю ширину:\n%s", at, strings.Join(screen, "\n"))
		}
	}
}

// После space и перечитывания наверху та же строка (случай 7).
func TestStreamKeepsTopAfterStageAndReload(t *testing.T) {
	_, model := streamRepo(t, 100, 20)
	for range 12 {
		model, _ = model.Update(key('j'))
	}
	want := screenOf(model)[bodyRow]

	model = pressAndSettle(model, space)
	if got := screenOf(model)[bodyRow]; got != want {
		t.Fatalf("после space наверху %q, ожидалась %q", got, want)
	}
	model = pressReload(t, model)
	if got := screenOf(model)[bodyRow]; got != want {
		t.Fatalf("после r наверху %q, ожидалась %q", got, want)
	}
}

// Пропавший файл сменяет следующий, а пропавший последний предыдущий
// (случай 8).
func TestVanishedFileGivesWayToNeighbour(t *testing.T) {
	tests := []struct {
		name, gone string
		jumps      int
		want       string
	}{
		{name: "следующий", gone: "b.txt", jumps: 1, want: "c.txt +1 -0"},
		{name: "предыдущий у последнего", gone: "c.txt", jumps: 2, want: "b.txt +2 -2"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, model := streamRepo(t, 100, 30)
			for range tc.jumps {
				model, _ = model.Update(key(']'))
			}

			runGit(t, repo, "checkout", "--", tc.gone)
			model = pressReload(t, model)

			if got := topFile(screenOf(model)); got != tc.want {
				t.Fatalf("наверху %q, ожидался %q:\n%s", got, tc.want, screenText(model))
			}
		})
	}
}

// Другой дифф открывается с начала ленты, хоть открытый файл есть и в нём:
// коммит из лога показывает b.txt с другими строками.
func TestOtherDiffStartsStreamFromTop(t *testing.T) {
	repo := streamFiles(t)
	commitAll(t, repo)
	write(t, repo, "a.txt", "a1\nA2\nA3\n")
	write(t, repo, "b.txt", strings.Replace(readText(t, filepath.Join(repo, "b.txt")), "l20\n", "L20\n", 1))
	model := resize(open(t, allFiles(repo)), 100, 30)
	model, _ = model.Update(key(']'))

	model = enterKey(openAsync(model, key('l')))

	if got := topFile(screenOf(model)); got != "a.txt +1 -1" {
		t.Fatalf("коммит из лога открылся с %q, ожидалось начало ленты:\n%s", got, screenText(model))
	}
}

// Выделение не уходит из файла: колесо и ctrl+d упираются в край его блока, и
// space ставит строки этого файла (случай 10).
func TestSelectionStaysInItsFile(t *testing.T) {
	repo, model := streamRepo(t, 100, 20)

	model = pressAndSettle(model, key('v'))
	for range 3 {
		model, _ = model.Update(ctrlD)
	}
	model = wheel(model, tea.MouseWheelDown, 3)

	screen := screenOf(model)
	if got := topFile(screen); got != "a.txt +1 -1" {
		t.Fatalf("при выделении наверху %q, ожидался блок a.txt:\n%s", got, strings.Join(screen, "\n"))
	}
	if marked := markedText(model); !strings.Contains(marked, "A2") {
		t.Fatalf("выделение потерялось:\n%s", strings.Join(screen, "\n"))
	}
	pressAndSettle(model, space)
	if got := staged(t, repo); got != "a.txt" {
		t.Fatalf("в индексе %q, ожидался файл выделения a.txt", got)
	}
}

// Выбор конфликтов тоже живёт в своём файле (случай 11).
func TestConflictSelectionStaysInItsFile(t *testing.T) {
	repo, _ := conflictRepo(t)
	write(t, repo, "z.txt", numberedLines(60))
	model := resize(open(t, allFiles(repo)), 100, 20)

	model = pressAndSettle(model, key('v'))
	for range 5 {
		model, _ = model.Update(ctrlD)
	}

	if header := headerText(screenOf(model)); !strings.Contains(header, "a.txt") {
		t.Fatalf("при выборе конфликтов заголовок %q, ожидался a.txt", header)
	}
	if marked := markedText(model); !strings.Contains(marked, "=======") {
		t.Fatalf("конфликт под курсором не виден:\n%s", screenText(model))
	}
}

// ], клик в панели и a снимают выделение (случай 16).
func TestJumpsEndSelection(t *testing.T) {
	_, model := streamRepo(t, 100, 30)

	model = pressAndSettle(model, key('v'))
	model, _ = model.Update(key(']'))
	if marked := markedText(model); marked != "" {
		t.Fatalf("после ] выделение осталось:\n%s", marked)
	}
	if got := topFile(screenOf(model)); got != "b.txt +2 -2" {
		t.Fatalf("после ] наверху %q, ожидался b.txt", got)
	}

	model = pressAndSettle(model, key('v'))
	model, _ = model.Update(key('a'))
	if marked := markedText(model); marked != "" {
		t.Fatalf("после a выделение осталось:\n%s", marked)
	}
	if headers := fileHeaders(screenOf(model)); len(headers) > 0 {
		t.Fatalf("после a лента осталась: %q", headers)
	}
}

// Клик в панели снимает выделение и ставит файл наверх.
func TestPanelClickEndsSelection(t *testing.T) {
	repo := streamFiles(t)
	opts := allFiles(repo)
	opts.Layout = layout.Default()
	model := toggleFiles(resize(open(t, opts), 120, 30))

	model = pressAndSettle(model, key('v'))
	model = clickPanel(t, model, "c.txt")

	if marked := markedText(model); marked != "" {
		t.Fatalf("после клика выделение осталось:\n%s", marked)
	}
	if header := headerText(screenOf(model)); !strings.Contains(header, "c.txt") {
		t.Fatalf("после клика заголовок %q, ожидался c.txt", header)
	}
}

// clickPanel кликает по строке панели файлов с текстом name: панель слева, и
// то же имя в заголовке экрана и в ленте правее неё.
func clickPanel(t *testing.T, model tea.Model, name string) tea.Model {
	t.Helper()

	for y, row := range screenOf(model) {
		if x := column(row, name); y >= bodyRow && x >= 0 && x < 30 {
			return clickAt(model, x, y)
		}
	}
	t.Fatalf("в панели нет %q:\n%s", name, screenText(model))
	return model
}

// x стёр единственный хунк выделения, и файл пропал из диффа: выделение
// снято, соседние файлы целы (случай 17).
func TestSelectionGoneWithItsFile(t *testing.T) {
	repo, model := streamRepo(t, 100, 30)

	model = pressAndSettle(model, key('v'))
	model = pressAndSettle(model, key('x'))
	model = pressAndSettle(model, key('y'))

	if marked := markedText(model); marked != "" {
		t.Fatalf("выделение пережило свой файл:\n%s", marked)
	}
	if got := runGit(t, repo, "diff", "--name-only"); got != "b.txt\nc.txt" {
		t.Fatalf("правки в рабочем дереве %q, ожидались b.txt и c.txt", got)
	}
	// тост итога закрывает верх ленты, esc его снимает
	model = pressAndSettle(model, escape)
	if got := topFile(screenOf(model)); got != "b.txt +2 -2" {
		t.Fatalf("наверху %q, ожидался следующий файл b.txt:\n%s", got, screenText(model))
	}
}

// Пустая лента не падает на прокрутке (случай 18).
func TestEmptyStreamScrolls(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "a\n")
	commitAll(t, repo)
	model := resize(open(t, allFiles(repo)), 100, 20)

	for _, press := range []tea.KeyPressMsg{key('j'), key('G'), ctrlD, key(']'), key('['), key('v'), key('a'), key('a')} {
		model = pressAndSettle(model, press)
	}
	if body := screenText(model); !strings.Contains(body, "No changes") {
		t.Fatalf("пустая лента без надписи:\n%s", body)
	}
}

// Сдвиг вбок у ленты общий: прокрутка через файл с короткими строками его не
// сбрасывает, полоса прокрутки остаётся (случай 19).
func TestShiftSurvivesShortFile(t *testing.T) {
	repo := newRepo(t)
	long := "start" + strings.Repeat(" x", 100) + "\n"
	write(t, repo, "a.txt", "a\n")
	write(t, repo, "b.txt", numberedLines(30))
	commitAll(t, repo)
	write(t, repo, "a.txt", long)
	write(t, repo, "b.txt", strings.Replace(numberedLines(30), "l1\n", "L1\n", 1))
	opts := allFiles(repo)
	opts.Config.Diff.Wrap = false
	model := resize(open(t, opts), 100, 16)

	for range 3 {
		model, _ = model.Update(right)
	}
	for range 12 {
		model, _ = model.Update(key('j'))
	}
	if header := headerText(screenOf(model)); !strings.Contains(header, "b.txt") {
		t.Fatalf("наверху не b.txt: %q", header)
	}
	if !strings.Contains(screenText(model), "▄") {
		t.Fatalf("у файла с короткими строками пропала полоса:\n%s", screenText(model))
	}
	for range 12 {
		model, _ = model.Update(key('k'))
	}

	if body := screenText(model); strings.Contains(body, "start") || !strings.Contains(body, "x x") {
		t.Fatalf("сдвиг сбросился:\n%s", body)
	}
}

// Клик по текущему файлу из его середины ставит наверх его заголовок, левая
// стрелка заголовка работает как [ (случай 20).
func TestClickCurrentFileAndArrows(t *testing.T) {
	repo := streamFiles(t)
	opts := allFiles(repo)
	opts.Layout = layout.Default()
	model := toggleFiles(resize(open(t, opts), 120, 30))

	model, _ = model.Update(key(']'))
	for range 4 {
		model, _ = model.Update(key('j'))
	}
	model = clickPanel(t, model, "b.txt")
	if got := streamTop(t, model); got != "b.txt +2 -2" {
		t.Fatalf("клик по текущему файлу поставил наверх %q", got)
	}

	model, _ = model.Update(key('j'))
	for _, want := range []string{"b.txt +2 -2", "a.txt +1 -1"} {
		model = clickOn(t, model, "‹")
		if got := streamTop(t, model); got != want {
			t.Fatalf("после ‹ наверху %q, ожидался %q", got, want)
		}
	}
}

// streamTop это заголовок файла наверху ленты, когда слева открыта панель:
// в строке под пустой верхней он стоит правее имени из панели.
func streamTop(t *testing.T, model tea.Model) string {
	t.Helper()

	screen := screenOf(model)
	if strings.TrimSpace(screen[bodyRow]) != "" {
		return ""
	}
	return streamHeader.FindString(screen[bodyRow+1])
}

var streamHeader = regexp.MustCompile(`\S+ \+\d+ -\d+`)

// Имя с управляющими символами в заголовке ленты выводится значками
// (случай 21).
func TestStreamHeaderIsPrintable(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, hostileName, "old\n")
	commitAll(t, repo)
	write(t, repo, hostileName, "new\n")
	model := resize(open(t, allFiles(repo)), 100, 20)

	if got := topFile(screenOf(model)); got != shownName+" +1 -1" {
		t.Errorf("заголовок файла в ленте %q, ожидался %q", got, shownName+" +1 -1")
	}
	noEscapes(t, model)
}

// Бинарный файл, смена прав, переименование без правок и конфликт без хунков
// стоят в ленте со своими надписями, а без изменённых строк у заголовка нет
// счётчиков «+0 -0» (случаи 22 и 24).
func TestSpecialFilesInStream(t *testing.T) {
	repo, _ := conflictRepo(t)
	runGit(t, repo, "merge", "--abort")
	write(t, repo, "pic.bin", "old\x00")
	write(t, repo, "run.sh", "echo\n")
	write(t, repo, "old.txt", "same\n")
	commitAll(t, repo)
	write(t, repo, "pic.bin", "new\x00")
	if err := os.Chmod(filepath.Join(repo, "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "mv", "old.txt", "new.txt")
	model := resize(open(t, allFiles(repo)), 100, 60)

	body := screenText(model)
	for _, want := range []string{"pic.bin", "Binary or large file", "run.sh", "mode 644 → 755", "old.txt → new.txt"} {
		if !strings.Contains(body, want) {
			t.Errorf("в ленте нет %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "+0 -0") {
		t.Errorf("в ленте счётчики +0 -0:\n%s", body)
	}
}

// Конфликт без хунков (бинарный файл в слиянии) тоже стоит в ленте.
func TestConflictWithoutHunksInStream(t *testing.T) {
	repo, _ := conflictRepo(t)
	model := resize(open(t, allFiles(repo)), 100, 60)

	if body := screenText(model); !strings.Contains(body, "b.bin") {
		t.Fatalf("в ленте нет конфликта без хунков b.bin:\n%s", body)
	}
}

// a держит место в обе стороны: начало файла становится его заголовком,
// строка в середине остаётся наверху (случай 12).
func TestAllFilesKeyKeepsPlace(t *testing.T) {
	repo := streamFiles(t)
	model := resize(open(t, options(repo, layout.Focus())), 100, 20)
	model, _ = model.Update(key(']'))

	model, _ = model.Update(key('a'))
	if got := topFile(screenOf(model)); got != "b.txt +2 -2" {
		t.Fatalf("с начала файла лента поставила наверх %q, ожидался его заголовок", got)
	}
	model, _ = model.Update(key('a'))
	if got := topNumber(t, screenOf(model)); got != 2 {
		t.Fatalf("без ленты наверху строка %d, ожидалось начало файла", got)
	}

	for range 3 {
		model, _ = model.Update(key('j'))
	}
	want := topNumber(t, screenOf(model))
	model, _ = model.Update(key('a'))
	if got := topNumber(t, screenOf(model)); got != want {
		t.Fatalf("в ленте наверху строка %d, ожидалась %d", got, want)
	}
	model, _ = model.Update(key('a'))
	if got := topNumber(t, screenOf(model)); got != want {
		t.Fatalf("после ленты наверху строка %d, ожидалась %d", got, want)
	}
}

// Первый кадр ленты на большом диффе: 300 файлов по 50 правленых строк.
// Порога времени нет, CI с ним флакует; бенчмарк показывает цену
// (случай 9).
func BenchmarkStreamFirstFrame(b *testing.B) {
	repo := newRepo(b)
	for index := range 300 {
		write(b, repo, "f"+strconv.Itoa(index)+".go", numberedLines(100))
	}
	commitAll(b, repo)
	for index := range 300 {
		content := numberedLines(100)
		for line := 1; line <= 100; line += 2 {
			content = strings.Replace(content, fmt.Sprintf("l%d\n", line), fmt.Sprintf("L%d changed\n", line), 1)
		}
		write(b, repo, "f"+strconv.Itoa(index)+".go", content)
	}
	opts := allFiles(repo)

	for b.Loop() {
		resize(open(b, opts), 160, 50).View()
	}
}

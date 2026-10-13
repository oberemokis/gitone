package tui_test

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

// Переход к другому файлу открывает его с первой строки, а не с прокрутки
// прежнего.
func TestNewFileScrollsToTop(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "x\n")
	write(t, repo, "b.txt", "x\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", numberedLines(60))
	write(t, repo, "b.txt", numberedLines(60))

	model := openView(t, repo, "unified", 100, nil)
	model = scrollDown(model, 10)
	if got := bodyRows(model, 1)[0]; got == "  1 - x" {
		t.Fatalf("после прокрутки наверху всё ещё первая строка: %q", got)
	}

	model, _ = model.Update(key(']'))

	if got := bodyRows(model, 1)[0]; got != "  1 - x" {
		t.Errorf("наверху нового файла %q, want «  1 - x»", got)
	}
}

// Перетаскивание стыка меняет число строк экрана, потому что перенос идёт по
// ширине половин, но верхняя строка файла остаётся той же. Каждая строка файла
// занимает три строки экрана, после стыка вправо шесть: без возврата на место
// наверху оказалась бы строка 6, а не 11.
func TestDragSplitKeepsTopLine(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", numberedLines(60))
	commitAll(t, repo)

	var edited strings.Builder
	for number := 1; number <= 60; number++ {
		fmt.Fprintf(&edited, "L%02d%s\n", number, strings.Repeat("x", 127))
	}
	write(t, repo, "a.txt", edited.String())

	model := scrollDown(openView(t, repo, "split", 140, nil), 30)
	if got := topNumber(t, screenOf(model)); got != 11 {
		t.Fatalf("до перетаскивания наверху строка %d, want 11", got)
	}

	model = dragMouse(model, 70, 110, bodyRow+3)

	if got := topNumber(t, screenOf(model)); got != 11 {
		t.Errorf("после перетаскивания стыка наверху строка %d, want 11", got)
	}
}

// Сдвиг не остаётся дальше края после того, как окно стало шире: короткий
// остаток самой длинной строки встаёт к правому краю, а не уходит за него.
func TestShiftFollowsWindowWidening(t *testing.T) {
	model := longLine(t, noWrap)
	for range 14 {
		model, _ = model.Update(right)
	}

	// код шире на 50 колонок: 145 колонок кода, предельный сдвиг 55
	model = resize(model, 150, 20)

	if got, want := addedRow(model), " 1 + "+digits[55:]; got != want {
		t.Errorf("после расширения строка %q, want %q", got, want)
	}
}

// Предельный сдвиг считается по самой длинной строке любой из сторон split:
// длинная строка бывает только справа или только слева.
func TestShiftCoversLongerSideOnly(t *testing.T) {
	tests := []struct {
		name, old, new, want string
	}{
		{name: "длинная справа", old: "keep\n", new: "keep\n" + digits + "\n", want: strings.Repeat(" ", 60) + " 2 + " + digits[8:63]},
		{name: "длинная слева", old: "keep\n" + digits + "\n", new: "keep\n", want: " 2 - " + digits[8:63]},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			write(t, repo, "a.txt", tc.old)
			commitAll(t, repo)
			write(t, repo, "a.txt", tc.new)

			model := openView(t, repo, "split", 120, noWrap)
			model, _ = model.Update(right)

			// на код в половине 55 колонок, после стрелки он с колонки 8
			if got := bodyRows(model, 2)[1]; got != tc.want {
				t.Errorf("строка %q, want %q", got, tc.want)
			}
		})
	}
}

// Последняя строка файла с длинной строкой не прячется под полосой прокрутки:
// полоса забирает нижнюю строку панели, и прокрутка до конца считает это.
func TestLastLineStaysAboveScrollbar(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "x\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", numberedLines(39)+digits+"\n")

	model := openView(t, repo, "unified", 100, noWrap)
	model, _ = model.Update(key('G'))

	if row := rowOf(screenOf(model), " 40 + 0123456789"); row < 0 {
		t.Errorf("последней строки файла нет на экране:\n%s", screenText(model))
	}
	if bar := paneBottom(model); !strings.Contains(bar, "▄") {
		t.Errorf("внизу панели нет полосы прокрутки: %q", bar)
	}
}

// barRun это полоса из полублоков одного цвета: дорожка #010101 или ползунок
// #020202 (так их красит longLine и barView).
var barRun = regexp.MustCompile(`\x1b\[([0-9;]*)m(▄+)`)

// barRuns читает полосу прокрутки как ряд кусков «track 4», «thumb 47»: цвет и
// число клеток.
func barRuns(bar string) []string {
	names := map[string]string{"38;2;1;1;1": "track", "38;2;2;2;2": "thumb"}

	var runs []string
	for _, found := range barRun.FindAllStringSubmatch(bar, -1) {
		runs = append(runs, fmt.Sprintf("%s %d", names[found[1]], utf8.RuneCountInString(found[2])))
	}
	return runs
}

// Размер ползунка это видимая доля самой длинной строки, место это сдвиг, а на
// предельном сдвиге ползунок у правого края без округленного зазора.
func TestScrollbarThumbSizeAndPlace(t *testing.T) {
	// видимо 95 колонок кода из 200: ползунок 47 клеток из 100
	tests := []struct {
		name    string
		presses int
		want    []string
	}{
		{name: "в начале", presses: 0, want: []string{"thumb 47", "track 53"}},
		{name: "после сдвига на 8", presses: 1, want: []string{"track 4", "thumb 47", "track 49"}},
		{name: "на предельном сдвиге", presses: 14, want: []string{"track 53", "thumb 47"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model := longLine(t, noWrap)
			for range tc.presses {
				model, _ = model.Update(right)
			}

			if got := barRuns(paneBottom(model)); !slices.Equal(got, tc.want) {
				t.Errorf("полоса %q, want %q", got, tc.want)
			}
		})
	}
}

// barSplit открывает split с одной строкой в 120 колонок кода на каждой
// стороне, без переноса, на экране шириной width. Дорожка #010101, ползунок
// #020202.
func barSplit(t *testing.T, width int) tea.Model {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "a"+digits[:119]+"\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "b"+digits[:119]+"\n")

	custom := theme.Dark()
	custom.BackgroundPanel = lipgloss.Color("#010101")
	custom.TextMuted = lipgloss.Color("#020202")
	opts := options(repo, layout.Focus())
	opts.Theme = custom
	opts.Config.Diff.Mode = "split"
	opts.Config.Diff.Wrap = false
	return resize(open(t, opts), width, 20)
}

// Ползунок считается по более узкой половине split: на нечётной ширине это
// левая, после перетаскивания стыка правая.
func TestScrollbarFollowsNarrowerHalf(t *testing.T) {
	// 121 колонка: половины 60 и 61, на код 55 и 56; видимо 55 из 120
	if got, want := barRuns(paneBottom(barSplit(t, 121))), []string{"thumb 55", "track 66"}; !slices.Equal(got, want) {
		t.Errorf("полоса на нечётной ширине %q, want %q", got, want)
	}

	// стык с 60 на 80: половины 80 и 40, на код 75 и 35; видимо 35 из 120
	dragged := dragMouse(barSplit(t, 120), 60, 80, bodyRow+3)
	if got, want := barRuns(paneBottom(dragged)), []string{"thumb 35", "track 85"}; !slices.Equal(got, want) {
		t.Errorf("полоса после перетаскивания стыка %q, want %q", got, want)
	}
}

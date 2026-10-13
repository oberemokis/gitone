package tui_test

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/config"
	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

// digits это строка из 200 цифр 0123456789…: по цифре видно, с какой колонки
// начинается видимая часть.
var digits = strings.Repeat("0123456789", 20)

var (
	right = tea.KeyPressMsg{Code: tea.KeyRight}
	left  = tea.KeyPressMsg{Code: tea.KeyLeft}
)

// noWrap выключает перенос, включённый по умолчанию.
func noWrap(cfg *config.Config) {
	cfg.Diff.Wrap = false
}

// longLine открывает дифф, где x заменён строкой из 200 цифр, в unified на
// экране 100 на 20. Полоса прокрутки красится отличимыми цветами: дорожка
// #010101 (38;2;1;1;1), ползунок #020202 (38;2;2;2;2).
func longLine(t *testing.T, change func(*config.Config)) tea.Model {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "x\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", digits+"\n")

	custom := theme.Dark()
	custom.BackgroundPanel = lipgloss.Color("#010101")
	custom.TextMuted = lipgloss.Color("#020202")

	opts := options(repo, layout.Focus())
	opts.Theme = custom
	opts.Config.Diff.Mode = "unified"
	if change != nil {
		change(&opts.Config)
	}
	return resize(open(t, opts), 100, 20)
}

// paneBottom это последняя строка панели диффа в сыром выводе: над пустой
// строкой, подвалом и пустой строкой под ним.
func paneBottom(model tea.Model) string {
	lines := strings.Split(model.View().Content, "\n")
	return lines[len(lines)-4]
}

// addedRow это строка экрана с добавленной строкой: вторая строка тела.
func addedRow(model tea.Model) string {
	return screenOf(model)[bodyRow+1]
}

// Без переноса строка шире панели: видно начало, внизу полоса из полублоков
// с ползунком у левого края. Фон клетки полоса не красит: она вдвое тоньше
// строки.
func TestLongLineShowsScrollbar(t *testing.T) {
	model := longLine(t, noWrap)

	if row := addedRow(model); !strings.Contains(row, "+ 0123456789") {
		t.Fatalf("строка не с начала: %q", row)
	}
	bar := paneBottom(model)
	thumb, track := strings.Index(bar, "38;2;2;2;2"), strings.Index(bar, "38;2;1;1;1")
	if thumb < 0 || track < 0 || thumb > track {
		t.Fatalf("внизу нет полосы с ползунком слева: %q", bar)
	}
	if strings.Contains(bar, "48;") || strings.Trim(ansi.Strip(bar), "▄") != "" {
		t.Fatalf("полоса не из полублоков без фона: %q", bar)
	}
}

// Стрелки сдвигают код на 8 колонок, номера и знак остаются на месте.
func TestArrowsShiftCode(t *testing.T) {
	model := longLine(t, noWrap)

	model, _ = model.Update(right)
	if row := addedRow(model); !strings.HasPrefix(row, " 1 + 8901234567") {
		t.Fatalf("после → строка %q, want код с колонки 8", row)
	}

	model, _ = model.Update(left)
	if row := addedRow(model); !strings.HasPrefix(row, " 1 + 0123456789") {
		t.Fatalf("после ← строка %q, want код с начала", row)
	}
}

// Сдвиг не уходит левее начала и дальше конца самой длинной строки.
func TestShiftStopsAtEdges(t *testing.T) {
	model := longLine(t, noWrap)

	model, _ = model.Update(left)
	if row := addedRow(model); !strings.HasPrefix(row, " 1 + 0123456789") {
		t.Fatalf("← в начале сдвинул строку: %q", row)
	}

	for range 30 {
		model, _ = model.Update(right)
	}
	end := addedRow(model)
	if !strings.HasSuffix(end, "0123456789") {
		t.Fatalf("в конце строка %q, want последние цифры у правого края", end)
	}
	if model, _ = model.Update(right); addedRow(model) != end {
		t.Fatalf("→ в конце сдвинул строку дальше: %q", addedRow(model))
	}
}

// По умолчанию строка переносится: продолжение идёт следующей строкой без
// номера и знака, полосы нет.
func TestWrapByDefault(t *testing.T) {
	model := longLine(t, nil)

	// колонка кода 100 - 5 = 95, продолжение начинается с цифры 95 % 10 = 5
	if next := screenOf(model)[bodyRow+2]; !strings.HasPrefix(next, "     5678901234") {
		t.Fatalf("продолжение строки %q, want с цифры 5 без номера", next)
	}
	if bar := paneBottom(model); strings.Contains(bar, "▄") {
		t.Fatalf("полоса есть при переносе: %q", bar)
	}
}

// w выключает перенос, второй w включает обратно.
func TestWKeyTogglesWrap(t *testing.T) {
	model := longLine(t, nil)

	model, _ = model.Update(key('w'))
	if bar := paneBottom(model); !strings.Contains(bar, "▄") {
		t.Fatalf("после w нет полосы прокрутки: %q", bar)
	}

	model, _ = model.Update(key('w'))
	if next := screenOf(model)[bodyRow+2]; !strings.HasPrefix(next, "     5678901234") {
		t.Fatalf("второй w не вернул перенос: %q", next)
	}
}

// Все строки помещаются: полосы нет.
func TestShortLinesHaveNoScrollbar(t *testing.T) {
	_, model := threeFileRepo(t)

	if bar := paneBottom(model); strings.TrimSpace(bar) != "" {
		t.Fatalf("у короткого файла внизу %q, want пустая строка", bar)
	}
}

// split: обе колонки сдвигаются вместе.
func TestSplitShiftsBothSides(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "a"+digits+"\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "b"+digits+"\n")

	opts := options(repo, layout.Focus())
	opts.Config.Diff.Mode = "split"
	opts.Config.Diff.Wrap = false
	model := resize(open(t, opts), 120, 20)

	model, _ = model.Update(right)
	if row := screenOf(model)[bodyRow]; !strings.Contains(row, "- 7890123") || !strings.Contains(row, "+ 7890123") {
		t.Fatalf("после → колонки сдвинулись не вместе: %q", row)
	}
}

// Новый файл открывается без сдвига.
func TestNewFileResetsShift(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "x\n")
	write(t, repo, "b.txt", "x\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", digits+"\n")
	write(t, repo, "b.txt", digits+"\n")

	opts := options(repo, layout.Focus())
	opts.Config.Diff.Mode = "unified"
	opts.Config.Diff.Wrap = false
	model := resize(open(t, opts), 100, 20)

	for _, press := range []tea.KeyPressMsg{right, key(']'), key('[')} {
		model, _ = model.Update(press)
	}
	if row := addedRow(model); !strings.HasPrefix(row, " 1 + 0123456789") {
		t.Fatalf("после смены файла строка %q, want без сдвига", row)
	}
}

// narrowDiff открывает дифф, где x заменён строками lines, в unified на экране
// шириной 25: номер и знак занимают 5 колонок, на код остаётся 20.
func narrowDiff(t *testing.T, change func(*config.Config), lines ...string) tea.Model {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "x\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", strings.Join(lines, "\n")+"\n")

	opts := options(repo, layout.Focus())
	opts.Config.Diff.Mode = "unified"
	if change != nil {
		change(&opts.Config)
	}
	return resize(open(t, opts), 25, 20)
}

// Широкий символ на границе куска не пропадает: он не помещается в хвост
// куска и целиком уходит в начало следующего.
func TestWrapKeepsWideCharacterAtChunkEdge(t *testing.T) {
	model := narrowDiff(t, nil, strings.Repeat("a", 19)+"世"+strings.Repeat("b", 18)+"XYccc")

	want := []string{
		" 1 - x",
		" 1 + " + strings.Repeat("a", 19),
		"     世" + strings.Repeat("b", 18),
		"     XYccc",
	}
	if got := bodyRows(model, len(want)); !slices.Equal(got, want) {
		t.Errorf("строки диффа %q, want %q", got, want)
	}
}

// В split куски левой и правой половины начинаются в разных колонках: слева
// 世 не влезает в первый кусок, справа ровный ряд символов режется по ширине.
func TestSplitWrapCutsHalvesIndependently(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", strings.Repeat("a", 19)+"世bbb\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", strings.Repeat("c", 30)+"\n")

	opts := options(repo, layout.Focus())
	opts.Config.Diff.Mode = "split"
	model := resize(open(t, opts), 50, 20)

	// половина 25 колонок: номер и знак 5, на код 20
	want := []string{
		" 1 - " + strings.Repeat("a", 19) + "  1 + " + strings.Repeat("c", 20),
		"     世bbb" + strings.Repeat(" ", 15) + "     " + strings.Repeat("c", 10),
	}
	if got := bodyRows(model, len(want)); !slices.Equal(got, want) {
		t.Errorf("строки диффа %q, want %q", got, want)
	}
}

// Без переноса после сдвига на 8 колонок широкий символ на краю окна
// заменяется пробелом: остаток строки стоит в тех же колонках, что у соседних
// строк. Девятка в колонке 9 и Z в колонке 25 второй строки должны стоять
// под девяткой и пятёркой линейки в первой.
func TestShiftReplacesWideCharacterAtLeftEdgeWithSpace(t *testing.T) {
	model := narrowDiff(t, noWrap,
		"0123456789"+"0123456789"+"0123456789",
		"aaaaaaa世9"+strings.Repeat("b", 15)+"Z"+"bbbb",
	)

	model, _ = model.Update(right)

	want := []string{
		" 1 + 89012345678901234567",
		" 2 +  9bbbbbbbbbbbbbbbZbb",
	}
	if got := bodyRows(model, 3)[1:]; !slices.Equal(got, want) {
		t.Errorf("строки диффа %q, want %q", got, want)
	}
}

// Номер строки с отступом слева, в unified один столбец номеров: у строки
// контекста и добавленной новый номер, у удалённой старый.
func TestUnifiedSingleNumberColumn(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "keep\nold\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "keep\nnew\n")

	opts := options(repo, layout.Focus())
	opts.Config.Diff.Mode = "unified"
	screen := screenOf(resize(open(t, opts), 100, 20))

	for index, want := range []string{" 1   keep", " 2 - old", " 2 + new"} {
		if row := screen[bodyRow+index]; !strings.HasPrefix(row, want) {
			t.Errorf("строка %d = %q, want %q", index, row, want)
		}
	}
}

// В split у номера тоже отступ слева.
func TestSplitNumberPadding(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "old\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "new\n")

	opts := options(repo, layout.Focus())
	opts.Config.Diff.Mode = "split"
	row := screenOf(resize(open(t, opts), 120, 20))[bodyRow]

	if !strings.HasPrefix(row, " 1 - old") || !strings.Contains(row, " 1 + new") {
		t.Fatalf("строка split %q, want « 1 - old» и « 1 + new»", row)
	}
}

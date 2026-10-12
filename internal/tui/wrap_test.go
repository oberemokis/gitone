package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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

// longLine открывает дифф, где x заменён строкой из 200 цифр, в unified на
// экране 100 на 20. Полоса прокрутки красится отличимыми цветами: дорожка
// #010101 (48;2;1;1;1), ползунок #020202 (48;2;2;2;2).
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

// paneBottom это последняя строка панели диффа в сыром выводе: над подвалом.
func paneBottom(model tea.Model) string {
	lines := strings.Split(model.View().Content, "\n")
	return lines[len(lines)-2]
}

// addedRow это строка экрана с добавленной строкой: вторая строка тела.
func addedRow(model tea.Model) string {
	return screenOf(model)[bodyRow+1]
}

// Строка шире панели: видно начало, внизу полоса с ползунком у левого края.
func TestLongLineShowsScrollbar(t *testing.T) {
	model := longLine(t, nil)

	if row := addedRow(model); !strings.Contains(row, "+ 0123456789") {
		t.Fatalf("строка не с начала: %q", row)
	}
	bar := paneBottom(model)
	thumb, track := strings.Index(bar, "48;2;2;2;2"), strings.Index(bar, "48;2;1;1;1")
	if thumb < 0 || track < 0 || thumb > track {
		t.Fatalf("внизу нет полосы с ползунком слева: %q", bar)
	}
}

// Стрелки сдвигают код на 8 колонок, номера и знак остаются на месте.
func TestArrowsShiftCode(t *testing.T) {
	model := longLine(t, nil)

	model, _ = model.Update(right)
	if row := addedRow(model); !strings.HasPrefix(row, "  1 + 8901234567") {
		t.Fatalf("после → строка %q, want код с колонки 8", row)
	}

	model, _ = model.Update(left)
	if row := addedRow(model); !strings.HasPrefix(row, "  1 + 0123456789") {
		t.Fatalf("после ← строка %q, want код с начала", row)
	}
}

// Сдвиг не уходит левее начала и дальше конца самой длинной строки.
func TestShiftStopsAtEdges(t *testing.T) {
	model := longLine(t, nil)

	model, _ = model.Update(left)
	if row := addedRow(model); !strings.HasPrefix(row, "  1 + 0123456789") {
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

// w включает перенос: продолжение идёт следующей строкой без номера и знака,
// полосы нет.
func TestWrapContinuesLine(t *testing.T) {
	model := longLine(t, nil)

	model, _ = model.Update(key('w'))

	// колонка кода 100 - 6 = 94, продолжение начинается с цифры 94 % 10 = 4
	if next := screenOf(model)[bodyRow+2]; !strings.HasPrefix(next, "      4567890123") {
		t.Fatalf("продолжение строки %q, want с цифры 4 без номера", next)
	}
	if bar := paneBottom(model); strings.Contains(bar, "48;2;2;2;2") {
		t.Fatalf("полоса осталась при переносе: %q", bar)
	}
}

// diff.wrap в config.json: запуск с переносом.
func TestWrapFromConfig(t *testing.T) {
	model := longLine(t, func(cfg *config.Config) { cfg.Diff.Wrap = true })

	if next := screenOf(model)[bodyRow+2]; !strings.HasPrefix(next, "      4567890123") {
		t.Fatalf("diff.wrap не включил перенос: %q", next)
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
	model := resize(open(t, opts), 100, 20)

	for _, press := range []tea.KeyPressMsg{right, key(']'), key('[')} {
		model, _ = model.Update(press)
	}
	if row := addedRow(model); !strings.HasPrefix(row, "  1 + 0123456789") {
		t.Fatalf("после смены файла строка %q, want без сдвига", row)
	}
}

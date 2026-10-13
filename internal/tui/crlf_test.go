package tui_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

// bodyRows это первые count строк тела без пробелов справа.
func bodyRows(model tea.Model, count int) []string {
	rows := slices.Clone(screenOf(model)[bodyRow : bodyRow+count])
	for index := range rows {
		rows[index] = strings.TrimRight(rows[index], " ")
	}

	return rows
}

// Правка, которая меняет только конец строки CRLF на LF, видна значком ␍ на
// удалённых строках: иначе в диффе стояли бы две пары одинаковых строк.
func TestEndingOnlyChangeShowsCarriageReturnMark(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\r\ntwo\r\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\ntwo\n")

	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	want := []string{" 1 - one␍", " 2 - two␍", " 1 + one", " 2 + two"}
	if got := bodyRows(model, len(want)); !slices.Equal(got, want) {
		t.Errorf("строки диффа %q, want %q", got, want)
	}
}

func TestEndingOnlyChangeShowsCarriageReturnMarkInSplit(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\r\n")

	opts := options(repo, layout.Focus())
	opts.Config.Diff.Mode = "split"
	model := resize(open(t, opts), 100, 20)

	// LF заменён на CRLF: значок у добавленной строки, правая половина
	want := " 1 - one" + strings.Repeat(" ", 42) + " 1 + one␍"
	if got := bodyRows(model, 1)[0]; got != want {
		t.Errorf("строка split %q, want %q", got, want)
	}
}

// Значок служебный, поэтому серый: цвет приглушённого текста темы. #020202 в
// ANSI это 38;2;2;2;2.
func TestCarriageReturnMarkIsMuted(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\r\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\n")

	custom := theme.Dark()
	custom.TextMuted = lipgloss.Color("#020202")
	opts := options(repo, layout.Focus())
	opts.Theme = custom

	raw := resize(open(t, opts), 100, 20).View().Content
	if !regexp.MustCompile(`38;2;2;2;2[;0-9]*m␍`).MatchString(raw) {
		t.Fatalf("значок не приглушён:\n%q", raw)
	}
}

// Обычный CRLF-файл не усыпан значками: ␍ нужен только там, где конец
// строки и есть отличие.
func TestOrdinaryCRLFFileHasNoCarriageReturnMarks(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\r\ntwo\r\nthree\r\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\r\nTWO\r\nthree\r\n")

	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	want := []string{" 1   one", " 2 - two", " 2 + TWO", " 3   three"}
	if got := bodyRows(model, len(want)); !slices.Equal(got, want) {
		t.Errorf("строки диффа %q, want %q", got, want)
	}
	if body := screenText(model); strings.Contains(body, "␍") {
		t.Errorf("на экране есть значок ␍:\n%s", body)
	}
}

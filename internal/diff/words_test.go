package diff

import (
	"slices"
	"strings"
	"testing"
)

// spanTexts возвращает текст отрезков, чтобы тест читался по словам, а не по
// смещениям.
func spanTexts(text string, spans []Span) []string {
	var texts []string
	for _, span := range spans {
		texts = append(texts, text[span.Start:span.End])
	}
	return texts
}

func TestWordSpans(t *testing.T) {
	tests := []struct {
		name    string
		old     string
		new     string
		wantOld []string
		wantNew []string
	}{
		{name: "одно слово", old: "a foo b", new: "a bar b", wantOld: []string{"foo"}, wantNew: []string{"bar"}},
		{name: "знаки считаются токенами", old: "a,b", new: "a;b", wantOld: []string{","}, wantNew: []string{";"}},
		{name: "строки совпадают", old: "same text", new: "same text"},
		{name: "переписана целиком", old: "alpha beta gamma", new: "x y z"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldSpans, newSpans := wordSpans(tc.old, tc.new)

			if got := spanTexts(tc.old, oldSpans); !slices.Equal(got, tc.wantOld) {
				t.Fatalf("слева подсвечено %q, ожидалось %q", got, tc.wantOld)
			}
			if got := spanTexts(tc.new, newSpans); !slices.Equal(got, tc.wantNew) {
				t.Fatalf("справа подсвечено %q, ожидалось %q", got, tc.wantNew)
			}
		})
	}
}

// В unified строка получает отрезки от своей пары, а лишние удалённые строки
// без пары подсветки не получают.
func TestUnifiedMarksPairedLinesOnly(t *testing.T) {
	file := File{Hunks: []Hunk{{
		OldStart: 1,
		NewStart: 1,
		Lines: []Line{
			{Kind: LineRemoved, Text: "x one"},
			{Kind: LineRemoved, Text: "x two"},
			{Kind: LineRemoved, Text: "x three"},
			{Kind: LineAdded, Text: "x uno"},
			{Kind: LineAdded, Text: "x dos"},
		},
	}}}

	rows := Unified(file)
	if len(rows) != 5 {
		t.Fatalf("строк %d, ожидалось 5", len(rows))
	}
	if got := spanTexts(rows[0].Text, rows[0].Changed); !slices.Equal(got, []string{"one"}) {
		t.Fatalf("первая пара слева %q, ожидалось one", got)
	}
	if got := spanTexts(rows[3].Text, rows[3].Changed); !slices.Equal(got, []string{"uno"}) {
		t.Fatalf("первая пара справа %q, ожидалось uno", got)
	}
	if len(rows[2].Changed) != 0 {
		t.Fatalf("лишняя удалённая строка подсвечена: %v", rows[2].Changed)
	}
}

func TestSplitMarksPairedCells(t *testing.T) {
	file := File{Hunks: []Hunk{{
		OldStart: 1,
		NewStart: 1,
		Lines: []Line{
			{Kind: LineRemoved, Text: "x one"},
			{Kind: LineRemoved, Text: "x three"},
			{Kind: LineAdded, Text: "x uno"},
		},
	}}}

	rows := Split(file)
	if got := spanTexts(rows[0].Left.Text, rows[0].Left.Changed); !slices.Equal(got, []string{"one"}) {
		t.Fatalf("пара слева %q, ожидалось one", got)
	}
	if got := spanTexts(rows[0].Right.Text, rows[0].Right.Changed); !slices.Equal(got, []string{"uno"}) {
		t.Fatalf("пара справа %q, ожидалось uno", got)
	}
	if len(rows[1].Left.Changed) != 0 {
		t.Fatalf("строка без пары подсвечена: %v", rows[1].Left.Changed)
	}
}

// Длинная строка не считается: ограничение памяти важнее подсветки.
func TestLongLinesGetNoMarks(t *testing.T) {
	long := strings.Repeat("w ", 400)
	oldSpans, newSpans := wordSpans(long, long+"tail")

	if len(oldSpans) != 0 || len(newSpans) != 0 {
		t.Fatalf("у длинной строки есть отрезки: %d и %d", len(oldSpans), len(newSpans))
	}
}

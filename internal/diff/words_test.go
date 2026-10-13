package diff_test

import (
	"slices"
	"strings"
	"testing"

	"gitone/internal/diff"
)

// spanTexts возвращает текст отрезков, чтобы тест читался по словам, а не по
// смещениям.
func spanTexts(text string, spans []diff.Span) []string {
	var texts []string
	for _, span := range spans {
		texts = append(texts, text[span.Start:span.End])
	}
	return texts
}

// change это хунк из одной удалённой и одной добавленной строки: в split они
// встают парой в одну строку.
func change(oldText, newText string) diff.File {
	return diff.File{Hunks: []diff.Hunk{{OldStart: 1, NewStart: 1, Lines: []diff.Line{removed(oldText), added(newText)}}}}
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
		// граница отрезка не заходит внутрь графемного кластера
		{
			name:    "эмодзи из ZWJ-последовательности подсвечивается целиком",
			old:     "x 👨‍👩‍👧 y",
			new:     "x 👨‍👩‍👦 y",
			wantOld: []string{"👨‍👩‍👧"},
			wantNew: []string{"👨‍👩‍👦"},
		},
		{
			name:    "буква с комбинирующим знаком NFD подсвечивается целиком",
			old:     "a café b",
			new:     "a cafe b",
			wantOld: []string{"café"},
			wantNew: []string{"cafe"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows := diff.Split(change(tc.old, tc.new))
			if len(rows) != 1 {
				t.Fatalf("Split(-%q +%q) = %d rows, want 1", tc.old, tc.new, len(rows))
			}

			left, right := rows[0].Left, rows[0].Right
			if got := spanTexts(left.Text, left.Changed); !slices.Equal(got, tc.wantOld) {
				t.Errorf("Split(-%q +%q).Left.Changed = %q, want %q", tc.old, tc.new, got, tc.wantOld)
			}
			if got := spanTexts(right.Text, right.Changed); !slices.Equal(got, tc.wantNew) {
				t.Errorf("Split(-%q +%q).Right.Changed = %q, want %q", tc.old, tc.new, got, tc.wantNew)
			}
		})
	}
}

// В unified строка получает отрезки от своей пары, а лишние удалённые строки
// без пары подсветки не получают.
func TestUnifiedMarksPairedLinesOnly(t *testing.T) {
	file := diff.File{Hunks: []diff.Hunk{{
		OldStart: 1,
		NewStart: 1,
		Lines:    []diff.Line{removed("x one"), removed("x two"), removed("x three"), added("x uno"), added("x dos")},
	}}}

	rows := diff.Unified(file)
	if len(rows) != 5 {
		t.Fatalf("строк %d, ожидалось 5", len(rows))
	}
	if got := spanTexts(rows[0].Text, rows[0].Changed); !slices.Equal(got, []string{"one"}) {
		t.Errorf("первая пара слева %q, ожидалось one", got)
	}
	if got := spanTexts(rows[3].Text, rows[3].Changed); !slices.Equal(got, []string{"uno"}) {
		t.Errorf("первая пара справа %q, ожидалось uno", got)
	}
	if len(rows[2].Changed) != 0 {
		t.Errorf("лишняя удалённая строка подсвечена: %v", rows[2].Changed)
	}
}

func TestSplitMarksPairedCells(t *testing.T) {
	file := diff.File{Hunks: []diff.Hunk{{
		OldStart: 1,
		NewStart: 1,
		Lines:    []diff.Line{removed("x one"), removed("x three"), added("x uno")},
	}}}

	rows := diff.Split(file)
	if len(rows) != 2 {
		t.Fatalf("строк %d, ожидалось 2", len(rows))
	}
	if got := spanTexts(rows[0].Left.Text, rows[0].Left.Changed); !slices.Equal(got, []string{"one"}) {
		t.Errorf("пара слева %q, ожидалось one", got)
	}
	if got := spanTexts(rows[0].Right.Text, rows[0].Right.Changed); !slices.Equal(got, []string{"uno"}) {
		t.Errorf("пара справа %q, ожидалось uno", got)
	}
	if len(rows[1].Left.Changed) != 0 {
		t.Errorf("строка без пары подсвечена: %v", rows[1].Left.Changed)
	}
}

// Правило пар одно на оба вида: удалённые строки блока встают в пары с
// добавленными по порядку, пока не встретится контекст. На чередовании
// -p +p +q -q +r вторая удалённая строка стоит напротив второй добавленной и
// в split, и в unified. Git такой хунк не выдаёт, но разбор принимает любой
// текст, а правка одного из видов не должна молча развести подсветку.
func TestSplitAndUnifiedPairInterleavedLinesTheSame(t *testing.T) {
	file := diff.File{Hunks: []diff.Hunk{{
		OldStart: 1,
		NewStart: 1,
		Lines:    []diff.Line{removed("p one"), added("p uno"), added("q dos"), removed("q two"), added("r tres")},
	}}}

	split := diff.Split(file)
	if len(split) != 3 {
		t.Fatalf("строк split %d, ожидалось 3", len(split))
	}
	unified := diff.Unified(file)
	if len(unified) != 5 {
		t.Fatalf("строк unified %d, ожидалось 5", len(unified))
	}

	marked := func(text string, spans []diff.Span) string {
		return strings.Join(spanTexts(text, spans), ",")
	}

	// порядок: p one, p uno, q two, q dos
	want := []string{"one", "uno", "two", "dos"}
	gotSplit := []string{
		marked(split[0].Left.Text, split[0].Left.Changed),
		marked(split[0].Right.Text, split[0].Right.Changed),
		marked(split[1].Left.Text, split[1].Left.Changed),
		marked(split[1].Right.Text, split[1].Right.Changed),
	}
	gotUnified := []string{
		marked(unified[0].Text, unified[0].Changed),
		marked(unified[1].Text, unified[1].Changed),
		marked(unified[3].Text, unified[3].Changed),
		marked(unified[2].Text, unified[2].Changed),
	}

	if !slices.Equal(gotSplit, want) {
		t.Errorf("отрезки split %q, ожидалось %q", gotSplit, want)
	}
	if !slices.Equal(gotUnified, want) {
		t.Errorf("отрезки unified %q, ожидалось %q", gotUnified, want)
	}
	if len(split[2].Right.Changed) != 0 || len(unified[4].Changed) != 0 {
		t.Errorf("добавленная строка без пары подсвечена: split %v, unified %v", split[2].Right.Changed, unified[4].Changed)
	}
}

// Длинная строка не считается: ограничение памяти важнее подсветки.
func TestLongLinesGetNoMarks(t *testing.T) {
	long := strings.Repeat("w ", 400)

	rows := diff.Split(change(long, long+"tail"))
	if len(rows) != 1 {
		t.Fatalf("строк %d, ожидалась 1", len(rows))
	}

	left, right := rows[0].Left, rows[0].Right
	if len(left.Changed) != 0 || len(right.Changed) != 0 {
		t.Errorf("у длинной строки есть отрезки: %d и %d", len(left.Changed), len(right.Changed))
	}
}

package diff_test

import (
	"reflect"
	"testing"

	"gitone/internal/diff"
)

func TestUnified(t *testing.T) {
	cases := []struct {
		name string
		file diff.File
		want []diff.UnifiedRow
	}{
		{
			name: "номера строк идут по своей стороне, между хунками один разрыв",
			file: diff.File{Hunks: []diff.Hunk{
				{OldStart: 1, NewStart: 1, Lines: []diff.Line{context("a"), removed("b"), added("B"), context("c")}},
				{OldStart: 20, NewStart: 20, Lines: []diff.Line{added("new"), context("z")}},
			}},
			want: []diff.UnifiedRow{
				{Kind: diff.RowLine, Line: diff.LineContext, OldNumber: 1, NewNumber: 1, Text: "a"},
				{Kind: diff.RowLine, Line: diff.LineRemoved, OldNumber: 2, Text: "b"},
				{Kind: diff.RowLine, Line: diff.LineAdded, NewNumber: 2, Text: "B"},
				{Kind: diff.RowLine, Line: diff.LineContext, OldNumber: 3, NewNumber: 3, Text: "c"},
				{Kind: diff.RowGap},
				{Kind: diff.RowLine, Line: diff.LineAdded, NewNumber: 20, Text: "new"},
				{Kind: diff.RowLine, Line: diff.LineContext, OldNumber: 20, NewNumber: 21, Text: "z"},
			},
		},
		{
			name: "строка без перевода в конце даёт следом строку-пометку",
			file: diff.File{Hunks: []diff.Hunk{{OldStart: 1, NewStart: 1, Lines: []diff.Line{
				{Kind: diff.LineRemoved, Text: "tail", NoNewline: true},
				added("tail"),
			}}}},
			want: []diff.UnifiedRow{
				{Kind: diff.RowLine, Line: diff.LineRemoved, OldNumber: 1, Text: "tail"},
				{Kind: diff.RowNoNewline, Line: diff.LineRemoved},
				{Kind: diff.RowLine, Line: diff.LineAdded, NewNumber: 1, Text: "tail"},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := diff.Unified(testCase.file)

			if !reflect.DeepEqual(got, testCase.want) {
				t.Fatalf("получили\n%+v\nожидалось\n%+v", got, testCase.want)
			}
		})
	}
}

func TestSplit(t *testing.T) {
	cases := []struct {
		name string
		file diff.File
		want []diff.SplitRow
	}{
		{
			name: "три удалённые и две добавленные дают две пары и удалённую без пары",
			file: diff.File{Hunks: []diff.Hunk{{OldStart: 10, NewStart: 10, Lines: []diff.Line{
				context("top"), removed("r1"), removed("r2"), removed("r3"), added("a1"), added("a2"), context("end"),
			}}}},
			want: []diff.SplitRow{
				line(cell(diff.LineContext, 10, "top"), cell(diff.LineContext, 10, "top")),
				line(cell(diff.LineRemoved, 11, "r1"), cell(diff.LineAdded, 11, "a1")),
				line(cell(diff.LineRemoved, 12, "r2"), cell(diff.LineAdded, 12, "a2")),
				line(cell(diff.LineRemoved, 13, "r3"), diff.Cell{}),
				line(cell(diff.LineContext, 14, "end"), cell(diff.LineContext, 13, "end")),
			},
		},
		{
			name: "только добавленные строки оставляют левую половину пустой",
			file: diff.File{Hunks: []diff.Hunk{{OldStart: 0, NewStart: 1, Lines: []diff.Line{added("n1"), added("n2")}}}},
			want: []diff.SplitRow{
				line(diff.Cell{}, cell(diff.LineAdded, 1, "n1")),
				line(diff.Cell{}, cell(diff.LineAdded, 2, "n2")),
			},
		},
		{
			name: "между хунками один разрыв, номера дальше идут с начала следующего хунка",
			file: diff.File{Hunks: []diff.Hunk{
				{OldStart: 1, NewStart: 1, Lines: []diff.Line{removed("2"), added("two")}},
				{OldStart: 18, NewStart: 18, Lines: []diff.Line{context("18"), removed("19")}},
			}},
			want: []diff.SplitRow{
				line(cell(diff.LineRemoved, 1, "2"), cell(diff.LineAdded, 1, "two")),
				{Kind: diff.RowGap},
				line(cell(diff.LineContext, 18, "18"), cell(diff.LineContext, 18, "18")),
				line(cell(diff.LineRemoved, 19, "19"), diff.Cell{}),
			},
		},
		{
			name: "пометка без перевода строки встаёт только на сторону своей строки",
			file: diff.File{Hunks: []diff.Hunk{{OldStart: 1, NewStart: 1, Lines: []diff.Line{
				{Kind: diff.LineRemoved, Text: "tail", NoNewline: true},
				added("tail"),
			}}}},
			want: []diff.SplitRow{
				line(cell(diff.LineRemoved, 1, "tail"), cell(diff.LineAdded, 1, "tail")),
				{Kind: diff.RowNoNewline, Left: diff.Cell{Kind: diff.LineRemoved}},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := diff.Split(testCase.file)

			if !reflect.DeepEqual(got, testCase.want) {
				t.Fatalf("получили\n%+v\nожидалось\n%+v", got, testCase.want)
			}
		})
	}
}

func line(left, right diff.Cell) diff.SplitRow {
	return diff.SplitRow{Kind: diff.RowLine, Left: left, Right: right}
}

func cell(kind diff.LineKind, number int, text string) diff.Cell {
	return diff.Cell{Kind: kind, Number: number, Text: text}
}

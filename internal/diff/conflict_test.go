package diff_test

import (
	"testing"

	"gitone/internal/diff"
)

const twoConflicts = `a
<<<<<<< HEAD
main one
=======
feat one
>>>>>>> feat
b
<<<<<<< HEAD
main two
||||||| base
base two
=======
feat two
>>>>>>> feat
c
`

func TestResolveConflict(t *testing.T) {
	cases := []struct {
		name  string
		start int
		side  diff.ConflictSide
		want  string
	}{
		{name: "наша", start: 2, side: diff.SideOurs, want: "a\nmain one\nb\n<<<<<<< HEAD\nmain two\n||||||| base\nbase two\n=======\nfeat two\n>>>>>>> feat\nc\n"},
		{name: "их", start: 2, side: diff.SideTheirs, want: "a\nfeat one\nb\n<<<<<<< HEAD\nmain two\n||||||| base\nbase two\n=======\nfeat two\n>>>>>>> feat\nc\n"},
		// основа diff3 выпадает
		{name: "обе в diff3", start: 8, side: diff.SideBoth, want: "a\n<<<<<<< HEAD\nmain one\n=======\nfeat one\n>>>>>>> feat\nb\nmain two\nfeat two\nc\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, found := diff.ResolveConflict(twoConflicts, tc.start, tc.side)
			if !found || got != tc.want {
				t.Fatalf("ResolveConflict = %q, %v\nwant %q", got, found, tc.want)
			}
		})
	}

	if _, found := diff.ResolveConflict(twoConflicts, 3, diff.SideOurs); found {
		t.Fatal("строка 3 не начинает конфликт")
	}
}

func TestConflictLabels(t *testing.T) {
	if ours, theirs, found := diff.ConflictLabels(twoConflicts, 8); !found || ours != "HEAD" || theirs != "feat" {
		t.Fatalf("ConflictLabels = %q, %q, %v", ours, theirs, found)
	}
}

// Конфликт это только закрытый блок с ======= и маркерами одной длины;
// длина маркера берётся из строки начала (#302).
func TestConflictCount(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{"два конфликта", twoConflicts, 2},
		{"незакрытый", "<<<<<<< HEAD\nmain\n=======\nfeat\n", 0},
		{"без =======", "<<<<<<< HEAD\nmain\n>>>>>>> feat\n", 0},
		{"не маркер", "<<<<<<<x\n=======\n>>>>>>> feat\n", 0},
		{"вложенный", "<<<<<<< a\n<<<<<<< b\nx\n=======\ny\n>>>>>>> b\n>>>>>>> a\n", 1},
		{"CRLF", "<<<<<<< HEAD\r\nmain\r\n=======\r\nfeat\r\n>>>>>>> feat\r\n", 1},
		{"conflict-marker-size=10", "<<<<<<<<<< HEAD\nmain\n=======\nnot middle\n==========\nfeat\n>>>>>>>>>> feat\n", 1},
		{"разная длина", "<<<<<<<<<< HEAD\nmain\n=======\nfeat\n>>>>>>> feat\n", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := diff.ConflictCount(tc.text); got != tc.want {
				t.Fatalf("ConflictCount = %d, ожидалось %d", got, tc.want)
			}
		})
	}

	got, found := diff.ResolveConflict("<<<<<<<<<< HEAD\nmain\n=======\nnot middle\n==========\nfeat\n>>>>>>>>>> feat\n", 1, diff.SideOurs)
	if !found || got != "main\n=======\nnot middle\n" {
		t.Fatalf("ResolveConflict длинного маркера = %q, %v", got, found)
	}
}

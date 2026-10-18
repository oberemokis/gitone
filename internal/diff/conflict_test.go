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
	if !diff.HasConflicts(twoConflicts) || diff.HasConflicts("a\n<<<<<<<< not a marker\n") {
		t.Fatal("HasConflicts ошибся")
	}
}

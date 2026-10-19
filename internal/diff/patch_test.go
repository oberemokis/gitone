package diff_test

import (
	"testing"

	"gitone/internal/diff"
)

const twoHunks = `diff --git a/a.txt b/a.txt
index 1111111..2222222 100644
--- a/a.txt
+++ b/a.txt
@@ -1,4 +1,4 @@
-one
+ONE
 two
 three
 four
@@ -8,3 +8,4 @@
 eight
-nine
+NINE
+nine and a half
 ten
`

func TestPatch(t *testing.T) {
	added := func(number int, text string) diff.LineRef {
		return diff.LineRef{Kind: diff.LineAdded, Number: number, Text: text}
	}
	removed := func(number int, text string) diff.LineRef {
		return diff.LineRef{Kind: diff.LineRemoved, Number: number, Text: text}
	}

	cases := []struct {
		name     string
		selected []diff.LineRef
		reverse  bool
		want     string
	}{
		{
			name:     "хунк целиком",
			selected: []diff.LineRef{removed(9, "nine"), added(9, "NINE"), added(10, "nine and a half")},
			want: `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -8,3 +8,4 @@
 eight
-nine
+NINE
+nine and a half
 ten
`,
		},
		{
			// невыбранная удалённая строка становится контекстом, добавленная выпадает
			name:     "одна добавленная строка",
			selected: []diff.LineRef{added(10, "nine and a half")},
			want: `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -8,3 +8,4 @@
 eight
 nine
+nine and a half
 ten
`,
		},
		{
			// второй хунк сдвинут на разницу первого
			name:     "оба хунка",
			selected: []diff.LineRef{removed(1, "one"), added(10, "nine and a half")},
			want: `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -1,4 +1,3 @@
-one
 two
 three
 four
@@ -8,3 +7,4 @@
 eight
 nine
+nine and a half
 ten
`,
		},
		{
			// для отмены в рабочем дереве невыбранная добавленная строка это контекст
			name:     "обратный",
			selected: []diff.LineRef{added(10, "nine and a half")},
			reverse:  true,
			want: `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -8,3 +8,4 @@
 eight
 NINE
+nine and a half
 ten
`,
		},
		{name: "ничего не выбрано", want: ""},
		// на номер встала другая строка, пока выделение висело (#305, случай 1)
		{name: "на номере другая строка", selected: []diff.LineRef{added(10, "something else")}, want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selected := map[diff.LineRef]bool{}
			for _, ref := range tc.selected {
				selected[ref] = true
			}

			if got := diff.Patch(twoHunks, selected, tc.reverse); got != tc.want {
				t.Fatalf("Patch =\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

// Пометка «нет перевода строки» остаётся за оставленной строкой и уходит
// вместе с выпавшей.
func TestPatchNoNewline(t *testing.T) {
	text := `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -1,2 +1,2 @@
 one
-two
\ No newline at end of file
+TWO
\ No newline at end of file
`
	got := diff.Patch(text, map[diff.LineRef]bool{{Kind: diff.LineRemoved, Number: 2, Text: "two"}: true}, false)
	want := `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -1,2 +1,1 @@
 one
-two
\ No newline at end of file
`
	if got != want {
		t.Fatalf("Patch =\n%s\nwant\n%s", got, want)
	}
}

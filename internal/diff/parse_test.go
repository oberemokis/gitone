package diff_test

import (
	"reflect"
	"strings"
	"testing"

	"gitone/internal/diff"
)

// Входы сняты с настоящего git 2.55 с флагами из internal/git: с ними git
// отключает внешний дифф, цвет и пути без префиксов из конфига пользователя.
func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []diff.File
	}{
		{
			name: "изменённый файл с двумя хунками считает добавленные и удалённые строки",
			input: lines(
				"diff --git a/multi.txt b/multi.txt",
				"index 0ff3bbb..9bebd18 100644",
				"--- a/multi.txt",
				"+++ b/multi.txt",
				"@@ -1,5 +1,5 @@",
				" 1",
				"-2",
				"+two",
				" 3",
				" 4",
				" 5",
				"@@ -15,6 +15,6 @@",
				" 15",
				" 16",
				" 17",
				"-18",
				"+eighteen",
				" 19",
				" 20",
			),
			want: []diff.File{{
				OldPath: "multi.txt",
				NewPath: "multi.txt",
				Status:  diff.StatusModified,
				Added:   2,
				Removed: 2,
				Hunks: []diff.Hunk{
					{OldStart: 1, NewStart: 1, Lines: []diff.Line{
						context("1"), removed("2"), added("two"), context("3"), context("4"), context("5"),
					}},
					{OldStart: 15, NewStart: 15, Lines: []diff.Line{
						context("15"), context("16"), context("17"), removed("18"), added("eighteen"), context("19"), context("20"),
					}},
				},
			}},
		},
		{
			name: "строки хунка, похожие на заголовки файла, остаются строками",
			input: lines(
				"diff --git a/query.sql b/query.sql",
				"index 8f04ea4..daea65b 100644",
				"--- a/query.sql",
				"+++ b/query.sql",
				"@@ -1,3 +1,3 @@",
				" select 1;",
				"--- comment",
				" select 2;",
				"+++ x",
			),
			want: []diff.File{{
				OldPath: "query.sql",
				NewPath: "query.sql",
				Status:  diff.StatusModified,
				Added:   1,
				Removed: 1,
				Hunks: []diff.Hunk{{OldStart: 1, NewStart: 1, Lines: []diff.Line{
					context("select 1;"), removed("-- comment"), context("select 2;"), added("++ x"),
				}}},
			}},
		},
		{
			name: "пропущенный счётчик в заголовке хунка значит одну строку",
			input: lines(
				"diff --git a/one-line-new.txt b/one-line-new.txt",
				"new file mode 100644",
				"index 0000000..3e75765",
				"--- /dev/null",
				"+++ b/one-line-new.txt",
				"@@ -0,0 +1 @@",
				"+new",
				"diff --git a/single.txt b/single.txt",
				"index 5626abf..e438487 100644",
				"--- a/single.txt",
				"+++ b/single.txt",
				"@@ -1 +1 @@",
				"-one",
				"+uno",
			),
			want: []diff.File{
				{
					NewPath: "one-line-new.txt",
					Status:  diff.StatusAdded,
					NewMode: "100644",
					Added:   1,
					Hunks:   []diff.Hunk{{OldStart: 0, NewStart: 1, Lines: []diff.Line{added("new")}}},
				},
				{
					OldPath: "single.txt",
					NewPath: "single.txt",
					Status:  diff.StatusModified,
					Added:   1,
					Removed: 1,
					Hunks:   []diff.Hunk{{OldStart: 1, NewStart: 1, Lines: []diff.Line{removed("one"), added("uno")}}},
				},
			},
		},
		{
			name: "удалённый файл и пустые файлы без строк --- и +++ получают путь из заголовка",
			input: lines(
				"diff --git a/gone.txt b/gone.txt",
				"deleted file mode 100644",
				"index de98044..0000000",
				"--- a/gone.txt",
				"+++ /dev/null",
				"@@ -1,3 +0,0 @@",
				"-a",
				"-b",
				"-c",
				"diff --git a/empty-gone.txt b/empty-gone.txt",
				"deleted file mode 100644",
				"index e69de29..0000000",
				"diff --git a/empty-new.txt b/empty-new.txt",
				"new file mode 100644",
				"index 0000000..e69de29",
			),
			want: []diff.File{
				{
					OldPath: "gone.txt",
					Status:  diff.StatusDeleted,
					OldMode: "100644",
					Removed: 3,
					Hunks:   []diff.Hunk{{OldStart: 1, NewStart: 0, Lines: []diff.Line{removed("a"), removed("b"), removed("c")}}},
				},
				{OldPath: "empty-gone.txt", Status: diff.StatusDeleted, OldMode: "100644"},
				{NewPath: "empty-new.txt", Status: diff.StatusAdded, NewMode: "100644"},
			},
		},
		{
			// git diff -B пишет её у переписанного целиком файла
			name: "строка dissimilarity index принимается и ничего не меняет",
			input: lines(
				"diff --git a/rewritten.txt b/rewritten.txt",
				"dissimilarity index 100%",
				"index 9caeabe..66256bc 100644",
				"--- a/rewritten.txt",
				"+++ b/rewritten.txt",
				"@@ -1 +1 @@",
				"-old",
				"+new",
			),
			want: []diff.File{{
				OldPath: "rewritten.txt",
				NewPath: "rewritten.txt",
				Status:  diff.StatusModified,
				Added:   1,
				Removed: 1,
				Hunks:   []diff.Hunk{{OldStart: 1, NewStart: 1, Lines: []diff.Line{removed("old"), added("new")}}},
			}},
		},
		{
			name: "переименование без правок, с правками и копирование берут пути из rename и copy",
			input: lines(
				"diff --git a/source.txt b/copied.txt",
				"similarity index 88%",
				"copy from source.txt",
				"copy to copied.txt",
				"index 9caeabe..66256bc 100644",
				"--- a/source.txt",
				"+++ b/copied.txt",
				"@@ -6,3 +6,4 @@ c5",
				" c6",
				" c7",
				" c8",
				"+c9",
				"diff --git a/edited-move.txt b/edited-renamed.txt",
				"similarity index 87%",
				"rename from edited-move.txt",
				"rename to edited-renamed.txt",
				"index 9027e5f..357e052 100644",
				"--- a/edited-move.txt",
				"+++ b/edited-renamed.txt",
				"@@ -1,7 +1,7 @@",
				" e1",
				" e2",
				" e3",
				"-e4",
				"+E4",
				" e5",
				" e6",
				" e7",
				"diff --git a/moved.txt b/renamed.txt",
				"similarity index 100%",
				"rename from moved.txt",
				"rename to renamed.txt",
			),
			want: []diff.File{
				{
					OldPath: "source.txt",
					NewPath: "copied.txt",
					Status:  diff.StatusCopied,
					Added:   1,
					Hunks: []diff.Hunk{{OldStart: 6, NewStart: 6, Lines: []diff.Line{
						context("c6"), context("c7"), context("c8"), added("c9"),
					}}},
				},
				{
					OldPath: "edited-move.txt",
					NewPath: "edited-renamed.txt",
					Status:  diff.StatusRenamed,
					Added:   1,
					Removed: 1,
					Hunks: []diff.Hunk{{OldStart: 1, NewStart: 1, Lines: []diff.Line{
						context("e1"), context("e2"), context("e3"), removed("e4"), added("E4"), context("e5"), context("e6"), context("e7"),
					}}},
				},
				{OldPath: "moved.txt", NewPath: "renamed.txt", Status: diff.StatusRenamed},
			},
		},
		{
			name: "бинарный файл и смена прав без хунков",
			input: lines(
				"diff --git a/image.png b/image.png",
				"index 45a21f1..bccac03 100644",
				"Binary files a/image.png and b/image.png differ",
				"diff --git a/script.sh b/script.sh",
				"old mode 100644",
				"new mode 100755",
			),
			want: []diff.File{
				{OldPath: "image.png", NewPath: "image.png", Status: diff.StatusModified, Binary: true},
				{OldPath: "script.sh", NewPath: "script.sh", Status: diff.StatusModified, OldMode: "100644", NewMode: "100755"},
			},
		},
		{
			name: "бинарный файл и смена прав с пробелом в пути берут путь из заголовка, поделив его пополам",
			input: lines(
				"diff --git a/Screen Shot.png b/Screen Shot.png",
				"index f584f4041fdb85307f985f76fce8c128a0d12921..23753768bb600005216588c5e1daa8c86bd1143a 100644",
				"Binary files a/Screen Shot.png and b/Screen Shot.png differ",
				"diff --git a/run me.sh b/run me.sh",
				"old mode 100644",
				"new mode 100755",
			),
			want: []diff.File{
				{OldPath: "Screen Shot.png", NewPath: "Screen Shot.png", Status: diff.StatusModified, Binary: true},
				{OldPath: "run me.sh", NewPath: "run me.sh", Status: diff.StatusModified, OldMode: "100644", NewMode: "100755"},
			},
		},
		{
			name: "путь с кавычкой, с пробелом и не ASCII приходит без кавычек и концевого таба",
			input: lines(
				`diff --git "a/quo\"te.txt" "b/quo\"te.txt"`,
				"index 3367afd..3e75765 100644",
				`--- "a/quo\"te.txt"`,
				`+++ "b/quo\"te.txt"`,
				"@@ -1 +1 @@",
				"-old",
				"+new",
				"diff --git a/with space.txt b/with space.txt",
				"index 3367afd..3e75765 100644",
				"--- a/with space.txt\t",
				"+++ b/with space.txt\t",
				"@@ -1 +1 @@",
				"-old",
				"+new",
				"diff --git a/файл.txt b/файл.txt",
				"index 3367afd..3e75765 100644",
				"--- a/файл.txt",
				"+++ b/файл.txt",
				"@@ -1 +1 @@",
				"-old",
				"+new",
				`diff --git "a/quo\"te.sh" "b/quo\"te.sh"`,
				"old mode 100644",
				"new mode 100755",
			),
			want: []diff.File{
				oneLineChange(`quo"te.txt`),
				oneLineChange("with space.txt"),
				oneLineChange("файл.txt"),
				{OldPath: `quo"te.sh`, NewPath: `quo"te.sh`, Status: diff.StatusModified, OldMode: "100644", NewMode: "100755"},
			},
		},
		{
			name: "переименование файла с кавычкой в имени снимает кавычки в rename",
			input: lines(
				`diff --git "a/old\"q.txt" "b/new\"q.txt"`,
				"similarity index 100%",
				`rename from "old\"q.txt"`,
				`rename to "new\"q.txt"`,
			),
			want: []diff.File{{OldPath: `old"q.txt`, NewPath: `new"q.txt`, Status: diff.StatusRenamed}},
		},
		{
			name: "перевод строки CRLF срезается и остаётся признаком CR, управляющие символы становятся видимыми значками",
			input: lines(
				"diff --git a/crlf.txt b/crlf.txt",
				"index cf9b2a8..1119ddb 100644",
				"--- a/crlf.txt",
				"+++ b/crlf.txt",
				"@@ -1,2 +1,3 @@",
				" line one\r",
				"-line two\r",
				"+line 2\r",
				"+\x1b[31mred\x1b[0m\tend\x7f",
			),
			want: []diff.File{{
				OldPath: "crlf.txt",
				NewPath: "crlf.txt",
				Status:  diff.StatusModified,
				Added:   2,
				Removed: 1,
				Hunks: []diff.Hunk{{OldStart: 1, NewStart: 1, Lines: []diff.Line{
					{Kind: diff.LineContext, Text: "line one", CR: true},
					{Kind: diff.LineRemoved, Text: "line two", CR: true},
					{Kind: diff.LineAdded, Text: "line 2", CR: true},
					added("␛[31mred␛[0m\tend␡"),
				}}},
			}},
		},
		{
			name: "символ C1 и битый байт в строке становятся U+FFFD",
			input: lines(
				"diff --git a/c1.txt b/c1.txt",
				"index 3367afd..3e75765 100644",
				"--- a/c1.txt",
				"+++ b/c1.txt",
				"@@ -0,0 +1 @@",
				"+a\u009bb\xffc",
			),
			want: []diff.File{{
				OldPath: "c1.txt",
				NewPath: "c1.txt",
				Status:  diff.StatusModified,
				Added:   1,
				Hunks:   []diff.Hunk{{OldStart: 0, NewStart: 1, Lines: []diff.Line{added("a�b�c")}}},
			}},
		},
		{
			name: "строка без перевода в конце файла получает отметку с обеих сторон",
			input: lines(
				"diff --git a/nonl.txt b/nonl.txt",
				"index bc43a4f..2622b85 100644",
				"--- a/nonl.txt",
				"+++ b/nonl.txt",
				"@@ -1,2 +1,2 @@",
				" keep",
				"-tail",
				`\ No newline at end of file`,
				"+TAIL",
				`\ No newline at end of file`,
			),
			want: []diff.File{{
				OldPath: "nonl.txt",
				NewPath: "nonl.txt",
				Status:  diff.StatusModified,
				Added:   1,
				Removed: 1,
				Hunks: []diff.Hunk{{OldStart: 1, NewStart: 1, Lines: []diff.Line{
					context("keep"),
					{Kind: diff.LineRemoved, Text: "tail", NoNewline: true},
					{Kind: diff.LineAdded, Text: "TAIL", NoNewline: true},
				}}},
			}},
		},
		{
			name: "пустая строка контекста без пробела остаётся контекстом",
			input: lines(
				"diff --git a/blank.txt b/blank.txt",
				"index a1a53b5..9c1e21d 100644",
				"--- a/blank.txt",
				"+++ b/blank.txt",
				"@@ -1,3 +1,3 @@",
				" a",
				"",
				"-b",
				"+B",
			),
			want: []diff.File{{
				OldPath: "blank.txt",
				NewPath: "blank.txt",
				Status:  diff.StatusModified,
				Added:   1,
				Removed: 1,
				Hunks: []diff.Hunk{{OldStart: 1, NewStart: 1, Lines: []diff.Line{
					context("a"), context(""), removed("b"), added("B"),
				}}},
			}},
		},
		{
			name:  "строка в 3000 символов остаётся целиком",
			input: minifiedInput(strings.Repeat("я", 3000)),
			want:  []diff.File{minifiedFile(strings.Repeat("я", 3000))},
		},
		{
			name:  "строка ровно в 4096 символов остаётся целиком",
			input: minifiedInput(strings.Repeat("я", 4096)),
			want:  []diff.File{minifiedFile(strings.Repeat("я", 4096))},
		},
		{
			name:  "строка длиннее 4096 символов обрезается, последний символ заменён на «…»",
			input: minifiedInput(strings.Repeat("я", 5000)),
			want:  []diff.File{minifiedFile(strings.Repeat("я", 4095) + "…")},
		},
		{
			name: "управляющие символы в сырой записи тоже становятся значками",
			input: lines(
				"diff --cc conflict.txt",
				"++\x1b[31mred\x1b[0m",
			),
			want: []diff.File{{
				NewPath: "conflict.txt",
				Status:  diff.StatusRaw,
				Raw:     "diff --cc conflict.txt\n++␛[31mred␛[0m",
			}},
		},
		{
			name:  "пустой вывод git значит, что изменений нет",
			input: "",
			want:  nil,
		},
		{
			name: "вывод --stat без единого файла становится сырой записью",
			input: lines(
				" multi.txt | 1 +",
				" 1 file changed, 1 insertion(+)",
			),
			want: []diff.File{{
				Status: diff.StatusRaw,
				Raw:    " multi.txt | 1 +\n 1 file changed, 1 insertion(+)",
			}},
		},
		{
			name: "diff --cc и строка неслитого пути становятся сырыми записями, соседний файл разбирается",
			input: lines(
				"diff --cc conflict.txt",
				"index ba2906d,2299c37..0000000",
				"--- a/conflict.txt",
				"+++ b/conflict.txt",
				"@@@ -1,1 -1,1 +1,5 @@@",
				"++<<<<<<< HEAD",
				" +main",
				"++=======",
				"+ side",
				"++>>>>>>> side",
				"diff --git a/single.txt b/single.txt",
				"index 5626abf..e438487 100644",
				"--- a/single.txt",
				"+++ b/single.txt",
				"@@ -1 +1 @@",
				"-old",
				"+new",
				"* Unmerged path other.txt",
			),
			want: []diff.File{
				{
					NewPath: "conflict.txt",
					Status:  diff.StatusRaw,
					Raw: strings.Join([]string{
						"diff --cc conflict.txt",
						"index ba2906d,2299c37..0000000",
						"--- a/conflict.txt",
						"+++ b/conflict.txt",
						"@@@ -1,1 -1,1 +1,5 @@@",
						"++<<<<<<< HEAD",
						" +main",
						"++=======",
						"+ side",
						"++>>>>>>> side",
					}, "\n"),
				},
				oneLineChange("single.txt"),
				{NewPath: "other.txt", Status: diff.StatusRaw, Raw: "* Unmerged path other.txt"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := withoutBlobs(diff.Parse(tc.input))

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%q) =\n%+v\nwant\n%+v", tc.input, got, tc.want)
			}
		})
	}
}

// У строки «diff --git » нет путей, делить пополам нечего: разбор оставляет
// файл с пустыми путями, а не падает на срезе.
func TestParseHeaderWithoutPaths(t *testing.T) {
	got := diff.Parse("diff --git \n")

	want := []diff.File{{Status: diff.StatusModified}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse(%q) = %+v, want %+v", "diff --git \n", got, want)
	}
}

// Без строк --- и +++ пути разные у --no-index: заголовок делится по « b/»,
// у бинарного файла пути берутся из «Binary files»; имя в кавычках с байтом
// не UTF-8 и кавычкой совпадает с именем на диске (#303).
func TestParsePathsWithoutFileLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		old, new string
	}{
		{name: "бинарные разной длины", input: lines(
			"diff --git a/longer.png b/x.png",
			"index 1111111..2222222 100644",
			"Binary files a/longer.png and b/x.png differ",
		), old: "longer.png", new: "x.png"},
		{name: "смена прав у разных имён", input: lines(
			"diff --git a/one.sh b/two.sh",
			"old mode 100644",
			"new mode 100755",
		), old: "one.sh", new: "two.sh"},
		{name: "новый бинарный", input: lines(
			"diff --git a/a b/a b/a b/a",
			"new file mode 100644",
			"index 0000000..2222222",
			"Binary files /dev/null and b/a b/a differ",
		), old: "", new: "a b/a"},
		{name: "кавычки и байт не UTF-8", input: lines(
			"diff --git \"a/caf\xe9\\\"q.txt\" \"b/caf\xe9\\\"q.txt\"",
			"index 1111111..2222222 100644",
			"--- \"a/caf\xe9\\\"q.txt\"",
			"+++ \"b/caf\xe9\\\"q.txt\"",
			"@@ -1 +1 @@",
			"-old",
			"+new",
		), old: "caf\xe9\"q.txt", new: "caf\xe9\"q.txt"},
		{name: "восьмеричный экран", input: lines(
			"diff --git \"a/caf\\303\\251\\tx\" \"b/caf\\303\\251\\tx\"",
			"old mode 100644",
			"new mode 100755",
		), old: "café\tx", new: "café\tx"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files := diff.Parse(tc.input)
			if len(files) != 1 {
				t.Fatalf("файлов %d, ожидался один", len(files))
			}
			if files[0].OldPath != tc.old || files[0].NewPath != tc.new {
				t.Fatalf("пути %q и %q, want %q и %q", files[0].OldPath, files[0].NewPath, tc.old, tc.new)
			}
		})
	}
}

// withoutBlobs убирает хэши версий: здесь проверяется устройство файлов, а
// хэши проверяет TestParseBlobs.
func withoutBlobs(files []diff.File) []diff.File {
	for index := range files {
		files[index].OldBlob, files[index].NewBlob = "", ""
	}
	return files
}

// Хэши обеих версий берутся из строки index. Хэш из нулей значит, что этой
// версии нет: у нового файла нет старой, у удалённого новой (#140). У
// конфликта слияния версий три, такой файл в контексте не красится.
func TestParseBlobs(t *testing.T) {
	header := func(index string) string {
		return lines("diff --git a/a.vue b/a.vue", index, "--- a/a.vue", "+++ b/a.vue", "@@ -1 +1 @@", "-old", "+new")
	}

	tests := []struct {
		name     string
		input    string
		old, new string
	}{
		{name: "изменённый файл", input: header("index 0ff3bbb4c..9bebd18aa 100644"), old: "0ff3bbb4c", new: "9bebd18aa"},
		{name: "новый файл", input: header("index 0000000000..3e75765abc"), old: "", new: "3e75765abc"},
		{name: "удалённый файл", input: header("index de98044abc..0000000000"), old: "de98044abc", new: ""},
		{name: "конфликт слияния идёт сырой записью без хэшей", input: lines(
			"diff --cc a.vue",
			"index ba2906d0666cf726c7eaadd2cd3db615dedfdf3a,2299c37978265a95cbe835a4b0f0bbf15aad5549..0000000000000000000000000000000000000000",
			"--- a/a.vue",
			"+++ b/a.vue",
			"@@@ -1,1 -1,1 +1,5 @@@",
			"++<<<<<<< HEAD",
			" +main",
			"++=======",
			"+ side",
			"++>>>>>>> side",
		), old: "", new: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files := diff.Parse(tc.input)
			if len(files) != 1 {
				t.Fatalf("файлов %d, ожидался один", len(files))
			}
			if files[0].OldBlob != tc.old || files[0].NewBlob != tc.new {
				t.Fatalf("хэши %q и %q, want %q и %q", files[0].OldBlob, files[0].NewBlob, tc.old, tc.new)
			}
		})
	}
}

// minifiedInput это дифф нового файла из одной строки text.
func minifiedInput(text string) string {
	return lines(
		"diff --git a/min.js b/min.js",
		"index 1111111..2222222 100644",
		"--- a/min.js",
		"+++ b/min.js",
		"@@ -0,0 +1 @@",
		"+"+text,
	)
}

func minifiedFile(text string) diff.File {
	return diff.File{
		OldPath: "min.js",
		NewPath: "min.js",
		Status:  diff.StatusModified,
		Added:   1,
		Hunks:   []diff.Hunk{{OldStart: 0, NewStart: 1, Lines: []diff.Line{added(text)}}},
	}
}

func lines(all ...string) string {
	return strings.Join(all, "\n") + "\n"
}

func context(text string) diff.Line {
	return diff.Line{Kind: diff.LineContext, Text: text}
}

func added(text string) diff.Line {
	return diff.Line{Kind: diff.LineAdded, Text: text}
}

func removed(text string) diff.Line {
	return diff.Line{Kind: diff.LineRemoved, Text: text}
}

func oneLineChange(path string) diff.File {
	return diff.File{
		OldPath: path,
		NewPath: path,
		Status:  diff.StatusModified,
		Added:   1,
		Removed: 1,
		Hunks:   []diff.Hunk{{OldStart: 1, NewStart: 1, Lines: []diff.Line{removed("old"), added("new")}}},
	}
}

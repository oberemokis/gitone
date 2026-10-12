package syntax_test

import (
	"strings"
	"testing"

	"gitone/internal/syntax"
)

// kindOf ищет вид отрезка, который начинается там же, где первое вхождение
// text в строку line. Пусто, если такого отрезка нет.
func kindOf(lines []string, spans [][]syntax.Span, line int, text string) syntax.Kind {
	at := strings.Index(lines[line], text)
	if at < 0 || line >= len(spans) {
		return ""
	}
	for _, span := range spans[line] {
		if span.Start <= at && at < span.End {
			return span.Kind
		}
	}
	return ""
}

func TestKinds(t *testing.T) {
	goLines := []string{
		"// Load читает дифф",
		"func Load(dir string) (int, error) {",
		"\tx := fmt.Sprintf(\"%d\", 42)",
		"\treturn readDiff(x), nil",
		"}",
	}
	// обрывок из середины функции, как в хунке
	tsLines := []string{
		"  const response = await fetch(url);",
		"  if (!response.ok || !response.body) {",
		"    throw new Error(`Failed: ${response.status}`);",
		"  }",
	}

	tests := []struct {
		name  string
		path  string
		lines []string
		line  int
		text  string
		want  syntax.Kind
	}{
		{name: "go: комментарий", path: "a.go", lines: goLines, line: 0, text: "// Load", want: syntax.Comment},
		{name: "go: ключевое слово", path: "a.go", lines: goLines, line: 1, text: "func", want: syntax.Keyword},
		{name: "go: имя функции в объявлении", path: "a.go", lines: goLines, line: 1, text: "Load", want: syntax.Function},
		{name: "go: встроенный тип", path: "a.go", lines: goLines, line: 1, text: "string", want: syntax.Type},
		{name: "go: строка", path: "a.go", lines: goLines, line: 2, text: `"%d"`, want: syntax.String},
		{name: "go: число", path: "a.go", lines: goLines, line: 2, text: "42", want: syntax.Number},
		{name: "go: оператор", path: "a.go", lines: goLines, line: 2, text: ":=", want: syntax.Operator},
		{name: "go: вызов функции", path: "a.go", lines: goLines, line: 3, text: "readDiff", want: syntax.Function},
		{name: "go: nil", path: "a.go", lines: goLines, line: 3, text: "nil", want: syntax.Constant},
		{name: "ts: ключевое слово", path: "a.ts", lines: tsLines, line: 0, text: "const", want: syntax.Keyword},
		{name: "ts: await", path: "a.ts", lines: tsLines, line: 0, text: "await", want: syntax.Keyword},
		{name: "ts: свойство", path: "a.ts", lines: tsLines, line: 1, text: "ok", want: syntax.Property},
		{name: "ts: оператор", path: "a.ts", lines: tsLines, line: 1, text: "||", want: syntax.Operator},
		{name: "ts: шаблонная строка", path: "a.ts", lines: tsLines, line: 2, text: "`Failed", want: syntax.String},
		// код внутри ${…} красится как код, а не как строка
		{name: "ts: свойство внутри шаблона", path: "a.ts", lines: tsLines, line: 2, text: "status", want: syntax.Property},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spans := syntax.Detect(tc.path).Lines(tc.lines)
			if got := kindOf(tc.lines, spans, tc.line, tc.text); got != tc.want {
				t.Errorf("%q в строке %d: вид %q, want %q (%+v)", tc.text, tc.line, got, tc.want, spans)
			}
		})
	}
}

// Многострочный комментарий окрашен во всех своих строках: разбирается вся
// сторона хунка, а не строка.
func TestMultilineComment(t *testing.T) {
	lines := []string{"x := 1", "/* первая", "   вторая */", "y := 2"}
	spans := syntax.Detect("a.go").Lines(lines)

	for _, line := range []int{1, 2} {
		if got := kindOf(lines, spans, line, strings.TrimSpace(lines[line])); got != syntax.Comment {
			t.Errorf("строка %d %q: вид %q, want comment", line, lines[line], got)
		}
	}
}

// Файл без грамматики: подсветки нет, ошибки нет.
func TestUnknownLanguage(t *testing.T) {
	if spans := syntax.Detect("notes.unknownext").Lines([]string{"func x"}); spans != nil {
		t.Errorf("для неизвестного расширения есть отрезки: %+v", spans)
	}
}

// Отрезки строки не пересекаются, идут по возрастанию и лежат в её границах:
// по ним дальше режут текст.
func TestSpansStayInsideLines(t *testing.T) {
	lines := []string{`x := "a" // b`, "\tif x != nil { return }", ""}
	spans := syntax.Detect("a.go").Lines(lines)

	if len(spans) != len(lines) {
		t.Fatalf("строк отрезков %d, want %d", len(spans), len(lines))
	}
	for index, line := range spans {
		end := 0
		for _, span := range line {
			if span.Start < end || span.End > len(lines[index]) || span.Start >= span.End {
				t.Fatalf("строка %d: отрезок %+v вне порядка или границ", index, span)
			}
			end = span.End
		}
	}
}

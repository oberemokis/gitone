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
	vueLines := []string{
		"<template>",
		`  <div class="card" v-if="ok">Hello</div>`,
		"  <!-- note -->",
		"</template>",
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
		// (identifier) @variable стоит в запросе первым и без цвета не должно
		// занимать имя вызова (#140)
		{name: "ts: вызов функции", path: "a.ts", lines: tsLines, line: 0, text: "fetch", want: syntax.Function},
		// запрос TS ставит (identifier) @type с ^[A-Z] ниже такого же шаблона
		// @constructor, и побеждает поздний: имя класса в TS это тип, а не
		// функция (#160)
		{name: "ts: конструктор", path: "a.ts", lines: tsLines, line: 2, text: "Error", want: syntax.Type},
		{name: "ts: свойство", path: "a.ts", lines: tsLines, line: 1, text: "ok", want: syntax.Property},
		{name: "ts: оператор", path: "a.ts", lines: tsLines, line: 1, text: "||", want: syntax.Operator},
		{name: "ts: шаблонная строка", path: "a.ts", lines: tsLines, line: 2, text: "`Failed", want: syntax.String},
		// код внутри ${…} красится как код, а не как строка
		{name: "ts: свойство внутри шаблона", path: "a.ts", lines: tsLines, line: 2, text: "status", want: syntax.Property},
		// #139: грамматика vue в сборке и свой запрос разметки
		{name: "vue: тег", path: "App.vue", lines: vueLines, line: 1, text: "div", want: syntax.Tag},
		{name: "vue: атрибут", path: "App.vue", lines: vueLines, line: 1, text: "class", want: syntax.Property},
		{name: "vue: значение атрибута", path: "App.vue", lines: vueLines, line: 1, text: "card", want: syntax.String},
		{name: "vue: директива", path: "App.vue", lines: vueLines, line: 1, text: "v-if", want: syntax.Keyword},
		{name: "vue: текст шаблона", path: "App.vue", lines: vueLines, line: 1, text: "Hello", want: ""},
		{name: "vue: комментарий", path: "App.vue", lines: vueLines, line: 2, text: "<!--", want: syntax.Comment},
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

// kindCase это ожидание вида одного слова в исходнике: слово ищется в строке
// line по тексту text.
type kindCase struct {
	name  string
	path  string
	lines []string
	line  int
	text  string
	want  syntax.Kind
}

func checkKinds(t *testing.T, tests []kindCase) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spans := syntax.Detect(tc.path).Lines(tc.lines)
			if got := kindOf(tc.lines, spans, tc.line, tc.text); got != tc.want {
				t.Errorf("%q в строке %d: вид %q, want %q (%+v)", tc.text, tc.line, got, tc.want, spans)
			}
		})
	}
}

// Узлу достаётся шаблон, в котором он вложен (поле или родитель в шаблоне,
// предикат), а не голый (тип) @вид, и среди равных поздний: так написаны
// запросы upstream (#160). Голый (property_identifier) @property стоит в
// запросе TS выше шаблона вызова метода, но вызову уступать не должен.
func TestPatternPriority(t *testing.T) {
	tsLines := []string{
		"const out = items.map((x) => x.label);",
		"const data = await response.json();",
	}
	jsLines := []string{"const MAX_SIZE = 10;"}
	cppLines := []string{
		"class A {",
		"public:",
		"  void run();",
		"};",
		"void use(A a) {",
		"  a.run();",
		"}",
	}
	nixLines := []string{
		"{ pkgs, lib }:",
		"let x = pkgs.hello; in",
		"x",
	}
	goLines := []string{
		"func (m Model) View() string {",
		"\treturn fmt.Sprintf(\"%d\", m.count)",
		"}",
	}
	pyLines := []string{
		"os.getcwd()",
		"print(os.sep)",
	}

	checkKinds(t, []kindCase{
		{name: "ts: метод в вызове", path: "a.ts", lines: tsLines, line: 0, text: "map", want: syntax.Function},
		{name: "ts: метод после await", path: "a.ts", lines: tsLines, line: 1, text: "json", want: syntax.Function},
		{name: "ts: свойство без вызова", path: "a.ts", lines: tsLines, line: 0, text: "label", want: syntax.Property},
		{name: "js: константа в верхнем регистре", path: "a.js", lines: jsLines, line: 0, text: "MAX_SIZE", want: syntax.Constant},
		{name: "cpp: метод в вызове", path: "a.cpp", lines: cppLines, line: 5, text: "run", want: syntax.Function},
		{name: "cpp: метод в классе", path: "a.cpp", lines: cppLines, line: 2, text: "run", want: syntax.Function},
		// имена формальных параметров и переменные без цвета, имена полей свойства
		{name: "nix: параметр", path: "a.nix", lines: nixLines, line: 0, text: "lib", want: ""},
		{name: "nix: переменная", path: "a.nix", lines: nixLines, line: 2, text: "x", want: ""},
		{name: "nix: поле в select", path: "a.nix", lines: nixLines, line: 1, text: "hello", want: syntax.Property},
		// голый (field_identifier) @property в Go стоит ниже вызова метода
		{name: "go: метод в вызове", path: "a.go", lines: goLines, line: 1, text: "Sprintf", want: syntax.Function},
		{name: "go: метод в объявлении", path: "a.go", lines: goLines, line: 0, text: "View", want: syntax.Function},
		{name: "go: поле", path: "a.go", lines: goLines, line: 1, text: "count", want: syntax.Property},
		// в Python (attribute attribute: …) @property стоит ниже вызова метода,
		// но у вызова шаблон длиннее
		{name: "python: метод в вызове", path: "a.py", lines: pyLines, line: 0, text: "getcwd", want: syntax.Function},
		{name: "python: атрибут", path: "a.py", lines: pyLines, line: 1, text: "sep", want: syntax.Property},
	})
}

// gotreesitter не проверяет #is-not? local: шаблон ((identifier)
// @function.method (#is-not? local)) ловил бы каждый идентификатор Ruby.
// Таблицы областей видимости у нас нет, поэтому такие шаблоны пропускаются:
// ложная функция хуже ненайденной (#160). Имена с собственным шаблоном
// (объявление метода, вызов через точку) красятся как раньше.
func TestIsNotLocalPatternsSkipped(t *testing.T) {
	ruby := []string{
		"def total(list)",
		"  sum = 0",
		"  list.each { |row| sum += row.price }",
		"end",
	}
	js := []string{"console.log(1);", "const load = require;"}

	checkKinds(t, []kindCase{
		{name: "ruby: объявление метода", path: "a.rb", lines: ruby, line: 0, text: "total", want: syntax.Function},
		{name: "ruby: параметр метода", path: "a.rb", lines: ruby, line: 0, text: "list", want: ""},
		{name: "ruby: локальная переменная", path: "a.rb", lines: ruby, line: 1, text: "sum", want: ""},
		{name: "ruby: получатель вызова", path: "a.rb", lines: ruby, line: 2, text: "list", want: ""},
		{name: "ruby: параметр блока", path: "a.rb", lines: ruby, line: 2, text: "row", want: ""},
		{name: "ruby: вызов с блоком", path: "a.rb", lines: ruby, line: 2, text: "each", want: syntax.Function},
		{name: "ruby: вызов через точку", path: "a.rb", lines: ruby, line: 2, text: "price", want: syntax.Function},
		// встроенные имена JS стоят под #is-not? local: подсветить console и
		// require, не отличив их от локальных, нельзя
		{name: "js: console без таблицы областей", path: "a.js", lines: js, line: 0, text: "console", want: ""},
		{name: "js: require без таблицы областей", path: "a.js", lines: js, line: 1, text: "require", want: ""},
		{name: "js: метод через точку", path: "a.js", lines: js, line: 0, text: "log", want: syntax.Function},
	})
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

// Сборка с явным списком грамматик (#139) держит прежний набор: языки из
// core100 определяются и разбираются, как раньше.
func TestGrammarSetKeepsCoreLanguages(t *testing.T) {
	for _, path := range []string{"a.py", "a.rs", "a.svelte", "a.astro", "a.yaml", "a.css", "Dockerfile", "a.sql"} {
		if spans := syntax.Detect(path).Lines([]string{"x"}); spans == nil {
			t.Errorf("%s: язык не определился", path)
		}
	}
}

// Код однофайловых компонентов разбирает своя грамматика: скрипт по атрибуту
// lang, стиль как CSS или SCSS, выражения шаблона как TypeScript (#140).
// Разметка вокруг остаётся разметкой.
func TestEmbeddedCode(t *testing.T) {
	vue := []string{
		"<template>",
		`  <p :title="label(item)">{{ format(title) }}</p>`,
		"</template>",
		`<script setup lang="ts">`,
		"const count: number = ref(0)",
		"</script>",
		`<style scoped lang="scss">`,
		"$gap: 4px;",
		".card { color: red; }",
		"</style>",
	}
	svelte := []string{
		`<script lang="ts">`,
		"  let count: number = 0",
		"</script>",
		"<button on:click={increment}>{format(count)}</button>",
		"<style>",
		".card { color: red; }",
		"</style>",
	}
	astro := []string{
		"---",
		"const title: string = await load()",
		"---",
		"<h1 class={pick(title)}>{title}</h1>",
	}

	tests := []struct {
		name  string
		path  string
		lines []string
		line  int
		text  string
		want  syntax.Kind
	}{
		{name: "vue: тег вокруг кода", path: "App.vue", lines: vue, line: 3, text: "script", want: syntax.Tag},
		{name: "vue: const в script", path: "App.vue", lines: vue, line: 4, text: "const", want: syntax.Keyword},
		{name: "vue: тип в script lang=ts", path: "App.vue", lines: vue, line: 4, text: "number", want: syntax.Type},
		{name: "vue: вызов в script", path: "App.vue", lines: vue, line: 4, text: "ref", want: syntax.Function},
		{name: "vue: вызов в {{ }}", path: "App.vue", lines: vue, line: 1, text: "format", want: syntax.Function},
		{name: "vue: вызов в директиве", path: "App.vue", lines: vue, line: 1, text: "label", want: syntax.Function},
		// у scss цвета есть у значений, у переменных и имён свойств нет
		{name: "vue: значение в style lang=scss", path: "App.vue", lines: vue, line: 8, text: "red", want: syntax.String},
		{name: "svelte: let в script", path: "App.svelte", lines: svelte, line: 1, text: "let", want: syntax.Keyword},
		{name: "svelte: вызов в {…}", path: "App.svelte", lines: svelte, line: 3, text: "format", want: syntax.Function},
		{name: "svelte: свойство в style", path: "App.svelte", lines: svelte, line: 5, text: "color", want: syntax.Property},
		{name: "astro: const во frontmatter", path: "Page.astro", lines: astro, line: 1, text: "const", want: syntax.Keyword},
		{name: "astro: вызов в атрибуте", path: "Page.astro", lines: astro, line: 3, text: "pick", want: syntax.Function},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spans := syntax.Detect(tc.path).Lines(tc.lines)
			if got := kindOf(tc.lines, spans, tc.line, tc.text); got != tc.want {
				t.Errorf("%q в строке %d: вид %q, want %q (%+v)", tc.text, tc.line, got, tc.want, spans[tc.line])
			}
		})
	}
}

// Однофайловым компонентам нужен файл целиком: по хунку не понять, из какого
// он блока. Остальным языкам хватает хунка.
func TestEmbedsNeedWholeFile(t *testing.T) {
	for path, want := range map[string]bool{"App.vue": true, "App.svelte": true, "Page.astro": true, "a.ts": false, "a.go": false, "a.unknownext": false} {
		if got := syntax.Detect(path).Embeds(); got != want {
			t.Errorf("%s: Embeds %v, want %v", path, got, want)
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

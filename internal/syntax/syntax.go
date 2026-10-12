// Package syntax размечает строки диффа по видам токенов: ключевые слова,
// строки, комментарии, числа, функции, типы, константы, свойства, операторы и
// теги. Разбор идёт tree-sitter на чистом Go (gotreesitter), язык выбирается
// по имени файла: по содержимому угадывание даёт ложные цвета, а подсветка без
// уверенности хуже её отсутствия. Пакет не знает о цветах: тему и ширину
// задаёт tui.
//
// Набор грамматик задают теги сборки grammar_set_core и
// gotreesitter_no_copyleft (Makefile, .goreleaser.yaml): сто распространённых
// языков без грамматик под копилефтом. Решения в задаче #101.
package syntax

import (
	"path/filepath"
	"slices"
	"strings"
	"sync"

	ts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

type Kind string

const (
	Keyword  Kind = "keyword"
	String   Kind = "string"
	Comment  Kind = "comment"
	Number   Kind = "number"
	Function Kind = "function"
	Type     Kind = "type"
	Constant Kind = "constant"
	Property Kind = "property"
	Operator Kind = "operator"
	Tag      Kind = "tag"
)

// Span это кусок строки в байтах с видом. Куски одной строки не пересекаются
// и идут по возрастанию.
type Span struct {
	Start int
	End   int
	Kind  Kind
}

// Lexer это язык файла. Нулевое значение значит, что язык неизвестен и
// подсветки нет.
type Lexer struct {
	entry *grammars.LangEntry
}

func Detect(path string) Lexer {
	entry := grammars.DetectLanguage(filepath.Base(path))
	if entry == nil || entry.HighlightQuery == "" {
		return Lexer{}
	}

	return Lexer{entry: entry}
}

// Lines размечает подряд идущие строки одной стороны хунка и возвращает
// отрезки каждой строки. Строки разбираются вместе: tree-sitter нужен
// контекст, по одной строке он путает типы и параметры, а многострочный
// комментарий терял бы продолжение. Без языка или при сбое разбора nil.
func (l Lexer) Lines(lines []string) [][]Span {
	if l.entry == nil || len(lines) == 0 {
		return nil
	}

	source := []byte(strings.Join(lines, "\n") + "\n")
	kinds := l.paint(source)
	if kinds == nil {
		return nil
	}

	result := make([][]Span, len(lines))
	start := 0
	for index, line := range lines {
		result[index] = runs(kinds[start : start+len(line)])
		start += len(line) + 1
	}
	return result
}

// paint красит каждый байт исходника видом. Для каждого узла берётся первый
// подошедший шаблон запроса, как в tree-sitter: запросы upstream написаны под
// этот порядок, общее (identifier) @variable стоит в конце и не должно
// затирать функции. Highlighter из gotreesitter отдаёт победу позднему
// шаблону, поэтому запрос выполняется здесь. Вложенный узел перекрывает
// внешний: код внутри ${…} шаблонной строки красится как код.
func (l Lexer) paint(source []byte) []Kind {
	language := l.entry.Language()
	query, err := compiled(l.entry.Name, l.entry.HighlightQuery, language)
	if err != nil {
		return nil
	}

	tree, err := parse(l.entry, language, source)
	if err != nil || tree == nil {
		return nil
	}

	type capture struct {
		start, end uint32
		kind       Kind
		pattern    int
	}
	best := map[[2]uint32]capture{}
	for _, match := range query.Execute(tree) {
		for _, found := range match.Captures {
			key := [2]uint32{found.Node.StartByte(), found.Node.EndByte()}
			if old, seen := best[key]; seen && old.pattern <= match.PatternIndex {
				continue
			}
			best[key] = capture{start: key[0], end: key[1], kind: kindOf(found.Name), pattern: match.PatternIndex}
		}
	}

	captures := make([]capture, 0, len(best))
	for _, found := range best {
		captures = append(captures, found)
	}
	// длинные красятся первыми, вложенные короткие поверх них
	slices.SortFunc(captures, func(a, b capture) int {
		return int(b.end-b.start) - int(a.end-a.start)
	})

	kinds := make([]Kind, len(source))
	for _, found := range captures {
		for at := found.start; at < found.end && int(at) < len(kinds); at++ {
			kinds[at] = found.kind
		}
	}
	return kinds
}

func parse(entry *grammars.LangEntry, language *ts.Language, source []byte) (*ts.Tree, error) {
	parser := ts.NewParser(language)
	if entry.TokenSourceFactory != nil {
		return parser.ParseWithTokenSource(source, entry.TokenSourceFactory(source, language))
	}
	return parser.Parse(source)
}

// queries хранит собранные запросы по имени языка: сборка запроса стоит
// дороже разбора короткого хунка.
var queries sync.Map

type compiledQuery struct {
	query *ts.Query
	err   error
}

func compiled(name, source string, language *ts.Language) (*ts.Query, error) {
	if cached, found := queries.Load(name); found {
		entry := cached.(compiledQuery)
		return entry.query, entry.err
	}

	query, err := ts.NewQuery(source, language)
	queries.Store(name, compiledQuery{query: query, err: err})
	return query, err
}

// runs сворачивает виды байтов строки в отрезки. Байты без вида (имена
// переменных, пунктуация) отрезков не дают: они идут цветом текста.
func runs(kinds []Kind) []Span {
	var spans []Span
	for start := 0; start < len(kinds); {
		end := start + 1
		for end < len(kinds) && kinds[end] == kinds[start] {
			end++
		}
		if kinds[start] != "" {
			spans = append(spans, Span{Start: start, End: end, Kind: kinds[start]})
		}
		start = end
	}
	return spans
}

// kindOf переводит имя захвата tree-sitter (keyword.return, function.method,
// string.special) в вид. Захваты без вида, как variable и punctuation,
// остаются цветом текста.
func kindOf(capture string) Kind {
	if capture == "variable.builtin" {
		return Constant
	}

	group, _, _ := strings.Cut(capture, ".")
	switch group {
	case "keyword", "conditional", "repeat", "include", "exception", "storageclass":
		return Keyword
	case "string", "character", "escape":
		return String
	case "comment":
		return Comment
	case "number", "float":
		return Number
	case "function", "method", "constructor":
		return Function
	case "type":
		return Type
	case "constant", "boolean":
		return Constant
	case "property", "field", "attribute":
		return Property
	case "operator":
		return Operator
	case "tag":
		return Tag
	}
	return ""
}

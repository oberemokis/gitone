package syntax_test

import (
	"slices"
	"testing"

	"gitone/internal/syntax"
)

// textsOf возвращает куски нужной категории, чтобы тест читался по тексту.
func textsOf(text string, spans []syntax.Span, kind syntax.Kind) []string {
	var texts []string
	for _, span := range spans {
		if span.Kind == kind {
			texts = append(texts, text[span.Start:span.End])
		}
	}
	return texts
}

func TestDetectByFileName(t *testing.T) {
	if syntax.Detect("internal/app.go").Tokens("func") == nil {
		t.Error("для .go язык не найден")
	}
	if syntax.Detect("notes.unknownext").Tokens("func") != nil {
		t.Error("для неизвестного расширения есть куски")
	}
}

func TestGoKinds(t *testing.T) {
	const text = `func main() { return 1 } "hi" // done`
	spans := syntax.Detect("main.go").Tokens(text)

	if got := textsOf(text, spans, syntax.Keyword); !slices.Equal(got, []string{"func", "return"}) {
		t.Errorf("ключевые слова %q", got)
	}
	if got := textsOf(text, spans, syntax.Number); !slices.Equal(got, []string{"1"}) {
		t.Errorf("числа %q", got)
	}
	if got := textsOf(text, spans, syntax.Comment); !slices.Equal(got, []string{"// done"}) {
		t.Errorf("комментарии %q", got)
	}
	if got := textsOf(text, spans, syntax.String); !slices.Equal(got, []string{`"hi"`}) {
		t.Errorf("строки %q", got)
	}
}

// Куски не перекрываются и лежат в границах строки: по ним дальше режут текст.
func TestSpansStayInsideText(t *testing.T) {
	const text = `x := "a" // b`
	end := 0
	for _, span := range syntax.Detect("a.go").Tokens(text) {
		if span.Start < end || span.End > len(text) || span.Start >= span.End {
			t.Fatalf("кусок %+v вне порядка или границ", span)
		}
		end = span.End
	}
}

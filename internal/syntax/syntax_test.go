package syntax

import (
	"slices"
	"testing"
)

// textsOf возвращает куски нужной категории, чтобы тест читался по тексту.
func textsOf(text string, spans []Span, kind Kind) []string {
	var texts []string
	for _, span := range spans {
		if span.Kind == kind {
			texts = append(texts, text[span.Start:span.End])
		}
	}
	return texts
}

func TestDetectByFileName(t *testing.T) {
	if Detect("internal/app.go").Tokens("func") == nil {
		t.Fatal("для .go язык не найден")
	}
	if Detect("notes.unknownext").Tokens("func") != nil {
		t.Fatal("для неизвестного расширения есть куски")
	}
}

func TestGoKinds(t *testing.T) {
	const text = `func main() { return 1 } "hi" // done`
	spans := Detect("main.go").Tokens(text)

	if got := textsOf(text, spans, Keyword); !slices.Equal(got, []string{"func", "return"}) {
		t.Fatalf("ключевые слова %q", got)
	}
	if got := textsOf(text, spans, Number); !slices.Equal(got, []string{"1"}) {
		t.Fatalf("числа %q", got)
	}
	if got := textsOf(text, spans, Comment); !slices.Equal(got, []string{"// done"}) {
		t.Fatalf("комментарии %q", got)
	}
	if got := textsOf(text, spans, String); !slices.Equal(got, []string{`"hi"`}) {
		t.Fatalf("строки %q", got)
	}
}

// Куски не перекрываются и лежат в границах строки: по ним дальше режут текст.
func TestSpansStayInsideText(t *testing.T) {
	const text = `x := "a" // b`
	end := 0
	for _, span := range Detect("a.go").Tokens(text) {
		if span.Start < end || span.End > len(text) || span.Start >= span.End {
			t.Fatalf("кусок %+v вне порядка или границ", span)
		}
		end = span.End
	}
}

// Package syntax находит в строке диффа ключевые слова, строки, комментарии и
// числа. Язык выбирается по имени файла: по содержимому угадывание даёт ложные
// цвета, а подсветка без уверенности хуже её отсутствия. Пакет не знает о
// цветах: тему и ширину задаёт tui.
package syntax

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

type Kind string

const (
	Keyword Kind = "keyword"
	String  Kind = "string"
	Comment Kind = "comment"
	Number  Kind = "number"
)

// Span это кусок строки в байтах с категорией. Куски не пересекаются и идут по
// возрастанию.
type Span struct {
	Start int
	End   int
	Kind  Kind
}

// Lexer это язык файла. Нулевое значение значит, что язык неизвестен и
// подсветки нет.
type Lexer struct {
	lexer chroma.Lexer
}

func Detect(path string) Lexer {
	lexer := lexers.Match(path)
	if lexer == nil {
		return Lexer{}
	}

	return Lexer{lexer: lexer}
}

// Tokens возвращает куски строки. Строка лексится сама по себе: состояние
// между строками не переносится, поэтому многострочные конструкции видны только
// в той части, что попала в строку.
func (l Lexer) Tokens(text string) []Span {
	if l.lexer == nil {
		return nil
	}

	iterator, err := l.lexer.Tokenise(nil, text)
	if err != nil {
		return nil
	}

	var spans []Span
	at := 0
	for _, token := range iterator.Tokens() {
		end := min(at+len(token.Value), len(text))
		if kind := kindOf(token.Type); kind != "" && end > at {
			spans = append(spans, Span{Start: at, End: end, Kind: kind})
		}
		at += len(token.Value)
	}

	return spans
}

func kindOf(tokenType chroma.TokenType) Kind {
	switch {
	case tokenType.InCategory(chroma.Keyword):
		return Keyword
	// строки и числа лежат в одной категории литералов, поэтому сравниваем
	// подкатегорию: иначе число попадает в строки
	case tokenType.InSubCategory(chroma.LiteralString):
		return String
	case tokenType.InSubCategory(chroma.LiteralNumber):
		return Number
	case tokenType.InCategory(chroma.Comment):
		return Comment
	}

	return ""
}

package diff

import (
	"unicode"
	"unicode/utf8"
)

// Span это отрезок строки в байтах: [Start, End).
type Span struct {
	Start int
	End   int
}

// maxCells ограничивает таблицу LCS. Строка, которую не уложить в неё, не
// подсвечивается: лучше без подсветки, чем таблица на мегабайты на каждую
// строку диффа.
// ponytail: LCS квадратичен, для длинных строк понадобится сравнение по
// префиксу и суффиксу.
const maxCells = 250_000

type token struct {
	text  string
	start int
	space bool
}

// tokens делит строку на слова, пробелы и знаки. Слово это подряд буквы,
// цифры и подчёркивание, пробелы идут одним куском, любой другой знак
// отдельным токеном.
func tokens(text string) []token {
	var result []token

	for index := 0; index < len(text); {
		r, size := utf8.DecodeRuneInString(text[index:])
		end := index + size

		switch {
		case isWordRune(r):
			end = extend(text, end, isWordRune)
		case unicode.IsSpace(r):
			end = extend(text, end, unicode.IsSpace)
		}

		result = append(result, token{text: text[index:end], start: index, space: unicode.IsSpace(r)})
		index = end
	}

	return result
}

// extend сдвигает конец, пока символы подходят под признак.
func extend(text string, end int, belongs func(rune) bool) int {
	for end < len(text) {
		r, size := utf8.DecodeRuneInString(text[end:])
		if !belongs(r) {
			break
		}
		end += size
	}

	return end
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// wordSpans возвращает изменённые отрезки обеих строк. Токены сравниваются по
// LCS: что не вошло в общую часть, то изменилось. Если совпала меньше половины
// значимых токенов, строка переписана целиком, и подсветка только шумит.
func wordSpans(oldText, newText string) ([]Span, []Span) {
	oldTokens, newTokens := tokens(oldText), tokens(newText)
	if len(oldTokens)*len(newTokens) > maxCells {
		return nil, nil
	}

	oldMatched, newMatched, common := matchTokens(oldTokens, newTokens)
	if common*2 < max(significant(oldTokens), significant(newTokens)) {
		return nil, nil
	}

	return unmatchedSpans(oldTokens, oldMatched), unmatchedSpans(newTokens, newMatched)
}

// matchTokens находит общую подпоследовательность токенов. common считает
// только значимые токены, пробелы в эту цифру не идут.
func matchTokens(oldTokens, newTokens []token) ([]bool, []bool, int) {
	n, m := len(oldTokens), len(newTokens)

	// table[i][j] длина LCS хвостов, начиная с i и j
	table := make([][]int, n+1)
	for index := range table {
		table[index] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if oldTokens[i].text == newTokens[j].text {
				table[i][j] = table[i+1][j+1] + 1
			} else {
				table[i][j] = max(table[i+1][j], table[i][j+1])
			}
		}
	}

	oldMatched, newMatched := make([]bool, n), make([]bool, m)
	common := 0
	for i, j := 0, 0; i < n && j < m; {
		switch {
		case oldTokens[i].text == newTokens[j].text:
			oldMatched[i], newMatched[j] = true, true
			if !oldTokens[i].space {
				common++
			}
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			i++
		default:
			j++
		}
	}

	return oldMatched, newMatched, common
}

func significant(tokens []token) int {
	count := 0
	for _, tok := range tokens {
		if !tok.space {
			count++
		}
	}

	return count
}

// unmatchedSpans собирает отрезки несовпавших токенов, соседние склеиваются.
func unmatchedSpans(tokens []token, matched []bool) []Span {
	var spans []Span

	for index, tok := range tokens {
		if matched[index] {
			continue
		}

		end := tok.start + len(tok.text)
		if last := len(spans) - 1; last >= 0 && spans[last].End == tok.start {
			spans[last].End = end
			continue
		}
		spans = append(spans, Span{Start: tok.start, End: end})
	}

	return spans
}

// wordMarks ставит пары в блоках: удалённые строки подряд, за ними добавленные.
// Пара это строки на одной позиции в своих блоках, как в split. Лишние строки
// пары не получают. Результат по индексу строки хунка.
func wordMarks(lines []Line) map[int][]Span {
	marks := make(map[int][]Span)

	for start := 0; start < len(lines); {
		if lines[start].Kind != LineRemoved {
			start++
			continue
		}

		end := start
		for end < len(lines) && lines[end].Kind == LineRemoved {
			end++
		}
		addedEnd := end
		for addedEnd < len(lines) && lines[addedEnd].Kind == LineAdded {
			addedEnd++
		}

		for offset := range min(end-start, addedEnd-end) {
			oldSpans, newSpans := wordSpans(lines[start+offset].Text, lines[end+offset].Text)
			marks[start+offset] = oldSpans
			marks[end+offset] = newSpans
		}

		start = addedEnd
	}

	return marks
}

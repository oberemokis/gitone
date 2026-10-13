package diff

import (
	"strings"
	"unicode/utf8"
)

// Экран всё равно обрежет строку по ширине колонки, а целиком длинная строка
// дорога на каждой перерисовке: строка в 1 МБ из минифицированного файла
// стоила 17 мс. ponytail: горизонтальной прокрутке понадобится больше.
const maxLineRunes = 1024

// clean готовит строку файла к выводу: срезает концевой `\r` перевода строки
// CRLF и обрезает длинную строку.
func clean(text string) string {
	return Printable(truncate(strings.TrimSuffix(text, "\r")))
}

// Printable готовит текст из диффа к выводу в терминал. Содержимое файла и
// имя файла не должны управлять терминалом: `\r` возвращает курсор в начало
// строки и ломает колонки split, escape-коды красят экран и сбрасывают фон
// строки. Поэтому управляющие символы заменяются значками U+2400–U+2421, C1 и
// битые байты на U+FFFD, а таб остаётся, его раскрывает экран. Пути
// File.OldPath и File.NewPath остаются сырыми: они нужны git как есть, а на
// экран идут через Printable.
func Printable(text string) string {
	text = strings.ToValidUTF8(text, string(utf8.RuneError))

	return strings.Map(func(char rune) rune {
		switch {
		case char == '\t':
			return char
		case char < 0x20:
			return 0x2400 + char
		case char == 0x7f:
			return 0x2421
		case char >= 0x80 && char <= 0x9f:
			return utf8.RuneError
		}
		return char
	}, text)
}

func truncate(text string) string {
	count := 0
	for index := range text {
		if count == maxLineRunes {
			return text[:index]
		}
		count++
	}

	return text
}

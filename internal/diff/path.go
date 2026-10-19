package diff

import "strings"

// headerPaths разбирает `a/X b/Y` из строки `diff --git`. Путь с пробелом
// здесь неоднозначен, поэтому это запасной источник: строки ---, +++, rename
// и copy его перекрывают. Где их нет (пустой файл, смена прав, бинарный),
// пути с обеих сторон совпадают, и строка делится ровно пополам.
func headerPaths(rest string) (string, string) {
	if quoted, ok := quotedPrefix(rest); ok {
		return path(quoted, "a/"), path(strings.TrimPrefix(rest[len(quoted):], " "), "b/")
	}

	if rest == "" {
		return "", ""
	}

	half := (len(rest) - 1) / 2
	return path(rest[:half], "a/"), path(rest[half+1:], "b/")
}

// path приводит путь из диффа к имени файла. Путь с пробелом git закрывает
// табом, путь с кавычкой или управляющим символом берёт в кавычки по правилам
// C (unquote). /dev/null значит, что файла с этой стороны нет.
func path(raw string, prefix string) string {
	raw = strings.TrimSuffix(raw, "\t")
	if raw == "/dev/null" {
		return ""
	}

	if strings.HasPrefix(raw, `"`) {
		if unquoted, ok := unquote(raw); ok {
			raw = unquoted
		}
	}

	return strings.TrimPrefix(raw, prefix)
}

// quotedPrefix это путь в кавычках в начале s.
func quotedPrefix(s string) (string, bool) {
	if !strings.HasPrefix(s, `"`) {
		return "", false
	}
	for index := 1; index < len(s); index++ {
		switch s[index] {
		case '\\':
			index++
		case '"':
			return s[:index+1], true
		}
	}
	return "", false
}

// unquote снимает кавычки git: `\ooo` это байт, `\t`, `\n` и остальные
// экраны C дают свой знак, прочие байты остаются как есть. strconv.Unquote не
// годится: сырой байт не UTF-8 в имени он заменил бы на U+FFFD.
func unquote(quoted string) (string, bool) {
	if len(quoted) < 2 || !strings.HasPrefix(quoted, `"`) || !strings.HasSuffix(quoted, `"`) {
		return "", false
	}

	body := quoted[1 : len(quoted)-1]
	var out strings.Builder
	for index := 0; index < len(body); index++ {
		if body[index] != '\\' {
			out.WriteByte(body[index])
			continue
		}
		index++
		if index >= len(body) {
			return "", false
		}

		switch escaped := body[index]; {
		case escaped >= '0' && escaped <= '3' && index+2 < len(body):
			value := 0
			for _, digit := range body[index : index+3] {
				if digit < '0' || digit > '7' {
					return "", false
				}
				value = value*8 + int(digit-'0')
			}
			out.WriteByte(byte(value))
			index += 2
		case strings.IndexByte("abtnvfr", escaped) >= 0:
			out.WriteByte("\a\b\t\n\v\f\r"[strings.IndexByte("abtnvfr", escaped)])
		default:
			out.WriteByte(escaped)
		}
	}
	return out.String(), true
}

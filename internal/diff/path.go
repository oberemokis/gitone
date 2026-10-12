package diff

import (
	"strconv"
	"strings"
)

// headerPaths разбирает `a/X b/Y` из строки `diff --git`. Путь с пробелом
// здесь неоднозначен, поэтому это запасной источник: строки ---, +++, rename
// и copy его перекрывают. Где их нет (пустой файл, смена прав, бинарный),
// пути с обеих сторон совпадают, и строка делится ровно пополам.
func headerPaths(rest string) (string, string) {
	if quoted, err := strconv.QuotedPrefix(rest); err == nil {
		return path(quoted, "a/"), path(strings.TrimPrefix(rest[len(quoted):], " "), "b/")
	}

	half := (len(rest) - 1) / 2
	if half < 0 {
		return "", ""
	}

	return path(rest[:half], "a/"), path(rest[half+1:], "b/")
}

// path приводит путь из диффа к имени файла. Путь с пробелом git закрывает
// табом, путь с кавычкой или управляющим символом берёт в кавычки по правилам
// C, их понимает strconv.Unquote. /dev/null значит, что файла с этой стороны нет.
func path(raw string, prefix string) string {
	raw = strings.TrimSuffix(raw, "\t")
	if raw == "/dev/null" {
		return ""
	}

	if strings.HasPrefix(raw, `"`) {
		if unquoted, err := strconv.Unquote(raw); err == nil {
			raw = unquoted
		}
	}

	return strings.TrimPrefix(raw, prefix)
}

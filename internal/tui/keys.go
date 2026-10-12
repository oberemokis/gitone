package tui

import "strings"

// ctrl+p показываем как «ctrl p», так читается в подвале critique
func displayKey(key string) string {
	return strings.ReplaceAll(key, "+", " ")
}

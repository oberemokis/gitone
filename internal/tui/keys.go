package tui

import (
	"strings"

	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// ctrl+p показываем как «ctrl p», так читается в подвале critique
func displayKey(key string) string {
	return strings.ReplaceAll(key, "+", " ")
}

// Клавиши внутри окон заданы в коде, их переназначение ждёт задачи #63.
// Подсказки и вопросы получают их подстановкой: перевод текста не должен
// менять клавишу, на которую отвечает код.
const (
	keyEnter = "enter"
	keyEsc   = "esc"
	keyYes   = "y"
	keyApply = "a"
	keyDrop  = "d"
)

func windowKeys() i18n.Args {
	return i18n.Args{"enter": keyEnter, "esc": keyEsc, "yes": keyYes, "apply": keyApply, "drop": keyDrop}
}

// Ключи словаря для подсказок keymap выводятся из имени действия и
// категории, а не лежат в keymap: тексты живут только в словаре.
func footerKey(action keymap.Action) string {
	return "action." + string(action) + ".footer"
}

func helpKey(action keymap.Action) string {
	return "action." + string(action) + ".help"
}

func categoryKey(category keymap.Category) string {
	return "category." + string(category)
}

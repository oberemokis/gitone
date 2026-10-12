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

// firstKey это первая клавиша действия для подсказки, пусто у отключённого.
func firstKey(keys keymap.Keymap, action keymap.Action) string {
	bound := keys.KeysFor(action)
	if len(bound) == 0 {
		return ""
	}

	return displayKey(bound[0])
}

// windowHint это строка подсказки окна: первая клавиша и подпись каждого
// действия в заданном порядке, закрытие последним. Отключённое действие в
// подсказку не идёт, как в подвале.
func windowHint(keys keymap.Keymap, text i18n.Dictionary, actions ...keymap.Action) string {
	var hints []string
	for _, action := range actions {
		if key := firstKey(keys, action); key != "" {
			hints = append(hints, key+" "+text.Text(hintKey(action), nil))
		}
	}

	return strings.Join(hints, "   ")
}

// questionArgs подставляет в вопрос клавишу ответа «да».
func questionArgs(keys keymap.Keymap) i18n.Args {
	return i18n.Args{"yes": firstKey(keys, keymap.QuestionYes)}
}

// Ключи словаря для подсказок keymap выводятся из имени действия и
// категории, а не лежат в keymap: тексты живут только в словаре.
func footerKey(action keymap.Action) string {
	return "action." + string(action) + ".footer"
}

func helpKey(action keymap.Action) string {
	return "action." + string(action) + ".help"
}

func hintKey(action keymap.Action) string {
	return "action." + string(action) + ".hint"
}

func categoryKey(category keymap.Category) string {
	return "category." + string(category)
}

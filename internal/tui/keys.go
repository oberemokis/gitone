package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"gitone/internal/i18n"
	"gitone/internal/keymap"
	"gitone/internal/tui/theme"
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
// действия в заданном порядке. Закрытие стоит в заголовке окна (closeHint).
// Отключённое действие в подсказку не идёт, как в подвале. Каждый кусок несёт
// фон окна сам: вложенный стиль со сбросом иначе погасил бы его.
func windowHint(colors theme.Theme, keys keymap.Keymap, text i18n.Dictionary, actions ...keymap.Action) string {
	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)

	var hints []string
	for _, action := range actions {
		if key := firstKey(keys, action); key != "" {
			hints = append(hints, keyHint(panel, colors, key, text.Text(hintKey(action), nil)))
		}
	}

	return strings.Join(hints, panel.Render("   "))
}

// keyHint это пара «клавиша подпись» для подвала и окон: клавиша цветом
// текста, подпись приглушённая, чтобы клавиша не терялась рядом с ней (#89).
func keyHint(base lipgloss.Style, colors theme.Theme, key, label string) string {
	return base.Render(key) + base.Foreground(colors.TextMuted).Render(" "+label)
}

// closeHint это «esc close» для заголовка окна по его действию закрытия.
// Отключённое закрытие даёт пустую строку, как в подсказках.
func closeHint(colors theme.Theme, keys keymap.Keymap, text i18n.Dictionary, action keymap.Action) string {
	key := firstKey(keys, action)
	if key == "" {
		return ""
	}

	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	return keyHint(panel, colors, key, text.Text("window.close", nil))
}

// windowTitle это заголовок окна шириной width, у всех окон как у справки:
// жирное название слева, закрытие справа (#107).
func windowTitle(colors theme.Theme, width int, title, close string) string {
	panel := lipgloss.NewStyle().Background(colors.BackgroundPanel)
	return justify(panel, width, panel.Bold(true).Render(title), close)
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

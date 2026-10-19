package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"gitone/internal/i18n"
	"gitone/internal/keymap"
	"gitone/internal/tui/theme"
	"gitone/internal/tui/zone"
)

// ctrl+p показываем как «ctrl p», так читается в подвале critique. Последний
// «+» это сама клавиша, а не разделитель: «+» остаётся «+», «ctrl++» читается
// «ctrl +».
func displayKey(key string) string {
	head, plus := strings.CutSuffix(key, "+")
	head = strings.ReplaceAll(head, "+", " ")
	if plus {
		head += "+"
	}

	return head
}

// firstKey это первая клавиша действия для подсказки, пусто у отключённого.
func firstKey(keys keymap.Keymap, action keymap.Action) string {
	bound := keys.KeysFor(action)
	if len(bound) == 0 {
		return ""
	}

	return displayKey(bound[0])
}

// paint это оформление окон: тема, словарь и клавиши (#351). Функции рисования
// получают его вместо трёх параметров, а фон окна и подсказки берут из его
// методов.
type paint struct {
	colors theme.Theme
	text   i18n.Dictionary
	keys   keymap.Keymap
}

func (m Model) paint() paint {
	return paint{colors: m.colors, text: m.text, keys: m.keys}
}

// panel это фон окна: на нём рисуется всё содержимое.
func (p paint) panel() lipgloss.Style {
	return lipgloss.NewStyle().Background(p.colors.BackgroundPanel)
}

// muted это приглушённый текст на фоне окна.
func (p paint) muted() lipgloss.Style {
	return p.panel().Foreground(p.colors.TextMuted)
}

// hint это строка подсказки окна: первая клавиша и подпись каждого действия в
// заданном порядке. Закрытие стоит в заголовке окна (close). Отключённое
// действие в подсказку не идёт, как в подвале. Каждый кусок несёт фон окна
// сам: вложенный стиль со сбросом иначе погасил бы его. Клик по подсказке
// нажимает её клавишу (#119).
func (p paint) hint(actions ...keymap.Action) string {
	panel := p.panel()

	var hints []string
	for _, action := range actions {
		if key := firstKey(p.keys, action); key != "" {
			hints = append(hints, pressable(action, keyHint(panel, p.colors, key, p.text.Text(hintKey(action), nil))))
		}
	}

	return strings.Join(hints, panel.Render("   "))
}

// pressable помечает подсказку местом клика: клик нажимает первую клавишу
// действия, как клавиатура (#119).
func pressable(action keymap.Action, hint string) string {
	return zone.Mark(pressTarget+string(action), hint)
}

// keyHint это пара «клавиша подпись» для подвала и окон: клавиша цветом
// текста, подпись приглушённая, чтобы клавиша не терялась рядом с ней (#89).
func keyHint(base lipgloss.Style, colors theme.Theme, key, label string) string {
	return base.Render(key) + base.Foreground(colors.TextMuted).Render(" "+label)
}

// close это «esc close» для заголовка окна по его действию закрытия.
// Отключённое закрытие даёт пустую строку, как в подсказках.
func (p paint) close(action keymap.Action) string {
	key := firstKey(p.keys, action)
	if key == "" {
		return ""
	}

	return pressable(action, keyHint(p.panel(), p.colors, key, p.text.Text("window.close", nil)))
}

// title это заголовок окна шириной width, у всех окон как у справки: жирное
// название слева, закрытие справа (#107).
func (p paint) title(width int, title, close string) string {
	panel := p.panel()
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

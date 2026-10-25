package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitone/internal/keymap"
	"gitone/internal/tui/zone"
)

const (
	// ширина содержимого окна без отступов
	helpWidth    = 44
	helpPaddingX = 4
	// строки окна, которые не прокручиваются: отступ сверху и снизу и заголовок
	helpChrome = 3
)

// helpBox не шире и не выше экрана: на узком окно ужимается, а длинные
// описания обрезаются, иначе окно уходило за край и теряло левые колонки.
// Строки под заголовком, которые не поместились по высоте, прокручиваются:
// top это первая показанная (#152). Клик по строке закрывает справку и делает
// действие строки, как меню клавиш lazygit (#119).
func helpBox(p paint, screenWidth, screenHeight, top int) string {
	width := max(min(helpWidth, screenWidth-2*helpPaddingX), 1)

	// заголовок как у всех окон; клавиши цветом текста, подписи
	// приглушённые, как в подвале (#89)
	rows := []string{p.title(width, p.text.Text("help.title", nil), p.close(keymap.HelpClose))}

	body := helpBody(p, width)
	if page := helpPage(screenHeight); len(body) > page {
		start := min(top, len(body)-page)
		body = body[start : start+page]
	}

	// MaxHeight держит окно в теле и на экране ниже трёх строк, где под отступы
	// и заголовок места нет
	return p.panel().Padding(1, helpPaddingX).MaxHeight(screenHeight).Render(strings.Join(append(rows, body...), "\n"))
}

// helpPage это сколько строк под заголовком помещается в теле высотой height.
func helpPage(height int) int {
	return max(height-helpChrome, 0)
}

// helpShown говорит, что справка на экране: окна списков закрывают её собой,
// вопрос с главного экрана нет.
func (m Model) helpShown() bool {
	return m.helpOpen && (m.active == noWindow || m.footerQuestion())
}

// helpOverflow это на сколько строк справка выше тела. Ноль значит, что она
// помещается целиком или её не видно: прокручивать нечего.
func (m Model) helpOverflow() int {
	if !m.helpShown() {
		return 0
	}

	body := helpBody(m.paint(), m.width)
	return max(len(body)-helpPage(m.bodyHeight()), 0)
}

// scrollHelp сдвигает справку на delta строк, не дальше её краёв.
func (m Model) scrollHelp(delta int) Model {
	m.helpTop = max(min(m.helpTop+delta, m.helpOverflow()), 0)
	return m
}

// helpKey двигает справку клавишами списка, пока она выше тела: иначе её
// нижние строки не достать. Справка клавиш не забирает, это единственное
// исключение, а когда она помещается, j и k работают как раньше. handled
// false значит, что клавиша идёт дальше.
func (m Model) helpKey(pressed string) (next Model, handled bool) {
	if m.helpOverflow() == 0 {
		return m, false
	}

	switch {
	case m.keys.Is(keymap.ListDown, pressed):
		return m.scrollHelp(1), true
	case m.keys.Is(keymap.ListUp, pressed):
		return m.scrollHelp(-1), true
	}

	return m, false
}

// wheelHelp двигает справку колесом на столько же строк, что и дифф. Вбок
// справке двигаться некуда.
func (m Model) wheelHelp(button tea.MouseButton) Model {
	switch button {
	case tea.MouseWheelDown:
		return m.scrollHelp(wheelStep)
	case tea.MouseWheelUp:
		return m.scrollHelp(-wheelStep)
	}

	return m
}

// helpBody это строки под заголовком: группы действий, перед каждой пустая
// строка.
func helpBody(p paint, width int) []string {
	// Каждый кусок строки несёт фон сам: Lip Gloss закрывает жирный полным
	// сбросом стиля, и без этого фон панели гаснет до конца строки.
	panel, dim := p.panel(), p.muted()
	heading := panel.Bold(true).Foreground(p.colors.Accent)

	var rows []string
	order, groups := groupByCategory(p.keys.Bindings())
	for _, category := range order {
		rows = append(rows, "", justify(panel, width, heading.Render(p.text.Text(categoryKey(category), nil)), ""))

		for _, binding := range groups[category] {
			displayed := make([]string, len(binding.Keys))
			for index, key := range binding.Keys {
				displayed[index] = displayKey(key)
			}

			description := p.text.Text(helpKey(binding.Action), nil)
			row := justify(panel, width, dim.Render(description), panel.Render(strings.Join(displayed, ", ")))
			rows = append(rows, zone.Mark(helpTarget+string(binding.Action), row))
		}
	}

	return rows
}

// groupByCategory пропускает действия не для справки и без клавиш: им не
// место в справке.
func groupByCategory(all []keymap.Binding) ([]keymap.Category, map[keymap.Category][]keymap.Binding) {
	var order []keymap.Category
	groups := map[keymap.Category][]keymap.Binding{}

	for _, binding := range all {
		if !binding.Help || len(binding.Keys) == 0 {
			continue
		}

		if _, seen := groups[binding.Category]; !seen {
			order = append(order, binding.Category)
		}
		groups[binding.Category] = append(groups[binding.Category], binding)
	}

	return order, groups
}

// justify прижимает left к левому краю, right к правому. Не влезает, left
// обрезается: клавиши справа важнее хвоста описания.
func justify(panel lipgloss.Style, width int, left, right string) string {
	left = truncate(left, max(width-lipgloss.Width(right)-1, 0), "…")
	gap := max(width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return left + panel.Render(strings.Repeat(" ", gap)) + right
}

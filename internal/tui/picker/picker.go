// Package picker это окно выбора из списка с нечётким поиском, в стиле окон
// opencode. Это поле ввода, поэтому оно получает нажатия напрямую, а не
// действия: q и ? здесь набираются в поиск. Что делать с выбором, решает
// корень.
package picker

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/tui/theme"
)

// Item это строка списка. Detail стоит справа серым и различает записи с
// одинаковым путём, например удалённый файл и симлинк на его месте.
type Item struct {
	Label  string
	Detail string
}

type Outcome int

const (
	Pending Outcome = iota
	Chosen
	Closed
)

type Model struct {
	title   string
	items   []Item
	query   string
	matches []int
	// позиция выделения в matches
	selected int
	colors   theme.Theme
}

func New(title string, items []Item, colors theme.Theme) Model {
	return Model{title: title, items: items, colors: colors}.filter()
}

// HandleKey разбирает нажатие. Chosen значит, что выбран элемент Selected,
// Closed значит, что окно закрыли без выбора.
func (m Model) HandleKey(key tea.KeyPressMsg) (Model, Outcome) {
	switch key.String() {
	case "esc":
		return m, Closed
	case "enter":
		if len(m.matches) == 0 {
			return m, Pending
		}
		return m, Chosen
	case "up", "ctrl+p":
		m.selected = max(m.selected-1, 0)
	case "down", "ctrl+n":
		m.selected = min(m.selected+1, max(len(m.matches)-1, 0))
	case "backspace":
		if m.query != "" {
			_, size := utf8.DecodeLastRuneInString(m.query)
			m.query = m.query[:len(m.query)-size]
			m = m.filter()
		}
	default:
		if key.Text != "" {
			m.query += key.Text
			m = m.filter()
		}
	}

	return m, Pending
}

// SelectedItem это выбранный элемент. found false, если под поиск ничего не
// подошло. Номер в списке не отдаётся: список, по которому строилось окно, у
// вызывающего мог смениться, пока окно открыто.
func (m Model) SelectedItem() (item Item, found bool) {
	if len(m.matches) == 0 {
		return Item{}, false
	}

	return m.items[m.matches[m.selected]], true
}

// filter оставляет элементы, в которых символы поиска идут по порядку. Порядок
// списка не меняется: ранжирования нет (решение в #11).
func (m Model) filter() Model {
	m.matches = m.matches[:0]
	for index, item := range m.items {
		if isSubsequence(m.query, item.Label) {
			m.matches = append(m.matches, index)
		}
	}

	m.selected = 0
	return m
}

func isSubsequence(query, text string) bool {
	text = strings.ToLower(text)
	for _, char := range strings.ToLower(query) {
		position := strings.IndexRune(text, char)
		if position < 0 {
			return false
		}
		text = text[position+utf8.RuneLen(char):]
	}

	return true
}

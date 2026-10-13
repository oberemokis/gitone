// Package picker это выбор из списка с нечётким поиском: запрос, найденное и
// выделение. Это поле ввода, поэтому оно получает нажатия напрямую, а не
// действия: q и ? здесь набираются в поиск. Клавиши своих действий (выбрать,
// закрыть, вверх, вниз) оно получает от корня строками, остальное набирается.
// Рисует окно и решает, что делать с выбором, корень: окно выглядит как
// остальные списки (#137).
package picker

import (
	"slices"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
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

// Keys это клавиши действий окна, как их возвращает KeyPressMsg.String().
// backspace стирает символ всегда, остальное набирается в поиск.
type Keys struct {
	Choose, Close, Up, Down []string
}

type Model struct {
	keys    Keys
	items   []Item
	query   string
	matches []int
	// позиция выделения в matches
	selected int
}

func New(keys Keys, items []Item) Model {
	return Model{keys: keys, items: items}.filter()
}

// Query это набранный запрос.
func (m Model) Query() string {
	return m.query
}

// Shown это найденные элементы по порядку списка, Selected позиция выделения
// среди них.
func (m Model) Shown() []Item {
	shown := make([]Item, len(m.matches))
	for position, index := range m.matches {
		shown[position] = m.items[index]
	}
	return shown
}

func (m Model) Selected() int {
	return m.selected
}

// HandleKey разбирает нажатие. Chosen значит, что выбран элемент Selected,
// Closed значит, что окно закрыли без выбора.
func (m Model) HandleKey(key tea.KeyPressMsg) (Model, Outcome) {
	pressed := key.String()
	switch {
	case slices.Contains(m.keys.Close, pressed):
		return m, Closed
	case slices.Contains(m.keys.Choose, pressed):
		if len(m.matches) == 0 {
			return m, Pending
		}
		return m, Chosen
	case slices.Contains(m.keys.Up, pressed):
		m = m.Move(-1)
	case slices.Contains(m.keys.Down, pressed):
		m = m.Move(1)
	case pressed == "backspace":
		if m.query != "" {
			_, size := utf8.DecodeLastRuneInString(m.query)
			m.query = m.query[:len(m.query)-size]
			m = m.filter()
		}
	default:
		if key.Text != "" {
			m = m.Type(key.Text)
		}
	}

	return m, Pending
}

// Type дописывает text к запросу, как набранные клавиши: так приходит вставка
// из буфера.
func (m Model) Type(text string) Model {
	m.query += text
	return m.filter()
}

// Move сдвигает выделение на delta позиций среди найденных, не дальше первой
// и последней: так его двигают клавиши и колесо мыши.
func (m Model) Move(delta int) Model {
	m.selected = max(min(m.selected+delta, len(m.matches)-1), 0)
	return m
}

// Select ставит выделение на позицию среди найденных (Shown), например по
// клику на строку. Позиция за краем списка выделение не трогает.
func (m Model) Select(position int) Model {
	if position >= 0 && position < len(m.matches) {
		m.selected = position
	}
	return m
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

package diffview

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Длинная строка либо переносится (diff.wrap), либо сдвигается вбок вместе
// со всем кодом файла. Оба режима видят кусок кода с колонки skip: при сдвиге
// это shift, при переносе номер куска на ширину колонки (#87).

// WithWrap включает перенос длинных строк (diff.wrap в config.json).
func (m Model) WithWrap(wrap bool) Model {
	m.wrap = wrap
	return m.rebuild()
}

// ToggleWrap переключает перенос и держит на месте верхнюю строку файла: с
// переносом строк экрана больше, и прежняя прокрутка показала бы другое
// место.
func (m Model) ToggleWrap() Model {
	top := m.topPosition()
	m.wrap = !m.wrap
	m.shift = 0
	return m.rebuild().seek(top)
}

// ScrollRight сдвигает код вбок на delta колонок, отрицательный delta влево.
// Сдвиг не уходит левее начала и дальше конца самой длинной строки. С
// переносом сдвигать нечего.
func (m Model) ScrollRight(delta int) Model {
	if m.wrap {
		return m
	}

	m.shift = max(min(m.shift+delta, m.maxShift()), 0)
	return m.clamp()
}

// prefixWidth это ширина отступа, номера и знака перед кодом: « 12 + ».
func (m Model) prefixWidth() int {
	if m.hideNumbers {
		return 3
	}
	return m.numberWidth + 4
}

// codeWidths это ширина кода в левой и правой колонке split или в единственной
// колонке unified (тогда правая 0).
func (m Model) codeWidths() (int, int) {
	if m.effectiveMode() == ModeSplit {
		left := m.width / 2
		return left - m.prefixWidth(), m.width - left - m.prefixWidth()
	}
	return m.width - m.prefixWidth(), 0
}

// maxShift это сдвиг, при котором конец самой длинной строки встаёт к правому
// краю самой узкой колонки.
func (m Model) maxShift() int {
	visible, _ := m.codeWidths()
	return max(m.longest-visible, 0)
}

// hasBar: полоса прокрутки нужна, только когда есть что сдвигать.
func (m Model) hasBar() bool {
	return !m.wrap && m.maxShift() > 0 && m.height > 1
}

// scrollbar это полоса горизонтальной прокрутки во всю ширину панели, как в
// opencode: дорожка цветом окон, ползунок приглушённым цветом. Полоса из
// нижних полублоков, без фона клетки: она вдвое тоньше строки (#100). Длина
// ползунка это видимая доля самой длинной строки, место это сдвиг.
func (m Model) scrollbar() string {
	visible, _ := m.codeWidths()
	thumb := max(m.width*visible/m.longest, 1)
	start := min(m.width*m.shift/m.longest, m.width-thumb)
	if m.shift == m.maxShift() {
		start = m.width - thumb
	}

	track := lipgloss.NewStyle().Foreground(m.colors.BackgroundPanel)
	knob := lipgloss.NewStyle().Foreground(m.colors.TextMuted)
	return cells(track, start) + cells(knob, thumb) + cells(track, m.width-start-thumb)
}

// halfBlock это нижний полублок: полоса из него вдвое тоньше клетки.
const halfBlock = "▄"

// cells это count полублоков в стиле style; ноль без кодов стиля.
func cells(style lipgloss.Style, count int) string {
	if count <= 0 {
		return ""
	}
	return style.Render(strings.Repeat(halfBlock, count))
}

// parts режет строку файла на строки экрана. Без переноса она одна, с
// переносом столько, сколько нужно самой длинной из её сторон.
func (m Model) parts(row line) []line {
	count := 1
	if m.wrap {
		left, right := row.textWidths()
		leftWidth, rightWidth := m.codeWidths()
		count = max(pieceCount(left, leftWidth), pieceCount(right, rightWidth))
	}

	result := make([]line, count)
	for part := range count {
		row.part = part
		result[part] = row
	}
	return result
}

// skip это колонка кода, с которой начинается видимая часть строки шириной
// width.
func (m Model) skip(part, width int) int {
	if m.wrap {
		return part * width
	}
	return m.shift
}

// pieceCount это число кусков ширины size в тексте ширины width, не меньше
// одного.
func pieceCount(width, size int) int {
	if size <= 0 || width <= size {
		return 1
	}
	return (width + size - 1) / size
}

// textWidths это ширина кода строки: левой и правой стороны split или
// единственной стороны unified. Таб считается четырьмя пробелами, как при
// отрисовке.
func (l line) textWidths() (int, int) {
	switch {
	case l.split != nil:
		return textWidth(l.split.Left.Text), textWidth(l.split.Right.Text)
	case l.unified != nil:
		return textWidth(l.unified.Text), 0
	}
	return 0, 0
}

func textWidth(text string) int {
	return ansi.StringWidth(strings.ReplaceAll(text, "\t", tabSpaces))
}

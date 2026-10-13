package diffview

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Длинная строка либо переносится (diff.wrap), либо сдвигается вбок вместе
// со всем кодом файла. Оба режима видят кусок кода с колонки skip: при сдвиге
// это shift, при переносе начало куска, которое режет по границам символов
// (#87).

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
		left := m.splitLeft()
		return left - m.prefixWidth(), m.width - left - m.prefixWidth()
	}
	return m.width - m.prefixWidth(), 0
}

// visibleWidth это ширина кода в самой узкой колонке: половины split бывают
// разной ширины (#122).
func (m Model) visibleWidth() int {
	left, right := m.codeWidths()
	if m.effectiveMode() == ModeSplit {
		return min(left, right)
	}
	return left
}

// maxShift это сдвиг, при котором конец самой длинной строки встаёт к правому
// краю самой узкой колонки.
func (m Model) maxShift() int {
	return max(m.longest-m.visibleWidth(), 0)
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
	thumb := max(m.width*m.visibleWidth()/m.longest, 1)
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

// chunk это кусок строки файла при переносе: его номер, 0 у первого, и колонка
// кода, с которой кусок начинается на левой и правой стороне split. У каждой
// стороны свои колонки: символы разной ширины режут половины в разных местах.
type chunk struct {
	part  int
	skips [2]int
}

// parts режет строку файла на строки экрана. Без переноса она одна, с
// переносом столько, сколько нужно самой длинной из её сторон.
func (m Model) parts(row line) []line {
	if !m.wrap {
		return []line{row}
	}

	left, right := row.texts()
	leftWidth, rightWidth := m.codeWidths()
	leftCuts, rightCuts := cuts(left, leftWidth), cuts(right, rightWidth)

	result := make([]line, max(len(leftCuts), len(rightCuts))-1)
	for part := range result {
		row.part = part
		// у стороны с меньшим числом кусков остаток пуст: колонка за концом текста
		row.skips = [2]int{leftCuts[min(part, len(leftCuts)-1)], rightCuts[min(part, len(rightCuts)-1)]}
		result[part] = row
	}
	return result
}

// cuts это колонки, где текст режется на куски шириной не больше width: начало
// каждого куска и в конце ширина текста. Режет по границам кластеров: символ,
// не поместившийся в хвост куска, целиком уходит в начало следующего, а не
// теряется на стыке. Кластер шире куска остаётся в куске один.
func cuts(text string, width int) []int {
	if width <= 0 {
		return []int{0, ansi.StringWidth(text)}
	}

	cuts, column := []int{0}, 0
	for text != "" {
		cluster, size := ansi.FirstGraphemeCluster(text, ansi.GraphemeWidth)
		text = text[len(cluster):]

		if start := cuts[len(cuts)-1]; column > start && column+size > start+width {
			cuts = append(cuts, column)
		}
		column += size
	}
	return append(cuts, column)
}

// skip это колонка кода, с которой начинается видимая часть строки на стороне
// side: сдвиг, а при переносе начало куска.
func (m Model) skip(at chunk, side int) int {
	if m.wrap {
		return at.skips[side]
	}
	return m.shift
}

// texts это код строки как он стоит на экране: левой и правой стороны split
// или единственной стороны unified. Таб занимает четыре пробела, как при
// отрисовке, значок ␍ одну клетку.
func (l line) texts() (string, string) {
	switch {
	case l.split != nil:
		return shown(l.split.Left.Text, l.split.Left.CR), shown(l.split.Right.Text, l.split.Right.CR)
	case l.unified != nil:
		return shown(l.unified.Text, l.unified.CR), ""
	}
	return "", ""
}

func shown(text string, cr bool) string {
	if cr {
		text += crMark
	}

	return strings.ReplaceAll(text, "\t", tabSpaces)
}

// textWidths это ширина кода строки по сторонам, как texts.
func (l line) textWidths() (int, int) {
	left, right := l.texts()
	return ansi.StringWidth(left), ansi.StringWidth(right)
}

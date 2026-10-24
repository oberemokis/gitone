package diffview

import "math"

// minHalf это самая узкая половина split при перетаскивании стыка (#122):
// номер, знак и хоть немного кода.
const minHalf = 20

// SplitLeft это ширина левой половины split на экране; found false в
// unified, где половин нет, и в ленте, где на экране нет файла в split.
func (m Model) SplitLeft() (left int, found bool) {
	if m.effectiveMode() != ModeSplit || (m.stream.on && !m.splitShown()) {
		return 0, false
	}
	return m.splitLeft(), true
}

// WithSplit ставит стык половин split так, чтобы левая была шириной left
// колонок, но обе не уже minHalf. Хранится доля, а не колонки: при смене
// ширины половины сохраняют соотношение. Перенос строк зависит от ширины
// половин, поэтому строки пересчитываются, а верхняя строка файла остаётся
// на месте.
func (m Model) WithSplit(left int) Model {
	if m.width < 2*minHalf {
		return m
	}

	return m.keepTop(func(m Model) Model {
		m.share = float64(min(max(left, minHalf), m.width-minHalf)) / float64(m.width)
		return m
	})
}

// splitLeft это ширина левой половины: пополам, пока стык не тянули, иначе
// доля share от ширины. На узкой панели всегда пополам.
func (m Model) splitLeft() int {
	if m.share == 0 || m.width < 2*minHalf {
		return m.width / 2
	}

	left := int(math.Round(float64(m.width) * m.share))
	return min(max(left, minHalf), m.width-minHalf)
}

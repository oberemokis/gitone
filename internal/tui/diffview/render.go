package diffview

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/diff"
)

// Таб раскрывается до обрезки: ширину строки иначе не посчитать, а Lip Gloss
// раскрыл бы его сам при отрисовке, и строка вылезла бы за колонку.
// ponytail: замена, а не позиции табуляции, таб посреди строки не выравнивает
// текст по колонкам.
const tabSpaces = "    "

const noNewlineText = "No newline at end of file"

func (m Model) render(line line) string {
	switch {
	case line.notice != nil:
		return m.renderNotice(*line.notice)
	case line.split != nil:
		return m.renderSplit(*line.split)
	default:
		return m.renderUnified(*line.unified)
	}
}

func (m Model) renderNotice(note notice) string {
	style := lipgloss.NewStyle()
	if !note.plain {
		style = style.Foreground(m.colors.TextMuted)
	}

	return style.Render(fill(m.width, note.text))
}

func (m Model) renderUnified(row diff.UnifiedRow) string {
	switch row.Kind {
	case diff.RowGap:
		return ""
	case diff.RowNoNewline:
		return m.noNewline(m.width, 2)
	}

	return m.cell(m.width, row.Line, []int{row.OldNumber, row.NewNumber}, row.Text)
}

// renderSplit делит ширину пополам. Половина без строки остаётся пустой и без
// фона, чтобы было видно, с какой стороны строки нет.
func (m Model) renderSplit(row diff.SplitRow) string {
	if row.Kind == diff.RowGap {
		return ""
	}

	leftWidth := m.width / 2
	return m.half(leftWidth, row.Kind, row.Left) + m.half(m.width-leftWidth, row.Kind, row.Right)
}

func (m Model) half(width int, kind diff.RowKind, side diff.Cell) string {
	switch {
	case side.Kind == "":
		return strings.Repeat(" ", width)
	case kind == diff.RowNoNewline:
		return m.noNewline(width, 1)
	}

	return m.cell(width, side.Kind, []int{side.Number}, side.Text)
}

// cell рисует строку файла шириной ровно width: номера, знак и текст,
// обрезанный по краю. Каждый кусок несёт фон строки сам: вложенный стиль
// со сбросом иначе гасит фон до конца строки.
func (m Model) cell(width int, kind diff.LineKind, numbers []int, text string) string {
	base := lipgloss.NewStyle()
	marker := " "
	switch kind {
	case diff.LineAdded:
		base = base.Background(m.colors.DiffAddedBg)
		marker = "+"
	case diff.LineRemoved:
		base = base.Background(m.colors.DiffRemovedBg)
		marker = "-"
	}

	columns := make([]string, len(numbers))
	for index, number := range numbers {
		columns[index] = m.formatNumber(number)
	}
	prefix := strings.Join(columns, " ") + " " + marker + " "

	return base.Foreground(m.colors.TextMuted).Render(prefix) + base.Render(fill(width-ansi.StringWidth(prefix), text))
}

// noNewline ставит пометку туда, где у строки начинается текст: под колонки
// номеров и знак.
func (m Model) noNewline(width, numberColumns int) string {
	indent := strings.Repeat(" ", numberColumns*(m.numberWidth+1)+2)
	return lipgloss.NewStyle().Foreground(m.colors.TextMuted).Render(fill(width, indent+noNewlineText))
}

// formatNumber выравнивает номер по правому краю. Ноль значит, что строки с
// этой стороны нет, и колонка остаётся пустой.
func (m Model) formatNumber(number int) string {
	if number == 0 {
		return strings.Repeat(" ", m.numberWidth)
	}

	return fmt.Sprintf("%*d", m.numberWidth, number)
}

// fill обрезает текст по ширине и дополняет пробелами ровно до неё.
func fill(width int, text string) string {
	width = max(width, 0)
	content := ansi.Truncate(strings.ReplaceAll(text, "\t", tabSpaces), width, "")
	return content + strings.Repeat(" ", max(width-ansi.StringWidth(content), 0))
}

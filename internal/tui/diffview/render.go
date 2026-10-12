package diffview

import (
	"fmt"
	"image/color"
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

	return m.cell(m.width, row.Line, []int{row.OldNumber, row.NewNumber}, row.Text, row.Changed)
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

	return m.cell(width, side.Kind, []int{side.Number}, side.Text, side.Changed)
}

// cell рисует строку файла шириной ровно width: номера, знак и текст,
// обрезанный по краю. Каждый кусок несёт фон строки сам: вложенный стиль
// со сбросом иначе гасит фон до конца строки.
func (m Model) cell(width int, kind diff.LineKind, numbers []int, text string, changed []diff.Span) string {
	base := lipgloss.NewStyle()
	wordBg := m.colors.DiffAddedWordBg
	marker := " "
	switch kind {
	case diff.LineAdded:
		base = base.Background(m.colors.DiffAddedBg)
		marker = "+"
	case diff.LineRemoved:
		base = base.Background(m.colors.DiffRemovedBg)
		wordBg = m.colors.DiffRemovedWordBg
		marker = "-"
	}

	prefix := marker + " "
	if !m.hideNumbers {
		columns := make([]string, len(numbers))
		for index, number := range numbers {
			columns[index] = m.formatNumber(number)
		}
		prefix = strings.Join(columns, " ") + " " + marker + " "
	}

	return base.Foreground(m.colors.TextMuted).Render(prefix) + m.textCells(width-ansi.StringWidth(prefix), base, wordBg, text, changed)
}

// textCells рисует текст ровно на width клеток. Изменённые слова идут с фоном
// слов, остальное с фоном строки. Таб раскрывается до обрезки, поэтому отрезки
// относятся к исходному тексту, а ширина считается по раскрытому.
func (m Model) textCells(width int, base lipgloss.Style, wordBg color.Color, text string, changed []diff.Span) string {
	var out strings.Builder
	left := max(width, 0)

	for _, piece := range pieces(text, changed) {
		if left == 0 {
			break
		}

		content := ansi.Truncate(strings.ReplaceAll(piece.text, "\t", tabSpaces), left, "")
		style := base
		if piece.changed {
			style = base.Background(wordBg)
		}

		out.WriteString(style.Render(content))
		left -= ansi.StringWidth(content)
	}

	return out.String() + base.Render(strings.Repeat(" ", left))
}

// piece это кусок строки: слово из пары или остальной текст.
type piece struct {
	text    string
	changed bool
}

// pieces режет текст по отрезкам. Отрезки идут по возрастанию и не
// пересекаются, как их отдаёт пакет diff.
func pieces(text string, changed []diff.Span) []piece {
	var result []piece
	at := 0

	for _, span := range changed {
		if span.Start > at {
			result = append(result, piece{text: text[at:span.Start]})
		}
		result = append(result, piece{text: text[span.Start:span.End], changed: true})
		at = span.End
	}

	if at < len(text) {
		result = append(result, piece{text: text[at:]})
	}

	return result
}

// noNewline ставит пометку туда, где у строки начинается текст: под колонки
// номеров и знак.
func (m Model) noNewline(width, numberColumns int) string {
	numbersWidth := numberColumns * (m.numberWidth + 1)
	if m.hideNumbers {
		numbersWidth = 0
	}

	indent := strings.Repeat(" ", numbersWidth+2)
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

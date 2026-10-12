package diffview

import (
	"fmt"
	"image/color"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/diff"
	"gitone/internal/syntax"
)

// Таб раскрывается до обрезки: ширину строки иначе не посчитать, а Lip Gloss
// раскрыл бы его сам при отрисовке, и строка вылезла бы за колонку.
// ponytail: замена, а не позиции табуляции, таб посреди строки не выравнивает
// текст по колонкам.
const tabSpaces = "    "

func (m Model) render(line line) string {
	switch {
	case line.notice != nil:
		return m.renderNotice(*line.notice)
	case line.split != nil:
		return m.renderSplit(*line.split, line.part)
	default:
		return m.renderUnified(*line.unified, line.part)
	}
}

func (m Model) renderNotice(note notice) string {
	style := lipgloss.NewStyle()
	if !note.plain {
		style = style.Foreground(m.colors.TextMuted)
	}

	return style.Render(fill(m.width, note.text))
}

func (m Model) renderUnified(row diff.UnifiedRow, part int) string {
	switch row.Kind {
	case diff.RowGap:
		return ""
	case diff.RowNoNewline:
		return m.noNewline(m.width, 2)
	}

	return m.cell(m.width, row.Line, []int{row.OldNumber, row.NewNumber}, row.Text, row.Changed, part)
}

// renderSplit делит ширину пополам. Половина без строки остаётся пустой и без
// фона, чтобы было видно, с какой стороны строки нет.
func (m Model) renderSplit(row diff.SplitRow, part int) string {
	if row.Kind == diff.RowGap {
		return ""
	}

	leftWidth := m.width / 2
	return m.half(leftWidth, row.Kind, row.Left, part) + m.half(m.width-leftWidth, row.Kind, row.Right, part)
}

func (m Model) half(width int, kind diff.RowKind, side diff.Cell, part int) string {
	switch {
	case side.Kind == "":
		return strings.Repeat(" ", width)
	case kind == diff.RowNoNewline:
		return m.noNewline(width, 1)
	}

	return m.cell(width, side.Kind, []int{side.Number}, side.Text, side.Changed, part)
}

// cell рисует строку файла шириной ровно width: номера, знак и текст,
// обрезанный по краю. Каждый кусок несёт фон строки сам: вложенный стиль
// со сбросом иначе гасит фон до конца строки. У изменённой строки номер и
// знак идут её цветом на своём фоне, светлее фона кода: так изменение видно
// по краю, даже когда код длинный. Продолжение перенесённой строки (part
// больше нуля) идёт без номера и знака, с тем же фоном.
func (m Model) cell(width int, kind diff.LineKind, numbers []int, text string, changed []diff.Span, part int) string {
	base := lipgloss.NewStyle()
	gutter := base.Foreground(m.colors.TextMuted)
	wordBg := m.colors.DiffAddedWordBg
	marker := " "
	switch kind {
	case diff.LineAdded:
		base = base.Background(m.colors.DiffAddedBg)
		gutter = lipgloss.NewStyle().Background(m.colors.DiffAddedNumberBg).Foreground(m.colors.DiffAdded)
		marker = "+"
	case diff.LineRemoved:
		base = base.Background(m.colors.DiffRemovedBg)
		gutter = lipgloss.NewStyle().Background(m.colors.DiffRemovedNumberBg).Foreground(m.colors.DiffRemoved)
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
	if part > 0 {
		prefix = strings.Repeat(" ", ansi.StringWidth(prefix))
	}

	codeWidth := width - ansi.StringWidth(prefix)
	code := m.lexer.Tokens(text)
	return gutter.Render(prefix) + m.textCells(codeWidth, m.skip(part, codeWidth), base, wordBg, text, changed, code)
}

// textCells рисует текст ровно на width клеток, начиная с колонки skip:
// сдвинутый вбок или перенесённый код. Изменённые слова идут с фоном слов,
// категории синтаксиса с цветом текста, остальное с фоном строки. Таб
// раскрывается до обрезки, поэтому отрезки относятся к исходному тексту, а
// ширина считается по раскрытому.
func (m Model) textCells(width, skip int, base lipgloss.Style, wordBg color.Color, text string, changed []diff.Span, code []syntax.Span) string {
	var out strings.Builder
	left := max(width, 0)

	for _, piece := range pieces(text, changed, code) {
		if left == 0 {
			break
		}

		content := strings.ReplaceAll(piece.text, "\t", tabSpaces)
		if pieceWidth := ansi.StringWidth(content); skip >= pieceWidth {
			skip -= pieceWidth
			continue
		}
		content = ansi.Truncate(ansi.TruncateLeft(content, skip, ""), left, "")
		skip = 0
		style := m.pieceStyle(base, piece, wordBg)

		out.WriteString(style.Render(content))
		left -= ansi.StringWidth(content)
	}

	return out.String() + base.Render(strings.Repeat(" ", left))
}

// pieceStyle красит кусок: цвет категории синтаксиса, фон слова, если слово
// изменено. Фон строки остаётся от base.
func (m Model) pieceStyle(base lipgloss.Style, piece piece, wordBg color.Color) lipgloss.Style {
	style := base
	if foreground := m.syntaxColor(piece.kind); foreground != nil {
		style = style.Foreground(foreground)
	}
	if piece.changed {
		style = style.Background(wordBg)
	}

	return style
}

func (m Model) syntaxColor(kind syntax.Kind) color.Color {
	switch kind {
	case syntax.Keyword:
		return m.colors.SyntaxKeyword
	case syntax.String:
		return m.colors.SyntaxString
	case syntax.Comment:
		return m.colors.SyntaxComment
	case syntax.Number:
		return m.colors.SyntaxNumber
	}

	return nil
}

// piece это кусок строки с признаком изменённого слова и категорией синтаксиса.
type piece struct {
	text    string
	changed bool
	kind    syntax.Kind
}

// pieces режет текст по всем границам сразу: отрезков слов и категорий. Внутри
// одного куска ни слово, ни категория не меняются.
func pieces(text string, changed []diff.Span, code []syntax.Span) []piece {
	cuts := []int{0, len(text)}
	for _, span := range changed {
		cuts = append(cuts, span.Start, span.End)
	}
	for _, span := range code {
		cuts = append(cuts, span.Start, span.End)
	}
	slices.Sort(cuts)
	cuts = slices.Compact(cuts)

	var result []piece
	for index := 0; index+1 < len(cuts); index++ {
		start, end := cuts[index], cuts[index+1]
		result = append(result, piece{
			text:    text[start:end],
			changed: coveredBy(start, changed),
			kind:    kindAt(start, code),
		})
	}

	return result
}

func coveredBy(at int, spans []diff.Span) bool {
	for _, span := range spans {
		if span.Start <= at && at < span.End {
			return true
		}
	}

	return false
}

func kindAt(at int, spans []syntax.Span) syntax.Kind {
	for _, span := range spans {
		if span.Start <= at && at < span.End {
			return span.Kind
		}
	}

	return ""
}

// noNewline ставит пометку туда, где у строки начинается текст: под колонки
// номеров и знак.
func (m Model) noNewline(width, numberColumns int) string {
	numbersWidth := numberColumns * (m.numberWidth + 1)
	if m.hideNumbers {
		numbersWidth = 0
	}

	indent := strings.Repeat(" ", numbersWidth+2)
	return lipgloss.NewStyle().Foreground(m.colors.TextMuted).Render(fill(width, indent+m.text.Text("diff.noNewline", nil)))
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

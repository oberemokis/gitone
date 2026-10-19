package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// Под окном экран темнеет, как под диалогом opencode (#367): там на весь экран
// лежит подложка RGBA(0, 0, 0, 150). Прозрачности терминал не знает, поэтому
// цвет каждой клетки под окном смешивается с чёрным здесь.

// backdropKeep это доля цвета, которая остаётся под подложкой: 255-150 из 255.
const backdropKeep = 105

// terminalColors это цвета терминала по умолчанию, ими красятся клетки без
// своего цвета. nil, пока терминал не ответил на запрос из Init.
type terminalColors struct {
	fg, bg color.Color
}

// backdrop кладёт окно window в клетку x, y поверх затемнённого экрана base.
func backdrop(base, window string, x, y int, terminal terminalColors) string {
	canvas := lipgloss.NewCanvas(lipgloss.Width(base), lipgloss.Height(base))
	canvas.Compose(lipgloss.NewLayer(base))
	for row := range canvas.Height() {
		for column := range canvas.Width() {
			cell := canvas.CellAt(column, row)
			cell.Style = terminal.shade(cell.Style)
		}
	}

	return canvas.Compose(lipgloss.NewCompositor(lipgloss.NewLayer(window).X(x).Y(y))).Render()
}

// shade смешивает цвета клетки с чёрным подложки. Клетка без своего цвета
// берёт цвет терминала. Терминал не ответил: текст гаснет атрибутом faint, а
// фон остаётся, считать его чёрным нельзя: на светлом терминале пропал бы
// текст по умолчанию.
func (t terminalColors) shade(style uv.Style) uv.Style {
	fg, bg := style.Fg, style.Bg
	if fg == nil {
		fg = t.fg
	}
	if bg == nil {
		bg = t.bg
	}

	if fg == nil {
		style.Attrs |= uv.AttrFaint
	} else {
		style.Fg = darker(fg)
	}
	if bg != nil {
		style.Bg = darker(bg)
	}
	return style
}

// darker это цвет c под подложкой. ponytail: на 16 цветах терминал округлит
// результат до палитры, и тёмный текст может уйти в чёрный; под окном он не
// нужен, а точнее без своей палитры не сделать.
func darker(c color.Color) color.Color {
	r, g, b, _ := c.RGBA()
	keep := func(channel uint32) uint8 { return uint8((channel >> 8) * backdropKeep / 255) }
	return color.RGBA{R: keep(r), G: keep(g), B: keep(b), A: 0xff}
}

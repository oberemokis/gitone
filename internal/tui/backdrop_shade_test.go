package tui

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// Подложка смешивает цвета экрана под окном с чёрным на 150 из 255, клетка без
// цвета берёт цвет терминала, а без ответа терминала текст гаснет faint.
// Клетки окна остаются своими.
func TestBackdropShadesScreenUnderWindow(t *testing.T) {
	white := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#ff0000"))
	known := terminalColors{fg: lipgloss.Color("#ffffff"), bg: lipgloss.Color("#000000")}
	cases := map[string]struct {
		terminal terminalColors
		x        int
		fg, bg   color.Color
		faint    bool
	}{
		"свой цвет":                {x: 1, fg: lipgloss.Color("#696969"), bg: lipgloss.Color("#690000")},
		"цвет терминала известен":  {terminal: known, x: 2, fg: lipgloss.Color("#696969"), bg: lipgloss.Color("#000000")},
		"терминал не ответил":      {x: 2, faint: true},
		"окно поверх подложки":     {terminal: known, x: 0, fg: lipgloss.Color("#ffffff"), bg: lipgloss.Color("#ff0000")},
		"пустая клетка за строкой": {terminal: known, x: 4, fg: lipgloss.Color("#696969"), bg: lipgloss.Color("#000000")},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			screen := backdrop(white.Render("ab")+"cd\n"+"     ", white.Render("W"), 0, 0, tc.terminal)

			canvas := lipgloss.NewCanvas(lipgloss.Width(screen), lipgloss.Height(screen))
			canvas.Compose(lipgloss.NewLayer(screen))
			style := canvas.CellAt(tc.x, 0).Style
			if !sameColor(style.Fg, tc.fg) || !sameColor(style.Bg, tc.bg) {
				t.Fatalf("клетка %d: цвет %v на %v, want %v на %v", tc.x, style.Fg, style.Bg, tc.fg, tc.bg)
			}
			if faint := style.Attrs&uv.AttrFaint != 0; faint != tc.faint {
				t.Fatalf("клетка %d: faint %v, want %v", tc.x, faint, tc.faint)
			}
		})
	}
}

func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	return ar == br && ag == bg && ab == bb
}

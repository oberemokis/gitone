// Package theme держит цвета интерфейса по ролям. Имена полей совпадают с
// ключами тем из docs/config.md, чтобы загрузка themes/*.json легла на них
// без переименований.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

type Theme struct {
	TextMuted       color.Color
	BackgroundPanel color.Color
	Accent          color.Color
	DiffAdded       color.Color
	DiffRemoved     color.Color
	DiffAddedBg     color.Color
	DiffRemovedBg   color.Color
}

// ponytail: тема зашита в код, из themes/*.json она начнёт читаться в #15.
func Dark() Theme {
	return Theme{
		TextMuted:       lipgloss.Color("8"),
		BackgroundPanel: lipgloss.Color("#141414"),
		Accent:          lipgloss.Color("#9d7cd8"),
		DiffAdded:       lipgloss.Color("#3fb950"),
		DiffRemoved:     lipgloss.Color("#f85149"),
		DiffAddedBg:     lipgloss.Color("#12261e"),
		DiffRemovedBg:   lipgloss.Color("#2d1214"),
	}
}

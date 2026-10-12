// Package theme держит цвета интерфейса по ролям. Имена полей совпадают с
// ключами тем из docs/config.md, поэтому файл темы ложится на них без
// переименований.
package theme

import (
	"fmt"
	"image/color"
	"maps"
	"regexp"
	"slices"
	"strconv"

	"charm.land/lipgloss/v2"

	"gitone/internal/config"
)

type Theme struct {
	TextMuted         color.Color
	BackgroundPanel   color.Color
	Primary           color.Color
	Accent            color.Color
	DiffAdded         color.Color
	DiffRemoved       color.Color
	DiffAddedBg       color.Color
	DiffRemovedBg     color.Color
	DiffAddedWordBg   color.Color
	DiffRemovedWordBg color.Color
	SyntaxKeyword     color.Color
	SyntaxString      color.Color
	SyntaxComment     color.Color
	SyntaxNumber      color.Color
	Error             color.Color
}

// Dark встроенная тема. Файлы тем накладываются на неё, поэтому ключ, которого
// в файле нет, остаётся таким, как здесь.
func Dark() Theme {
	return Theme{
		TextMuted:         lipgloss.Color("8"),
		BackgroundPanel:   lipgloss.Color("#141414"),
		Primary:           lipgloss.Color("#fab283"),
		Accent:            lipgloss.Color("#9d7cd8"),
		DiffAdded:         lipgloss.Color("#3fb950"),
		DiffRemoved:       lipgloss.Color("#f85149"),
		DiffAddedBg:       lipgloss.Color("#12261e"),
		DiffRemovedBg:     lipgloss.Color("#2d1214"),
		DiffAddedWordBg:   lipgloss.Color("#1c4a2f"),
		DiffRemovedWordBg: lipgloss.Color("#5a1d22"),
		SyntaxKeyword:     lipgloss.Color("#ff7b72"),
		SyntaxString:      lipgloss.Color("#a5d6ff"),
		SyntaxComment:     lipgloss.Color("#6e7681"),
		SyntaxNumber:      lipgloss.Color("#79c0ff"),
		Error:             lipgloss.Color("#f85149"),
	}
}

// ANSI-палитра занимает номера 0-255.
const ansiLimit = 255

var (
	hexColor  = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	ansiColor = regexp.MustCompile(`^[0-9]{1,3}$`)
)

// Load возвращает тему по имени из config.json. dark встроена и файла не
// требует. Остальные имена читаются из themes/<имя>.json, без файла ошибка.
func Load(dir, name string) (Theme, error) {
	file, found, err := config.ReadTheme(dir, name)
	if err != nil {
		return Dark(), err
	}
	if !found {
		if name == "dark" {
			return Dark(), nil
		}
		return Dark(), fmt.Errorf("тема «%s» не найдена: нет themes/%s.json", name, name)
	}

	palette, err := Build(file)
	if err != nil {
		return Dark(), fmt.Errorf("themes/%s.json: %w", name, err)
	}

	return palette, nil
}

// Build накладывает описание темы на Dark. При ошибке возвращается Dark, чтобы
// вызывающий не получил наполовину собранную тему.
func Build(file config.ThemeFile) (Theme, error) {
	palette := Dark()
	fields := palette.fields()

	for _, key := range slices.Sorted(maps.Keys(file.Colors)) {
		field, known := fields[key]
		if !known {
			return Dark(), fmt.Errorf("colors.%s: ключ пока не используется", key)
		}

		value, err := resolve(file, file.Colors[key])
		if err != nil {
			return Dark(), fmt.Errorf("colors.%s: %w", key, err)
		}
		*field = value
	}

	return palette, nil
}

// fields связывает ключи файла с полями. Ключ без поля в коде пока не
// используется и дает ошибку, а не молча пропадает.
func (t *Theme) fields() map[string]*color.Color {
	return map[string]*color.Color{
		"textMuted":         &t.TextMuted,
		"backgroundPanel":   &t.BackgroundPanel,
		"primary":           &t.Primary,
		"accent":            &t.Accent,
		"diffAdded":         &t.DiffAdded,
		"diffRemoved":       &t.DiffRemoved,
		"diffAddedBg":       &t.DiffAddedBg,
		"diffRemovedBg":     &t.DiffRemovedBg,
		"diffAddedWordBg":   &t.DiffAddedWordBg,
		"diffRemovedWordBg": &t.DiffRemovedWordBg,
		"syntaxKeyword":     &t.SyntaxKeyword,
		"syntaxString":      &t.SyntaxString,
		"syntaxComment":     &t.SyntaxComment,
		"syntaxNumber":      &t.SyntaxNumber,
		"error":             &t.Error,
	}
}

// resolve переводит значение в цвет. Имя из defs заменяется его значением,
// один раз: имя внутри defs не раскрывается.
func resolve(file config.ThemeFile, value config.Color) (color.Color, error) {
	text := string(value)
	if named, ok := file.Defs[text]; ok {
		text = string(named)
	}

	switch {
	case text == "none":
		return lipgloss.NoColor{}, nil
	case hexColor.MatchString(text):
		return lipgloss.Color(text), nil
	case ansiColor.MatchString(text):
		if number, _ := strconv.Atoi(text); number <= ansiLimit {
			return lipgloss.Color(text), nil
		}
	}

	return nil, fmt.Errorf("«%s» не цвет: #rrggbb, число 0-255, имя из defs или none", value)
}

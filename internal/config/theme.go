package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"gitone/internal/i18n"
)

// ThemeFile это themes/<имя>.json как он записан: цвета строками, без проверки
// значений. Проверяет их пакет theme, потому что он знает роли.
type ThemeFile struct {
	Schema string           `json:"$schema"`
	Defs   map[string]Color `json:"defs"`
	Colors map[string]Color `json:"colors"`
}

// Color это значение цвета из файла: строка или число, число 8 и строка «8»
// дают одно значение.
type Color string

// ReadTheme читает themes/<name>.json. Файла нет, значит found false без
// ошибки. Имя темы из config.json не может выводить за каталог тем, как и имя
// раскладки.
func ReadTheme(dir, name string) (ThemeFile, bool, error) {
	data, found, err := readNamed(dir, "themes", name)
	if err != nil {
		return ThemeFile{}, false, err
	}
	if !found {
		return ThemeFile{}, false, nil
	}

	file, err := decodeTheme(data)
	if err != nil {
		return ThemeFile{}, false, fmt.Errorf("themes/%s.json: %w", name, err)
	}

	return file, true, nil
}

// themeJSON держит цвета сырыми: ошибку значения нужно назвать по ключу, а
// json.Unmarshaler такого ключа не знает.
type themeJSON struct {
	Schema string                     `json:"$schema"`
	Defs   map[string]json.RawMessage `json:"defs"`
	Colors map[string]json.RawMessage `json:"colors"`
}

func decodeTheme(data []byte) (ThemeFile, error) {
	var raw themeJSON
	if err := decodeStrict(data, &raw); err != nil {
		return ThemeFile{}, err
	}

	defs, err := colorsOf("defs", raw.Defs)
	if err != nil {
		return ThemeFile{}, err
	}
	colors, err := colorsOf("colors", raw.Colors)
	if err != nil {
		return ThemeFile{}, err
	}

	return ThemeFile{Schema: raw.Schema, Defs: defs, Colors: colors}, nil
}

func colorsOf(section string, raw map[string]json.RawMessage) (map[string]Color, error) {
	colors := make(map[string]Color, len(raw))
	for _, key := range slices.Sorted(maps.Keys(raw)) {
		var text string
		if json.Unmarshal(raw[key], &text) == nil {
			colors[key] = Color(text)
			continue
		}

		var number json.Number
		if json.Unmarshal(raw[key], &number) == nil {
			colors[key] = Color(number)
			continue
		}

		return nil, i18n.New("theme.colorType", i18n.Args{"section": section, "key": key})
	}

	return colors, nil
}

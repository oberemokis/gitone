package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
// ошибки. Имя темы из config.json не может выводить за каталог тем, поэтому
// путь, пустое имя и точка в начале отбрасываются.
func ReadTheme(dir, name string) (ThemeFile, bool, error) {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return ThemeFile{}, false, fmt.Errorf("тема «%s»: имя не может быть пустым, содержать путь или начинаться с точки", name)
	}

	data, err := os.ReadFile(filepath.Join(dir, "themes", name+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return ThemeFile{}, false, nil
	}
	if err != nil {
		return ThemeFile{}, false, fmt.Errorf("themes/%s.json: %w", name, err)
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
	if err := decodeStrict(data, &raw, "темы"); err != nil {
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

		return nil, fmt.Errorf("%s.%s: цвет должен быть строкой или числом", section, key)
	}

	return colors, nil
}

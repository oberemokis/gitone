package config

import (
	"encoding/json"
	"fmt"

	"gitone/internal/i18n"
)

// LayoutFile это layouts/<имя>.json как он записан. Проверку дерева делает
// пакет layout: он знает панели и правила размеров.
type LayoutFile struct {
	Schema string     `json:"$schema"`
	Root   LayoutNode `json:"root"`
}

// LayoutNode это узел дерева: контейнер с direction и children либо панель.
type LayoutNode struct {
	Direction string       `json:"direction"`
	Children  []LayoutNode `json:"children"`
	Pane      string       `json:"pane"`
	Size      Size         `json:"size"`
}

// Size это размер как он записан: «30%», число ячеек или пусто. Число 6 и
// строка «6» дают одно значение.
type Size string

func (s *Size) UnmarshalJSON(data []byte) error {
	var text string
	if json.Unmarshal(data, &text) == nil {
		*s = Size(text)
		return nil
	}

	var number json.Number
	if json.Unmarshal(data, &number) == nil {
		*s = Size(number)
		return nil
	}

	return i18n.New("layout.sizeType", nil)
}

// ReadLayout читает layouts/<name>.json. Файла нет, значит found false без
// ошибки. Имя не может содержать путь, как и имя темы.
func ReadLayout(dir, name string) (LayoutFile, bool, error) {
	data, found, err := readNamed(dir, "layouts", name)
	if err != nil {
		return LayoutFile{}, false, err
	}
	if !found {
		return LayoutFile{}, false, nil
	}

	file, err := decodeLayout(data)
	if err != nil {
		return LayoutFile{}, false, fmt.Errorf("layouts/%s.json: %w", name, err)
	}

	return file, true, nil
}

func decodeLayout(data []byte) (LayoutFile, error) {
	var file LayoutFile
	if err := decodeStrict(data, &file); err != nil {
		return LayoutFile{}, err
	}

	return file, nil
}

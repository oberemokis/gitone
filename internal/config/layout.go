package config

import (
	"encoding/json"
	"fmt"
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
	// Size это размер как он записан: «30%», число ячеек или пусто. Сырой он
	// потому, что json.Unmarshaler не знает путь узла, а ошибка типа должна его
	// назвать: разбирает размер layout через StringOrNumber.
	Size json.RawMessage `json:"size"`
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

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"gitone/internal/i18n"
)

const fileName = "config.json"

// Config это значения настроек. Поля без значения в файле остаются
// встроенными: Load начинает с Default и поверх читает файл.
type Config struct {
	Schema string `json:"$schema"`
	// язык интерфейса; пусто значит язык системы. Есть ли для него словарь,
	// проверяет i18n при выборе языка, а не config
	Language string `json:"language"`
	// имя темы: встроенная dark или файл themes/<имя>.json
	Theme string `json:"theme"`
	// имя раскладки: встроенная default или focus либо файл layouts/<имя>.json
	Layout string `json:"layout"`
	UI     UI     `json:"ui"`
	Diff   Diff   `json:"diff"`
}

type Diff struct {
	Mode              string `json:"mode"`
	AutoSplitMinWidth int    `json:"autoSplitMinWidth"`
	// Wrap переносит длинные строки, без него они прокручиваются вбок
	Wrap bool `json:"wrap"`
}

// UI выключает служебные элементы: false прячет элемент. Все включены по
// умолчанию, полоса прокрутки и строка статуса пока не принимаются.
type UI struct {
	KeyHints    bool `json:"keyHints"`
	PaneTitles  bool `json:"paneTitles"`
	LineNumbers bool `json:"lineNumbers"`
	// перечитывание при изменении файлов в рабочей копии
	AutoReload bool `json:"autoReload"`
}

func Default() Config {
	return Config{
		Theme:  "dark",
		Layout: "default",
		UI:     UI{KeyHints: true, PaneTitles: true, LineNumbers: true, AutoReload: true},
		Diff:   Diff{Mode: "auto", AutoSplitMinWidth: 120},
	}
}

// Load читает config.json из каталога. Файла нет, значит встроенные значения
// без ошибки. Файл с ошибкой отбрасывается целиком: лучше встроенные значения,
// чем половина пользовательских. Неизвестное поле тоже ошибка: опечатка в
// ключе иначе молча ничего не делает.
func Load(dir string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), fmt.Errorf("%s: %w", fileName, err)
	}

	cfg, err := decode(data)
	if err != nil {
		return Default(), fmt.Errorf("%s: %w", fileName, err)
	}

	return cfg, nil
}

func decode(data []byte) (Config, error) {
	cfg := Default()
	if err := decodeStrict(data, &cfg); err != nil {
		return Config{}, err
	}

	switch cfg.Diff.Mode {
	case "auto", "split", "unified":
	default:
		return Config{}, i18n.New("config.badMode", i18n.Args{"mode": cfg.Diff.Mode})
	}

	return cfg, nil
}

// decodeStrict читает ровно один объект JSON. Неизвестное поле и текст после
// объекта это ошибка: опечатка в ключе иначе молча ничего не делает. Какой
// файл, говорит префикс ошибки у вызывающего.
func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return i18n.New("json.extraText", nil)
	}

	return nil
}

// Dir возвращает каталог настроек: $XDG_CONFIG_HOME/gitone, а без переменной
// ~/.config/gitone. Переменные передаются снаружи, чтобы функция не читала
// окружение и проверялась таблицей.
func Dir(xdgConfigHome, home string) string {
	if xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "gitone")
	}

	return filepath.Join(home, ".config", "gitone")
}

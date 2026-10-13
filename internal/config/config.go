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
	"strings"

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
	// Wrap переносит длинные строки (по умолчанию), без него они
	// прокручиваются вбок
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
		Diff:   Diff{Mode: "auto", AutoSplitMinWidth: 120, Wrap: true},
	}
}

// Load читает config.json из каталога. Файла нет, значит встроенные значения
// без ошибки. Файл с ошибкой отбрасывается целиком: лучше встроенные значения,
// чем половина пользовательских. Неизвестное поле тоже ошибка: опечатка в
// ключе иначе молча ничего не делает.
func Load(dir string) (Config, error) {
	data, found, err := readFile(filepath.Join(dir, fileName))
	if err != nil {
		return Default(), fmt.Errorf("%s: %w", fileName, err)
	}
	if !found {
		return Default(), nil
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

	// Список режимов записан ещё в diffview (ModeAuto, ModeSplit, ModeUnified)
	// и в тексте config.badMode словаря. Импорт diffview граф запрещает, а без
	// проверки здесь ошибочный файл не отбрасывался бы целиком: новый режим
	// правится во всех трёх местах.
	switch cfg.Diff.Mode {
	case "auto", "split", "unified":
	default:
		return Config{}, i18n.New("config.badMode", i18n.Args{"mode": cfg.Diff.Mode})
	}

	return cfg, nil
}

// readFile читает файл настроек. found false без ошибки значит, что файла нет.
// ErrNotExist приходит и от битой символической ссылки (dotfiles через stow,
// перенесённый репозиторий), но тогда настройки есть и не читаются: молча
// подменять их встроенными нельзя. Ссылкой может быть сам файл или каталог, в
// котором он лежит: Lstat файла в каталоге-ссылке тоже отвечает ErrNotExist.
func readFile(path string) (data []byte, found bool, err error) {
	data, err = os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) && !dangling(path) && !dangling(filepath.Dir(path)) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	return data, true, nil
}

// dangling говорит, что путь существует как символическая ссылка, но ведёт в
// никуда: Lstat видит ссылку, Stat уже нет.
func dangling(path string) bool {
	_, linkErr := os.Lstat(path)
	_, targetErr := os.Stat(path)

	return linkErr == nil && errors.Is(targetErr, fs.ErrNotExist)
}

// readNamed читает <sub>/<name>.json из каталога настроек, например тему или
// раскладку. Имя приходит из config.json и не должно выводить за каталог sub,
// поэтому пустое имя, разделители пути и точка в начале отбрасываются.
func readNamed(dir, sub, name string) (data []byte, found bool, err error) {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return nil, false, i18n.New("config.badName", i18n.Args{"sub": sub, "name": name})
	}

	data, found, err = readFile(filepath.Join(dir, sub, name+".json"))
	if err != nil {
		return nil, false, fmt.Errorf("%s/%s.json: %w", sub, name, err)
	}

	return data, found, nil
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

// StringOrNumber читает значение, которое в файле пишут строкой или числом:
// число 8 и строка «8» дают одно значение. ok false значит другой тип JSON
// или пустое значение. Ошибку по ней строит вызывающий, ему известен путь
// узла, а json.Unmarshaler его не знает.
func StringOrNumber(raw json.RawMessage) (text string, ok bool) {
	if json.Unmarshal(raw, &text) == nil {
		return text, true
	}

	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return number.String(), true
	}

	return "", false
}

// Dir возвращает каталог настроек: $XDG_CONFIG_HOME/gitone, а без переменной
// ~/.config/gitone. Относительный $XDG_CONFIG_HOME (чаще нераскрытая тильда из
// launchctl или environment.d) игнорируется, как велит спецификация XDG: его
// путь считался бы от просматриваемого репозитория, и репозиторий подставлял
// бы свои настройки. Переменные передаются снаружи, чтобы функция не читала
// окружение и проверялась таблицей.
func Dir(xdgConfigHome, home string) string {
	if filepath.IsAbs(xdgConfigHome) {
		return filepath.Join(xdgConfigHome, "gitone")
	}

	return filepath.Join(home, ".config", "gitone")
}

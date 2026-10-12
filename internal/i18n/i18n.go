// Package i18n держит тексты интерфейса на разных языках. Словари это
// languages/<язык>.json, вшитые в бинарник: ключ строки и шаблон с
// подстановками вида {name}. Английский словарь полный, остальные его
// переводят; строки, которой в словаре нет, берётся из английского.
//
// Пакет ничего не импортирует из проекта: им пользуются и листья для своих
// ошибок, и край, который собирает текст на выбранном языке при показе.
package i18n

import (
	"cmp"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"sync"
)

const english = "en"

//go:embed languages/*.json
var files embed.FS

// catalog это словари по языкам: язык → ключ → шаблон.
type catalog map[string]map[string]string

// builtin разбирает встроенные словари один раз, дальше они только читаются,
// как регулярные выражения в diff и layout.
var builtin = sync.OnceValue(func() catalog { return parse(files) })

// parse паникует на битом файле: словари вшиты в бинарник, и такой файл это
// ошибка сборки, а не окружения пользователя. Тест разбирает каждый словарь.
func parse(fsys fs.FS) catalog {
	names, err := fs.Glob(fsys, "languages/*.json")
	if err != nil {
		panic(err)
	}

	languages := make(catalog, len(names))
	for _, name := range names {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			panic(err)
		}

		var texts map[string]string
		if err := json.Unmarshal(data, &texts); err != nil {
			panic(fmt.Sprintf("i18n: %s: %v", name, err))
		}
		languages[strings.TrimSuffix(path.Base(name), ".json")] = texts
	}

	return languages
}

// Args это значения подстановок по имени: {"pane": "diff"} для {pane}.
type Args map[string]any

// Dictionary это тексты одного языка. Нулевое значение английское.
type Dictionary struct {
	lang  string
	texts map[string]string
}

// Keys это ключи английского словаря по порядку, то есть все строки.
func Keys() []string {
	return slices.Sorted(maps.Keys(builtin()[english]))
}

// Custom собирает словарь не из встроенных файлов: для тестов и псевдоязыка.
func Custom(lang string, texts map[string]string) Dictionary {
	return Dictionary{lang: lang, texts: texts}
}

func (d Dictionary) Lang() string {
	return cmp.Or(d.lang, english)
}

// Text отдаёт строку по ключу с подстановками. Строки нет в словаре языка,
// значит английская; нет и в английском, значит сам ключ: так пропуск виден
// на экране, а не прячется пустым местом.
func (d Dictionary) Text(key string, args Args) string {
	template, found := d.texts[key]
	if !found {
		template, found = builtin()[english][key]
	}
	if !found {
		return key
	}

	return fill(template, args)
}

// fill подставляет значения за один проход: подставленное значение заново не
// разбирается, и результат не зависит от порядка обхода map.
func fill(template string, args Args) string {
	if len(args) == 0 {
		return template
	}

	pairs := make([]string, 0, 2*len(args))
	for name, value := range args {
		pairs = append(pairs, "{"+name+"}", fmt.Sprint(value))
	}

	return strings.NewReplacer(pairs...).Replace(template)
}

// Error это ошибка, текст которой собирается на языке пользователя при
// показе. Сама по себе она английская: так её видят логи и тесты.
type Error struct {
	Key  string
	Args Args
}

func New(key string, args Args) error {
	return &Error{Key: key, Args: args}
}

func (e *Error) Error() string {
	return Dictionary{}.Text(e.Key, e.Args)
}

// Error собирает текст ошибки на языке словаря. Обёртка вида «префикс: %w»
// оставляет префикс (имя файла, путь, операцию git), а внутренняя ошибка
// переводится. Чужие ошибки (git, json, os) и обёртки, где внутренняя ошибка
// стоит не в конце, идут как есть.
func (d Dictionary) Error(err error) string {
	if own, ok := err.(*Error); ok {
		return d.Text(own.Key, own.Args)
	}

	inner := errors.Unwrap(err)
	if inner == nil {
		return err.Error()
	}

	prefix, found := strings.CutSuffix(err.Error(), inner.Error())
	if !found {
		return err.Error()
	}

	return prefix + d.Error(inner)
}

// Choose выбирает словарь: язык из настроек, иначе язык системы, иначе
// английский. env это LC_ALL, LC_MESSAGES и LANG по порядку.
func Choose(configured string, env ...string) (Dictionary, error) {
	return choose(builtin(), configured, env)
}

// choose: язык из настроек без словаря даёт английский и ошибку, его просили
// явно. Язык системы без словаря молча даёт английский. Решает первая
// непустая переменная, как в POSIX: LC_ALL перекрывает LANG и тогда, когда
// словаря для неё нет.
func choose(languages catalog, configured string, env []string) (Dictionary, error) {
	if configured != "" {
		texts, found := languages[configured]
		if !found {
			available := strings.Join(slices.Sorted(maps.Keys(languages)), ", ")
			return Dictionary{}, New("language.unknown", Args{"language": configured, "available": available})
		}
		return Dictionary{lang: configured, texts: texts}, nil
	}

	for _, locale := range env {
		if locale == "" {
			continue
		}

		lang := languageOf(locale)
		if texts, found := languages[lang]; found {
			return Dictionary{lang: lang, texts: texts}, nil
		}
		return Dictionary{}, nil
	}

	return Dictionary{}, nil
}

// languageOf берёт язык из локали: ru_RU.UTF-8, ru.UTF-8 и ru@latin дают ru.
func languageOf(locale string) string {
	if cut := strings.IndexAny(locale, "_.@"); cut >= 0 {
		return locale[:cut]
	}

	return locale
}

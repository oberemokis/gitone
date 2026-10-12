package i18n_test

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"gitone/internal/i18n"
)

func TestText(t *testing.T) {
	custom := i18n.Custom("xx", map[string]string{
		"greeting": "hello {name}, {name}",
		"order":    "{second} before {first}",
	})

	tests := []struct {
		name       string
		dictionary i18n.Dictionary
		key        string
		args       i18n.Args
		want       string
	}{
		{name: "подстановка во всех местах", dictionary: custom, key: "greeting", args: i18n.Args{"name": "Ann"}, want: "hello Ann, Ann"},
		{name: "порядок задаёт шаблон", dictionary: custom, key: "order", args: i18n.Args{"first": 1, "second": 2}, want: "2 before 1"},
		{name: "значение с именем подстановки не подставляется второй раз", dictionary: custom, key: "order", args: i18n.Args{"first": "{second}", "second": "x"}, want: "x before {second}"},
		{name: "неизвестная подстановка остаётся", dictionary: custom, key: "greeting", args: nil, want: "hello {name}, {name}"},
		{name: "нет строки в словаре: английский", dictionary: custom, key: "screen.noChanges", want: "No changes"},
		{name: "нулевой словарь английский", dictionary: i18n.Dictionary{}, key: "screen.noChanges", want: "No changes"},
		{name: "нет строки нигде: сам ключ", dictionary: custom, key: "missing.key", want: "missing.key"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.dictionary.Text(tc.key, tc.args); got != tc.want {
				t.Errorf("Text(%q, %v) = %q, want %q", tc.key, tc.args, got, tc.want)
			}
		})
	}
}

func TestError(t *testing.T) {
	custom := i18n.Custom("xx", map[string]string{
		"pane.twice":       "панель {pane} повторяется",
		"screen.noChanges": "изменений нет",
	})
	twice := i18n.New("pane.twice", i18n.Args{"pane": "diff"})
	// текст обёртки идёт до внутренней ошибки, префикс от неё не отделить
	notAtEnd := fmt.Errorf("%w (diff)", i18n.New("screen.noChanges", nil))

	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "своя ошибка на языке словаря", err: twice, want: "панель diff повторяется"},
		{name: "префикс обёртки остаётся", err: fmt.Errorf("layouts/x.json: %w", fmt.Errorf("root.children[1]: %w", twice)), want: "layouts/x.json: root.children[1]: панель diff повторяется"},
		{name: "чужая ошибка как есть", err: fs.ErrNotExist, want: "file does not exist"},
		{name: "обёртка чужой ошибки как есть", err: fmt.Errorf("diff: %w", errors.New("fatal: not a git repository")), want: "diff: fatal: not a git repository"},
		{name: "обёртка не в конце по-английски", err: notAtEnd, want: "No changes (diff)"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := custom.Error(tc.err); got != tc.want {
				t.Errorf("Error(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestErrorKeepsIdentity(t *testing.T) {
	sentinel := i18n.New("update.dev", nil)
	wrapped := fmt.Errorf("update: %w", sentinel)

	if !errors.Is(wrapped, sentinel) {
		t.Errorf("errors.Is(%v, sentinel) = false, want true", wrapped)
	}
	if errors.Is(wrapped, i18n.New("update.dev", nil)) {
		t.Errorf("errors.Is(%v, другая ошибка с тем же ключом) = true, want false", wrapped)
	}
}

func TestChoose(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		env        []string
		wantLang   string
		wantErr    bool
	}{
		{name: "ничего не задано", wantLang: "en"},
		{name: "язык из настроек", configured: "en", env: []string{"de_DE.UTF-8"}, wantLang: "en"},
		{name: "язык из настроек без словаря", configured: "de", wantLang: "en", wantErr: true},
		{name: "язык системы без словаря", env: []string{"", "", "de_DE.UTF-8"}, wantLang: "en"},
		{name: "язык системы со словарём", env: []string{"", "", "en_GB.UTF-8"}, wantLang: "en"},
		{name: "локаль C", env: []string{"C"}, wantLang: "en"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dictionary, err := i18n.Choose(tc.configured, tc.env...)

			if got := dictionary.Lang(); got != tc.wantLang {
				t.Errorf("Choose(%q, %q).Lang() = %q, want %q", tc.configured, tc.env, got, tc.wantLang)
			}
			if (err != nil) != tc.wantErr {
				t.Errorf("Choose(%q, %q) error = %v, want error %v", tc.configured, tc.env, err, tc.wantErr)
			}
		})
	}
}

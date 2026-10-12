package i18n

import (
	"maps"
	"regexp"
	"slices"
	"testing"
)

// Тесты внутри пакета: второго встроенного языка пока нет, поэтому выбор языка
// проверяется на своём каталоге, а полнота читает встроенные словари напрямую.

func TestChooseWithSecondLanguage(t *testing.T) {
	languages := catalog{
		"en": {"hello": "hello"},
		"ru": {"hello": "привет"},
	}

	tests := []struct {
		name       string
		configured string
		env        []string
		wantLang   string
	}{
		{name: "из LANG", env: []string{"", "", "ru_RU.UTF-8"}, wantLang: "ru"},
		{name: "без страны", env: []string{"", "", "ru.UTF-8"}, wantLang: "ru"},
		{name: "с вариантом", env: []string{"", "", "ru@latin"}, wantLang: "ru"},
		{name: "LC_ALL перекрывает LANG", env: []string{"en_US.UTF-8", "", "ru_RU.UTF-8"}, wantLang: "en"},
		{name: "LC_ALL без словаря перекрывает LANG", env: []string{"de_DE.UTF-8", "", "ru_RU.UTF-8"}, wantLang: "en"},
		{name: "настройки перекрывают систему", configured: "en", env: []string{"ru_RU.UTF-8"}, wantLang: "en"},
		{name: "язык из настроек", configured: "ru", wantLang: "ru"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dictionary, err := choose(languages, tc.configured, tc.env)
			if err != nil {
				t.Fatalf("choose(%q, %q) error = %v", tc.configured, tc.env, err)
			}

			if got := dictionary.Lang(); got != tc.wantLang {
				t.Errorf("choose(%q, %q).Lang() = %q, want %q", tc.configured, tc.env, got, tc.wantLang)
			}
			if got, want := dictionary.Text("hello", nil), languages[tc.wantLang]["hello"]; got != want {
				t.Errorf("choose(%q, %q).Text(hello) = %q, want %q", tc.configured, tc.env, got, want)
			}
		})
	}
}

var placeholder = regexp.MustCompile(`\{[A-Za-z]+\}`)

// У каждого встроенного языка те же ключи и те же подстановки, что у
// английского: пропущенная строка ушла бы на экран по-английски, а потерянная
// подстановка молча выбросила бы значение.
func TestDictionariesMatchEnglish(t *testing.T) {
	languages := builtin()
	english := languages["en"]
	if len(english) == 0 {
		t.Fatal("английский словарь пуст или не найден")
	}

	for lang, texts := range languages {
		if lang == "en" {
			continue
		}

		if got, want := slices.Sorted(maps.Keys(texts)), slices.Sorted(maps.Keys(english)); !slices.Equal(got, want) {
			t.Errorf("ключи %s.json = %q, want %q", lang, got, want)
		}
		for key, template := range english {
			got := slices.Sorted(slices.Values(placeholder.FindAllString(texts[key], -1)))
			want := slices.Sorted(slices.Values(placeholder.FindAllString(template, -1)))
			if !slices.Equal(got, want) {
				t.Errorf("подстановки %s.json[%q] = %q, want %q", lang, key, got, want)
			}
		}
	}
}

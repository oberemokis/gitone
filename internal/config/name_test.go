package config_test

import (
	"strings"
	"testing"

	"gitone/internal/config"
)

// Имя темы и раскладки приходит из config.json и не должно выводить за каталог
// настроек, поэтому обе функции отвергают одни и те же имена. Случаи с точкой в
// начале и «../» ловит и правило про точку, а «x/../../outside» только запрет
// разделителей.
func TestNameOutsideDirIsRejected(t *testing.T) {
	readers := []struct {
		name string
		read func(dir, name string) error
	}{
		{name: "тема", read: func(dir, name string) error {
			_, _, err := config.ReadTheme(dir, name)
			return err
		}},
		{name: "раскладка", read: func(dir, name string) error {
			_, _, err := config.ReadLayout(dir, name)
			return err
		}},
	}
	names := []struct{ name, value string }{
		{name: "подкаталог", value: "sub/x"},
		{name: "обратная косая", value: `a\b`},
		{name: "пустое имя", value: ""},
		{name: "на каталог выше", value: "../compact"},
		{name: "точка в начале", value: ".hidden"},
		{name: "выход из каталога через подкаталог", value: "x/../../outside"},
	}

	for _, reader := range readers {
		for _, tc := range names {
			t.Run(reader.name+"/"+tc.name, func(t *testing.T) {
				err := reader.read(t.TempDir(), tc.value)

				if err == nil || !strings.Contains(err.Error(), "contain a path") {
					t.Errorf("%s с именем %q: error = %v, want error with %q", reader.name, tc.value, err, "contain a path")
				}
			})
		}
	}
}

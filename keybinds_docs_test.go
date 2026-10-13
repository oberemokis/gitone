package gitone

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gitone/internal/keymap"
)

// actionRow это строка таблицы действий: в первой ячейке имена в обратных
// кавычках через запятую.
var actionRow = regexp.MustCompile("(?m)^\\| (`[^`|]+`(?:, `[^`|]+`)*) \\|")

// Справка по ? показывает подпись и клавиши, а имя действия для keybinds.json
// не показывает: угаданное имя роняет весь файл. Поэтому docs/config.md
// перечисляет все действия, глобальные и окон, и тест сверяет таблицы с
// keymap.Default.
func TestConfigDocListsEveryAction(t *testing.T) {
	data, err := os.ReadFile("docs/config.md")
	if err != nil {
		t.Fatal(err)
	}

	var documented []string
	for _, row := range actionRow.FindAllStringSubmatch(string(data), -1) {
		documented = append(documented, strings.Split(strings.ReplaceAll(row[1], "`", ""), ", ")...)
	}

	var coded []string
	for _, binding := range keymap.Default().Bindings() {
		coded = append(coded, string(binding.Action))
	}

	missing := slices.DeleteFunc(slices.Clone(coded), func(name string) bool { return slices.Contains(documented, name) })
	unknown := slices.DeleteFunc(slices.Clone(documented), func(name string) bool { return slices.Contains(coded, name) })

	if len(missing)+len(unknown) > 0 {
		t.Errorf("docs/config.md расходится с keymap.Default\nнет в таблицах: %q\nнет в коде: %q", missing, unknown)
	}
}

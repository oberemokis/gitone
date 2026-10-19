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
// кавычках через запятую, во второй клавиши по умолчанию.
var actionRow = regexp.MustCompile("(?m)^\\| (`[^`|]+`(?:, `[^`|]+`)*) \\| ([^|]*) \\|")

// defaultKeys разбирает ячейку клавиш по умолчанию для count действий: клавиши
// одного действия идут через запятую, действия разделены «и»; у нескольких
// действий по одной клавише они идут просто через запятую.
func defaultKeys(cell string, count int) [][]string {
	keys := func(part string) []string {
		return strings.Split(strings.ReplaceAll(part, "`", ""), ", ")
	}

	parts := strings.Split(cell, " и ")
	if len(parts) == count {
		var result [][]string
		for _, part := range parts {
			result = append(result, keys(part))
		}
		return result
	}

	var result [][]string
	for _, key := range keys(cell) {
		result = append(result, []string{key})
	}
	return result
}

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

// Клавиши по умолчанию в таблицах docs/config.md те же и в том же порядке,
// что в keymap.Default: таблица единственное место, где их видно до запуска.
func TestConfigDocDefaultKeys(t *testing.T) {
	data, err := os.ReadFile("docs/config.md")
	if err != nil {
		t.Fatal(err)
	}

	defaults := keymap.Default()
	for _, row := range actionRow.FindAllStringSubmatch(string(data), -1) {
		actions := strings.Split(strings.ReplaceAll(row[1], "`", ""), ", ")
		documented := defaultKeys(row[2], len(actions))
		if len(documented) != len(actions) {
			t.Errorf("%s: в ячейке %q не разобрать клавиши на %d действий", row[1], row[2], len(actions))
			continue
		}

		for i, action := range actions {
			if coded := defaults.KeysFor(keymap.Action(action)); !slices.Equal(documented[i], coded) {
				t.Errorf("%s: в docs/config.md %q, в keymap.Default %q", action, documented[i], coded)
			}
		}
	}
}

// Package keymap переводит клавиши в действия. Встроенный список действий и
// клавиш лежит здесь, keybinds.json его переназначает. Панели получают
// действие, а не клавишу, поэтому правка файла не трогает их код.
package keymap

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const fileName = "keybinds.json"

type Action string

const (
	AppQuit          Action = "app.quit"
	AppHelp          Action = "app.help"
	FilesPick        Action = "files.pick"
	FilesNext        Action = "files.next"
	FilesPrevious    Action = "files.previous"
	FilesStage       Action = "files.stage"
	FilesDiscard     Action = "files.discard"
	FilesCommit      Action = "files.commit"
	AppLog           Action = "app.log"
	DiffWorkingTree  Action = "diff.workingTree"
	AppBranches      Action = "app.branches"
	StashPush        Action = "stash.push"
	StashList        Action = "stash.list"
	HelpClose        Action = "help.close"
	DiffToggleMode   Action = "diff.toggleMode"
	DiffReload       Action = "diff.reload"
	DiffScrollDown   Action = "diff.scrollDown"
	DiffScrollUp     Action = "diff.scrollUp"
	DiffHalfPageDown Action = "diff.halfPageDown"
	DiffHalfPageUp   Action = "diff.halfPageUp"
	DiffTop          Action = "diff.top"
	DiffBottom       Action = "diff.bottom"
)

// Заголовок группы в справке. Порядок групп задаёт первое появление
// категории в списке, подряд клавиши одной категории стоять не обязаны.
type Category string

const (
	CategoryGeneral Category = "General"
	CategoryFiles   Category = "Files"
	CategoryDiff    Category = "Diff"
)

type Binding struct {
	Action   Action
	Category Category
	// как их возвращает KeyPressMsg.String(), пустой список значит действие отключено
	Keys []string
	// подпись в подвале, пустая значит в подвал не выводить
	Label string
	// строка в справке, пустая значит в справку не выводить
	Description string
}

// Keymap это список действий с клавишами. Нулевое значение пустое: клавиш
// нет, поэтому корень получает его из Default или Load.
type Keymap struct {
	bindings []Binding
}

func (k Keymap) Bindings() []Binding {
	return k.bindings
}

func (k Keymap) KeysFor(action Action) []string {
	for _, binding := range k.bindings {
		if binding.Action == action {
			return binding.Keys
		}
	}

	return nil
}

func (k Keymap) ActionFor(key string) Action {
	for _, binding := range k.bindings {
		if slices.Contains(binding.Keys, key) {
			return binding.Action
		}
	}

	return ""
}

func Default() Keymap {
	return Keymap{bindings: defaults()}
}

// Load читает keybinds.json из каталога. Файла нет, значит встроенные клавиши.
// Файл с ошибкой отбрасывается целиком, как config.json.
func Load(dir string) (Keymap, error) {
	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), fmt.Errorf("%s: %w", fileName, err)
	}

	km, err := decode(data)
	if err != nil {
		return Default(), fmt.Errorf("%s: %w", fileName, err)
	}

	return km, nil
}

type file struct {
	Schema   string                     `json:"$schema"`
	Keybinds map[string]json.RawMessage `json:"keybinds"`
}

func decode(data []byte) (Keymap, error) {
	var f file

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&f); err != nil {
		return Keymap{}, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return Keymap{}, errors.New("лишний текст после клавиш")
	}

	bindings := defaults()
	// имена идут по порядку, чтобы ошибка при нескольких проблемах не
	// менялась от запуска к запуску
	for _, name := range slices.Sorted(maps.Keys(f.Keybinds)) {
		index := slices.IndexFunc(bindings, func(b Binding) bool { return string(b.Action) == name })
		if index < 0 {
			return Keymap{}, fmt.Errorf("неизвестное действие «%s»", name)
		}

		keys, err := parseKeys(f.Keybinds[name])
		if err != nil {
			return Keymap{}, fmt.Errorf("%s: %w", name, err)
		}
		// переназначение заменяет клавиши действия целиком, не добавляет к ним
		bindings[index].Keys = keys
	}

	if err := checkConflicts(bindings); err != nil {
		return Keymap{}, err
	}

	return Keymap{bindings: bindings}, nil
}

// parseKeys принимает строку, массив строк, «none» или false. Оба значения
// отключают действие.
func parseKeys(raw json.RawMessage) ([]string, error) {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		if one == "none" {
			return nil, nil
		}
		return checkKeys([]string{one})
	}

	var many []string
	if json.Unmarshal(raw, &many) == nil {
		return checkKeys(many)
	}

	var off bool
	if json.Unmarshal(raw, &off) == nil && !off {
		return nil, nil
	}

	return nil, errors.New("ожидалась клавиша, массив клавиш, «none» или false")
}

func checkKeys(keys []string) ([]string, error) {
	for _, key := range keys {
		if key == "" {
			return nil, errors.New("пустая клавиша")
		}
		if strings.HasPrefix(key, "<leader>") {
			return nil, errors.New("<leader> пока не поддерживается")
		}
	}

	return keys, nil
}

// checkConflicts ищет клавишу, назначенную двум действиям. Проверка идёт по
// всему списку, а не по области: фокуса панелей пока нет, все действия
// работают одновременно.
func checkConflicts(bindings []Binding) error {
	owner := make(map[string]Action)
	for _, binding := range bindings {
		for _, key := range binding.Keys {
			if other, taken := owner[key]; taken {
				return fmt.Errorf("клавиша «%s» назначена на %s и %s", key, other, binding.Action)
			}
			owner[key] = binding.Action
		}
	}

	return nil
}

func defaults() []Binding {
	return []Binding{
		{Action: AppQuit, Category: CategoryGeneral, Keys: []string{"q", "ctrl+c"}, Label: "quit", Description: "Quit"},
		{Action: FilesPick, Category: CategoryFiles, Keys: []string{"ctrl+p"}, Label: "files", Description: "File list"},
		{Action: AppHelp, Category: CategoryGeneral, Keys: []string{"?"}, Label: "help", Description: "Toggle this help"},
		{Action: FilesNext, Category: CategoryFiles, Keys: []string{"]"}, Description: "Next file"},
		{Action: FilesPrevious, Category: CategoryFiles, Keys: []string{"["}, Description: "Previous file"},
		{Action: FilesStage, Category: CategoryFiles, Keys: []string{"space"}, Description: "Stage or unstage file"},
		{Action: FilesDiscard, Category: CategoryFiles, Keys: []string{"x"}, Description: "Discard file changes"},
		{Action: FilesCommit, Category: CategoryFiles, Keys: []string{"c"}, Description: "Commit staged changes"},
		{Action: AppLog, Category: CategoryDiff, Keys: []string{"l"}, Description: "Commit log"},
		{Action: DiffWorkingTree, Category: CategoryDiff, Keys: []string{"b"}, Description: "Working tree diff"},
		{Action: AppBranches, Category: CategoryGeneral, Keys: []string{"B"}, Description: "Branches"},
		{Action: StashPush, Category: CategoryFiles, Keys: []string{"z"}, Description: "Save changes to stash"},
		{Action: StashList, Category: CategoryFiles, Keys: []string{"Z"}, Description: "Stash list"},
		{Action: DiffScrollDown, Category: CategoryDiff, Keys: []string{"j", "down"}, Description: "Scroll down"},
		{Action: DiffScrollUp, Category: CategoryDiff, Keys: []string{"k", "up"}, Description: "Scroll up"},
		{Action: DiffHalfPageDown, Category: CategoryDiff, Keys: []string{"ctrl+d"}, Description: "Half page down"},
		{Action: DiffHalfPageUp, Category: CategoryDiff, Keys: []string{"ctrl+u"}, Description: "Half page up"},
		{Action: DiffTop, Category: CategoryDiff, Keys: []string{"g"}, Description: "Top"},
		{Action: DiffBottom, Category: CategoryDiff, Keys: []string{"G"}, Description: "Bottom"},
		{Action: DiffToggleMode, Category: CategoryDiff, Keys: []string{"m"}, Description: "Split or unified"},
		{Action: DiffReload, Category: CategoryDiff, Keys: []string{"r"}, Description: "Reload diff"},
		// в справке стоит в заголовке, как esc в окнах opencode
		{Action: HelpClose, Keys: []string{"esc"}},
	}
}

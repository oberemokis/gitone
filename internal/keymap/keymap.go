// Package keymap переводит клавиши в действия. Встроенный список действий и
// клавиш лежит здесь, keybinds.json его переназначает. Корень работает с
// действием, а не с клавишей, поэтому правка файла не трогает код.
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
	"unicode/utf8"

	"gitone/internal/i18n"
)

const fileName = "keybinds.json"

type Action string

const (
	AppQuit          Action = "app.quit"
	AppHelp          Action = "app.help"
	FilesPick        Action = "files.pick"
	FilesToggle      Action = "files.toggle"
	FilesNext        Action = "files.next"
	FilesPrevious    Action = "files.previous"
	FilesStage       Action = "files.stage"
	FilesDiscard     Action = "files.discard"
	FilesCommit      Action = "files.commit"
	AppLog           Action = "app.log"
	DiffWorkingTree  Action = "diff.workingTree"
	AppBranches      Action = "app.branches"
	AppWorktrees     Action = "app.worktrees"
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
	DiffToggleWrap   Action = "diff.toggleWrap"
	DiffScrollLeft   Action = "diff.scrollLeft"
	DiffScrollRight  Action = "diff.scrollRight"
)

// Действия окон. Окно забирает нажатия целиком, поэтому эти действия ищутся
// только в своём окне, а глобальные внутри окна не действуют.
const (
	ListUp          Action = "list.up"
	ListDown        Action = "list.down"
	ListClose       Action = "list.close"
	LogOpen         Action = "log.open"
	BranchesSwitch  Action = "branches.switch"
	WorktreesSwitch Action = "worktrees.switch"
	StashDiff       Action = "stash.diff"
	StashApply      Action = "stash.apply"
	StashDrop       Action = "stash.drop"
	QuestionYes     Action = "question.yes"
	SearchChoose    Action = "search.choose"
	SearchClose     Action = "search.close"
	SearchUp        Action = "search.up"
	SearchDown      Action = "search.down"
	CommitSubmit    Action = "commit.submit"
	CommitClose     Action = "commit.close"
	// поиск в логе: / открывает поле, done оставляет фильтр, cancel снимает
	LogSearch       Action = "log.search"
	LogSearchDone   Action = "logSearch.done"
	LogSearchCancel Action = "logSearch.cancel"
)

// Группа в справке. Порядок групп задаёт первое появление категории в
// списке, подряд клавиши одной категории стоять не обязаны. Заголовок группы
// берётся из словаря по ключу category.<категория>.
type Category string

const (
	CategoryGeneral Category = "general"
	CategoryFiles   Category = "files"
	CategoryDiff    Category = "diff"
)

// Binding это действие и его клавиши. Тексты подсказок здесь не лежат: подпись
// в подвале и строка в справке берутся из словаря по ключам
// action.<действие>.footer и action.<действие>.help.
type Binding struct {
	Action   Action
	Category Category
	// как их возвращает KeyPressMsg.String(), пустой список значит действие отключено
	Keys []string
	// Footer: действие с подписью в подвале
	Footer bool
	// Help: действие со строкой в справке
	Help bool
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

// ActionFor ищет глобальное действие клавиши. Действия окон здесь не ищутся:
// окно разбирает свои клавиши само через Is.
func (k Keymap) ActionFor(key string) Action {
	for _, binding := range k.bindings {
		if !inWindow(binding.Action) && slices.Contains(binding.Keys, key) {
			return binding.Action
		}
	}

	return ""
}

// Is говорит, что клавиша вызывает действие. Им окно проверяет свои действия.
func (k Keymap) Is(action Action, key string) bool {
	return slices.Contains(k.KeysFor(action), key)
}

// windows это действия, которые действуют разом в одном окне. Конфликт
// клавиш ищется внутри окна: с глобальными действиями окно не конфликтует,
// потому что забирает нажатия целиком. Навигация общая у всех списков.
func windows() [][]Action {
	list := []Action{ListUp, ListDown, ListClose}
	return [][]Action{
		append([]Action{LogOpen, LogSearch}, list...),
		append([]Action{BranchesSwitch}, list...),
		append([]Action{WorktreesSwitch}, list...),
		append([]Action{StashDiff, StashApply, StashDrop}, list...),
		{QuestionYes},
		{SearchChoose, SearchClose, SearchUp, SearchDown},
		{CommitSubmit, CommitClose},
		{LogSearchDone, LogSearchCancel},
	}
}

func inWindow(action Action) bool {
	return slices.ContainsFunc(windows(), func(window []Action) bool {
		return slices.Contains(window, action)
	})
}

// inField: действие поля ввода, где остальные клавиши набирают текст.
func inField(action Action) bool {
	switch action {
	case SearchChoose, SearchClose, SearchUp, SearchDown, CommitSubmit, CommitClose, LogSearchDone, LogSearchCancel:
		return true
	}
	return false
}

// closes: действие закрытия окна. Без него из окна не выйти, кроме как
// закрыв приложение.
func closes(action Action) bool {
	switch action {
	case ListClose, SearchClose, CommitClose, LogSearchCancel:
		return true
	}
	return false
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
		return Keymap{}, i18n.New("json.extraText", nil)
	}

	bindings := defaults()
	// имена идут по порядку, чтобы ошибка при нескольких проблемах не
	// менялась от запуска к запуску
	for _, name := range slices.Sorted(maps.Keys(f.Keybinds)) {
		index := slices.IndexFunc(bindings, func(b Binding) bool { return string(b.Action) == name })
		if index < 0 {
			return Keymap{}, i18n.New("keymap.unknownAction", i18n.Args{"action": name})
		}

		keys, err := parseKeys(f.Keybinds[name])
		if err != nil {
			return Keymap{}, fmt.Errorf("%s: %w", name, err)
		}
		// переназначение заменяет клавиши действия целиком, не добавляет к ним
		bindings[index].Keys = keys
	}

	for _, binding := range bindings {
		if err := checkWindowKeys(binding); err != nil {
			return Keymap{}, err
		}
	}
	if err := checkConflicts(bindings); err != nil {
		return Keymap{}, err
	}

	return Keymap{bindings: bindings}, nil
}

// checkWindowKeys ловит при загрузке окно-ловушку и поле, которое теряет
// буквы. ctrl+c в окнах зашит: выход, а в вопросах «нет». backspace в полях
// стирает символ.
func checkWindowKeys(binding Binding) error {
	action := binding.Action
	if !inWindow(action) {
		return nil
	}
	if len(binding.Keys) == 0 && (closes(action) || action == QuestionYes) {
		return i18n.New("keymap.cannotDisable", i18n.Args{"action": action})
	}

	for _, key := range binding.Keys {
		args := i18n.Args{"action": action, "key": key}
		switch {
		case key == "ctrl+c", inField(action) && key == "backspace":
			return i18n.New("keymap.reserved", args)
		case closes(action) && utf8.RuneCountInString(key) > 1 && strings.ToLower(key) != key:
			return i18n.New("keymap.lowercase", args)
		case inField(action) && printable(key):
			return i18n.New("keymap.typesText", args)
		}
	}

	return nil
}

// printable: клавиша набирает текст. Для таких клавиш Bubble Tea отдаёт сам
// набранный текст (одна руна, заглавная, составной символ), для остальных
// имя (enter, ctrl+j, f1). shift+space в kitty набирает пробел.
func printable(key string) bool {
	key = strings.TrimPrefix(key, "shift+")
	if key == "space" || utf8.RuneCountInString(key) == 1 {
		return true
	}

	return !strings.Contains(key, "+") && strings.ContainsFunc(key, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
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

	return nil, i18n.New("keymap.badKeys", nil)
}

func checkKeys(keys []string) ([]string, error) {
	for _, key := range keys {
		if key == "" {
			return nil, i18n.New("keymap.emptyKey", nil)
		}
		if strings.HasPrefix(key, "<leader>") {
			return nil, i18n.New("keymap.leader", nil)
		}
	}

	return keys, nil
}

// checkConflicts ищет клавишу, назначенную двум действиям, которые действуют
// разом: среди глобальных (фокуса панелей нет) и внутри каждого окна.
func checkConflicts(bindings []Binding) error {
	var global []Action
	for _, binding := range bindings {
		if !inWindow(binding.Action) {
			global = append(global, binding.Action)
		}
	}

	for _, together := range append(windows(), global) {
		owner := make(map[string]Action)
		for _, binding := range bindings {
			if !slices.Contains(together, binding.Action) {
				continue
			}
			for _, key := range binding.Keys {
				if other, taken := owner[key]; taken {
					return i18n.New("keymap.conflict", i18n.Args{"key": key, "first": other, "second": binding.Action})
				}
				owner[key] = binding.Action
			}
		}
	}

	return nil
}

func defaults() []Binding {
	return []Binding{
		{Action: AppQuit, Category: CategoryGeneral, Keys: []string{"q", "ctrl+c"}, Footer: true, Help: true},
		{Action: FilesPick, Category: CategoryFiles, Keys: []string{"ctrl+p"}, Help: true},
		{Action: FilesToggle, Category: CategoryFiles, Keys: []string{"f"}, Help: true},
		{Action: AppHelp, Category: CategoryGeneral, Keys: []string{"?"}, Footer: true, Help: true},
		{Action: FilesNext, Category: CategoryFiles, Keys: []string{"]"}, Help: true},
		{Action: FilesPrevious, Category: CategoryFiles, Keys: []string{"["}, Help: true},
		{Action: FilesStage, Category: CategoryFiles, Keys: []string{"space"}, Help: true},
		{Action: FilesDiscard, Category: CategoryFiles, Keys: []string{"x"}, Help: true},
		{Action: FilesCommit, Category: CategoryFiles, Keys: []string{"c"}, Help: true},
		{Action: AppLog, Category: CategoryDiff, Keys: []string{"l"}, Help: true},
		{Action: DiffWorkingTree, Category: CategoryDiff, Keys: []string{"b"}, Help: true},
		{Action: AppBranches, Category: CategoryGeneral, Keys: []string{"B"}, Help: true},
		{Action: AppWorktrees, Category: CategoryGeneral, Keys: []string{"W"}, Help: true},
		{Action: StashPush, Category: CategoryFiles, Keys: []string{"z"}, Help: true},
		{Action: StashList, Category: CategoryFiles, Keys: []string{"Z"}, Help: true},
		{Action: DiffScrollDown, Category: CategoryDiff, Keys: []string{"j", "down"}, Help: true},
		{Action: DiffScrollUp, Category: CategoryDiff, Keys: []string{"k", "up"}, Help: true},
		{Action: DiffHalfPageDown, Category: CategoryDiff, Keys: []string{"ctrl+d"}, Help: true},
		{Action: DiffHalfPageUp, Category: CategoryDiff, Keys: []string{"ctrl+u"}, Help: true},
		{Action: DiffTop, Category: CategoryDiff, Keys: []string{"g"}, Help: true},
		{Action: DiffBottom, Category: CategoryDiff, Keys: []string{"G"}, Help: true},
		{Action: DiffToggleMode, Category: CategoryDiff, Keys: []string{"m"}, Help: true},
		{Action: DiffToggleWrap, Category: CategoryDiff, Keys: []string{"w"}, Help: true},
		{Action: DiffScrollLeft, Category: CategoryDiff, Keys: []string{"left"}, Help: true},
		{Action: DiffScrollRight, Category: CategoryDiff, Keys: []string{"right"}, Help: true},
		{Action: DiffReload, Category: CategoryDiff, Keys: []string{"r"}, Help: true},
		// в справке стоит в заголовке, как esc в окнах opencode
		{Action: HelpClose, Keys: []string{"esc"}},

		// окна: в подвал и справку не идут, клавиши видны в подсказке окна
		{Action: ListUp, Keys: []string{"k", "up"}},
		{Action: ListDown, Keys: []string{"j", "down"}},
		{Action: ListClose, Keys: []string{"esc"}},
		{Action: LogOpen, Keys: []string{"enter"}},
		{Action: LogSearch, Keys: []string{"/"}},
		{Action: BranchesSwitch, Keys: []string{"enter"}},
		{Action: WorktreesSwitch, Keys: []string{"enter"}},
		{Action: StashDiff, Keys: []string{"enter"}},
		{Action: StashApply, Keys: []string{"a"}},
		{Action: StashDrop, Keys: []string{"d"}},
		{Action: QuestionYes, Keys: []string{"y"}},
		{Action: SearchChoose, Keys: []string{"enter"}},
		{Action: SearchClose, Keys: []string{"esc"}},
		{Action: SearchUp, Keys: []string{"up", "ctrl+p"}},
		{Action: SearchDown, Keys: []string{"down", "ctrl+n"}},
		{Action: CommitSubmit, Keys: []string{"enter"}},
		{Action: CommitClose, Keys: []string{"esc"}},
		{Action: LogSearchDone, Keys: []string{"enter"}},
		{Action: LogSearchCancel, Keys: []string{"esc"}},
	}
}

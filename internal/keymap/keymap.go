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
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

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
	FilesIgnore      Action = "files.ignore"
	FilesEdit        Action = "files.edit"
	FilesAmend       Action = "files.amend"
	FilesHistory     Action = "files.history"
	AppLog           Action = "app.log"
	DiffWorkingTree  Action = "diff.workingTree"
	AppBranches      Action = "app.branches"
	AppWorktrees     Action = "app.worktrees"
	GitPull          Action = "git.pull"
	GitPush          Action = "git.push"
	GitFetch         Action = "git.fetch"
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
	// вид диффа (#217): пробелы и строки контекста
	DiffToggleWhitespace Action = "diff.toggleWhitespace"
	DiffMoreContext      Action = "diff.moreContext"
	DiffLessContext      Action = "diff.lessContext"
	DiffScrollLeft       Action = "diff.scrollLeft"
	DiffScrollRight      Action = "diff.scrollRight"
)

// Действия окон. Окно забирает нажатия целиком, поэтому эти действия ищутся
// только в своём окне, а глобальные внутри окна не действуют.
const (
	ListUp    Action = "list.up"
	ListDown  Action = "list.down"
	ListClose Action = "list.close"
	LogOpen   Action = "log.open"
	LogRevert Action = "log.revert"
	// ветка и checkout от коммита лога (#223)
	LogNewBranch Action = "log.newBranch"
	LogCheckout  Action = "log.checkout"
	LogCopy      Action = "log.copy"
	LogReset     Action = "log.reset"
	// cherry-pick (#222): copy в логе, paste в логе и в окне веток
	CherryPickCopy  Action = "cherryPick.copy"
	CherryPickPaste Action = "cherryPick.paste"
	// сравнение с веткой или коммитом (#216): в логе и в окне веток
	CompareWith Action = "compare.with"
	// фильтр лога по автору и пути (#226)
	LogFilter Action = "log.filter"
	// правка истории в логе (#219)
	LogSquash        Action = "log.squash"
	LogFixup         Action = "log.fixup"
	LogReword        Action = "log.reword"
	LogDrop          Action = "log.drop"
	LogMoveUp        Action = "log.moveUp"
	LogMoveDown      Action = "log.moveDown"
	BranchesSwitch   Action = "branches.switch"
	BranchesNew      Action = "branches.new"
	BranchesRename   Action = "branches.rename"
	BranchesDelete   Action = "branches.delete"
	BranchesMerge    Action = "branches.merge"
	BranchesRebase   Action = "branches.rebase"
	BranchesReset    Action = "branches.reset"
	BranchesUpstream Action = "branches.upstream"
	WorktreesSwitch  Action = "worktrees.switch"
	StashDiff        Action = "stash.diff"
	StashApply       Action = "stash.apply"
	StashPop         Action = "stash.pop"
	StashDrop        Action = "stash.drop"
	StashNew         Action = "stash.new"
	StashRename      Action = "stash.rename"
	QuestionYes      Action = "question.yes"
	SearchChoose     Action = "search.choose"
	SearchClose      Action = "search.close"
	SearchUp         Action = "search.up"
	SearchDown       Action = "search.down"
	CommitSubmit     Action = "commit.submit"
	CommitField      Action = "commit.field"
	CommitClose      Action = "commit.close"
	// поиск в логе, ветках и worktree: / открывает поле, done оставляет
	// фильтр, cancel снимает
	ListSearch       Action = "list.search"
	ListSearchDone   Action = "listSearch.done"
	ListSearchCancel Action = "listSearch.cancel"
	// однострочное поле ввода: имя ветки, тега, сообщение stash (#227)
	PromptSubmit Action = "prompt.submit"
	PromptClose  Action = "prompt.close"
	// выбор строки в меню (#236), навигация и закрытие общие со списками
	MenuChoose Action = "menu.choose"
)

// Category это группа в справке. Порядок групп задаёт первое появление
// категории в списке, подряд клавиши одной категории стоять не обязаны.
// Заголовок группы берётся из словаря по ключу category.<категория>.
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
		append([]Action{LogOpen, LogRevert, LogNewBranch, LogCheckout, LogCopy, LogReset, LogFilter, LogSquash, LogFixup, LogReword, LogDrop, LogMoveUp, LogMoveDown, CherryPickCopy, CherryPickPaste, CompareWith, ListSearch}, list...),
		append([]Action{BranchesSwitch, BranchesNew, BranchesRename, BranchesDelete, BranchesMerge, BranchesRebase, BranchesReset, BranchesUpstream, CherryPickPaste, CompareWith, ListSearch}, list...),
		append([]Action{WorktreesSwitch, ListSearch}, list...),
		append([]Action{StashDiff, StashApply, StashPop, StashDrop, StashNew, StashRename}, list...),
		append([]Action{MenuChoose}, list...),
		{QuestionYes},
		{SearchChoose, SearchClose, SearchUp, SearchDown},
		{CommitSubmit, CommitField, CommitClose},
		{ListSearchDone, ListSearchCancel},
		{PromptSubmit, PromptClose},
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
	case SearchChoose, SearchClose, SearchUp, SearchDown, CommitSubmit, CommitField, CommitClose, ListSearchDone, ListSearchCancel, PromptSubmit, PromptClose:
		return true
	}
	return false
}

// closes: действие закрытия окна. Без него из окна не выйти, кроме как
// закрыв приложение.
func closes(action Action) bool {
	switch action {
	case ListClose, SearchClose, CommitClose, ListSearchCancel, PromptClose:
		return true
	}
	return false
}

func Default() Keymap {
	return Keymap{bindings: defaults()}
}

// Load читает keybinds.json из каталога. Файла нет, значит встроенные клавиши.
// Файл с ошибкой отбрасывается целиком, как config.json. Ссылка, которая ведёт
// в никуда, ошибка, хоть ReadFile и отвечает на неё ErrNotExist: это и ссылка
// keybinds.json, и ссылка вместо каталога настроек. Правило то же, что у
// config.readFile: по графу пакетов keymap не может импортировать config, ради
// десяти строк стрелку не заводим.
func Load(dir string) (Keymap, error) {
	path := filepath.Join(dir, fileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) && !dangling(path) && !dangling(dir) {
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

// dangling говорит, что путь существует как символическая ссылка, но ведёт в
// никуда: Lstat видит ссылку, Stat уже нет.
func dangling(path string) bool {
	_, linkErr := os.Lstat(path)
	_, targetErr := os.Stat(path)

	return linkErr == nil && errors.Is(targetErr, fs.ErrNotExist)
}

type file struct {
	Schema   string                     `json:"$schema"`
	Keybinds map[string]json.RawMessage `json:"keybinds"`
}

// decode читает keybinds.json. Строгий разбор JSON (неизвестное поле и текст
// после объекта) скопирован из config.decodeStrict: по графу пакетов keymap
// не может импортировать config, ради восьми строк стрелку не заводим.
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
		if err := checkBindingKeys(binding); err != nil {
			return Keymap{}, err
		}
	}
	if err := checkConflicts(bindings); err != nil {
		return Keymap{}, err
	}

	return Keymap{bindings: bindings}, nil
}

// editorKeys это клавиши правки описания коммита: стрелки, home, end, delete и
// enter (новая строка). Они зашиты, как backspace, и в описании сильнее
// переключения поля, поэтому commit.field их не достаются.
var editorKeys = []string{"left", "right", "up", "down", "home", "end", "delete", "enter"}

// checkBindingKeys ловит при загрузке окно-ловушку и поле, которое теряет
// буквы. ctrl+c зашит: в окнах выход, а в вопросах «нет», на главном экране
// выход, поэтому глобальному действию он не достаётся, кроме app.quit.
// backspace в полях стирает символ.
func checkBindingKeys(binding Binding) error {
	action := binding.Action
	if !inWindow(action) {
		if action != AppQuit && slices.Contains(binding.Keys, "ctrl+c") {
			return i18n.New("keymap.reservedQuit", i18n.Args{"action": action, "key": "ctrl+c"})
		}
		return nil
	}
	if len(binding.Keys) == 0 {
		args := i18n.Args{"action": action}
		switch {
		case closes(action):
			return i18n.New("keymap.cannotDisable", args)
		case action == QuestionYes:
			// выход из вопроса есть, любая другая клавиша значит «нет»
			return i18n.New("keymap.cannotDisableYes", args)
		}
	}

	for _, key := range binding.Keys {
		args := i18n.Args{"action": action, "key": key}
		switch {
		case key == "ctrl+c", inField(action) && key == "backspace", action == CommitField && slices.Contains(editorKeys, key):
			return i18n.New("keymap.reserved", args)
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

// checkKeys проверяет каждую клавишу и убирает повторы: одна клавиша дважды в
// одном действии не конфликт. Имя действия к ошибке добавляет вызывающий.
func checkKeys(keys []string) ([]string, error) {
	for _, key := range keys {
		var err error
		switch {
		case key == "":
			err = i18n.New("keymap.emptyKey", nil)
		case key == "none":
			err = i18n.New("keymap.noneInList", nil)
		case strings.HasPrefix(key, "<leader>"):
			err = i18n.New("keymap.leader", nil)
		default:
			err = checkKey(key)
		}
		if err != nil {
			return nil, err
		}
	}

	seen := make(map[string]bool)
	return slices.DeleteFunc(keys, func(key string) bool {
		repeat := seen[key]
		seen[key] = true
		return repeat
	}), nil
}

// modifiers идут в том порядке, в каком их пишет Keystroke.
var modifiers = []string{"ctrl", "alt", "shift", "meta", "hyper", "super"}

// keyNames это клавиши, которые Keystroke пишет словом: значения keyTypeString
// из github.com/charmbracelet/ultraviolet (версия в go.mod) без одиночных
// символов и функциональных клавиш f1…f63. Остальное Bubble Tea присылает
// самим символом.
const keyNames = `enter tab backspace esc space up down left right begin find insert delete select
pgup pgdown home end equal mul plus comma minus period div sep
capslock scrolllock numlock printscreen pause menu
mediaplay mediapause mediaplaypause mediareverse mediastop mediafastforward mediarewind
medianext mediaprev mediarecord lowervol raisevol mute
leftshift leftalt leftctrl leftsuper lefthyper leftmeta
rightshift rightalt rightctrl rightsuper righthyper rightmeta isolevel3shift isolevel5shift`

// checkKey ловит клавишу, которой Bubble Tea не пришлёт: такая привязка молча
// мертва. Клавиша это модификаторы в порядке Keystroke и основа, один
// графемный символ или имя из keyNames. Заглавная буква сама по себе
// приходит заглавной («B»), а с модификатором код строчный.
func checkKey(key string) error {
	base := key
	var mods []string
	for _, mod := range modifiers {
		if rest, found := strings.CutPrefix(base, mod+"+"); found && rest != "" {
			base = rest
			mods = append(mods, mod)
		}
	}

	if !keyName(base) && !symbol(base) {
		return i18n.New("keymap.unknownKey", i18n.Args{"key": key})
	}

	letter, size := utf8.DecodeRuneInString(base)
	if len(mods) == 0 || size != len(base) || !unicode.IsLetter(letter) {
		return nil
	}

	args := i18n.Args{"key": key, "capital": strings.ToUpper(base)}
	switch {
	case slices.Equal(mods, []string{"shift"}):
		return i18n.New("keymap.shiftLetter", args)
	case base != strings.ToLower(base):
		return i18n.New("keymap.lowercase", args)
	}

	return nil
}

func keyName(base string) bool {
	if slices.Contains(strings.Fields(keyNames), base) {
		return true
	}

	digits := strings.TrimPrefix(base, "f")
	number, err := strconv.Atoi(digits)
	return digits != base && err == nil && strconv.Itoa(number) == digits && number >= 1 && number <= 63
}

// symbol: основа это один видимый графемный символ. Пробел сюда не входит,
// Bubble Tea называет его space.
func symbol(base string) bool {
	cluster, _ := ansi.FirstGraphemeCluster(base, ansi.GraphemeWidth)
	first, _ := utf8.DecodeRuneInString(base)

	return cluster == base && unicode.IsGraphic(first) && !unicode.IsSpace(first)
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
		{Action: FilesIgnore, Category: CategoryFiles, Keys: []string{"i"}, Help: true},
		{Action: FilesEdit, Category: CategoryFiles, Keys: []string{"e"}, Help: true},
		{Action: FilesAmend, Category: CategoryFiles, Keys: []string{"A"}, Help: true},
		{Action: FilesHistory, Category: CategoryFiles, Keys: []string{"h"}, Help: true},
		{Action: AppLog, Category: CategoryDiff, Keys: []string{"l"}, Help: true},
		{Action: DiffWorkingTree, Category: CategoryDiff, Keys: []string{"b"}, Help: true},
		{Action: AppBranches, Category: CategoryGeneral, Keys: []string{"B"}, Help: true},
		{Action: AppWorktrees, Category: CategoryGeneral, Keys: []string{"W"}, Help: true},
		{Action: GitPull, Category: CategoryGeneral, Keys: []string{"p"}, Help: true},
		{Action: GitPush, Category: CategoryGeneral, Keys: []string{"P"}, Help: true},
		// f у lazygit, здесь f открывает панель файлов
		{Action: GitFetch, Category: CategoryGeneral, Keys: []string{"F"}, Help: true},
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
		// как в lazygit: ctrl+w пробелы, } и { контекст
		{Action: DiffToggleWhitespace, Category: CategoryDiff, Keys: []string{"ctrl+w"}, Help: true},
		{Action: DiffMoreContext, Category: CategoryDiff, Keys: []string{"}"}, Help: true},
		{Action: DiffLessContext, Category: CategoryDiff, Keys: []string{"{"}, Help: true},
		{Action: DiffScrollLeft, Category: CategoryDiff, Keys: []string{"left"}, Help: true},
		{Action: DiffScrollRight, Category: CategoryDiff, Keys: []string{"right"}, Help: true},
		{Action: DiffReload, Category: CategoryDiff, Keys: []string{"r"}, Help: true},
		// в справке стоит в заголовке, как esc в окнах opencode
		{Action: HelpClose, Keys: []string{"esc"}},

		// окна: в подвал и справку не идут, их клавиши перечислены в
		// docs/config.md. Подсказка окна показывает не все: навигации
		// (list.up, list.down, search.up, search.down) в ней нет
		{Action: ListUp, Keys: []string{"k", "up"}},
		{Action: ListDown, Keys: []string{"j", "down"}},
		{Action: ListClose, Keys: []string{"esc"}},
		{Action: LogOpen, Keys: []string{"enter"}},
		{Action: LogRevert, Keys: []string{"t"}},
		{Action: CherryPickCopy, Keys: []string{"C"}},
		{Action: CherryPickPaste, Keys: []string{"V"}},
		{Action: CompareWith, Keys: []string{"W"}},
		{Action: LogFilter, Keys: []string{"ctrl+s"}},
		{Action: LogSquash, Keys: []string{"s"}},
		{Action: LogFixup, Keys: []string{"f"}},
		{Action: LogReword, Keys: []string{"r"}},
		{Action: LogDrop, Keys: []string{"d"}},
		{Action: LogMoveUp, Keys: []string{"ctrl+k"}},
		{Action: LogMoveDown, Keys: []string{"ctrl+j"}},
		{Action: LogNewBranch, Keys: []string{"n"}},
		{Action: LogCheckout, Keys: []string{"space"}},
		{Action: LogCopy, Keys: []string{"y"}},
		{Action: LogReset, Keys: []string{"g"}},
		{Action: ListSearch, Keys: []string{"/"}},
		{Action: BranchesSwitch, Keys: []string{"enter"}},
		{Action: BranchesNew, Keys: []string{"n"}},
		{Action: BranchesRename, Keys: []string{"R"}},
		{Action: BranchesDelete, Keys: []string{"d"}},
		{Action: BranchesMerge, Keys: []string{"M"}},
		{Action: BranchesRebase, Keys: []string{"r"}},
		{Action: BranchesReset, Keys: []string{"g"}},
		{Action: BranchesUpstream, Keys: []string{"u"}},
		{Action: WorktreesSwitch, Keys: []string{"enter"}},
		{Action: StashDiff, Keys: []string{"enter"}},
		{Action: StashApply, Keys: []string{"a"}},
		{Action: StashPop, Keys: []string{"g"}},
		{Action: StashNew, Keys: []string{"n"}},
		{Action: StashRename, Keys: []string{"r"}},
		{Action: StashDrop, Keys: []string{"d"}},
		{Action: QuestionYes, Keys: []string{"y"}},
		{Action: SearchChoose, Keys: []string{"enter"}},
		{Action: SearchClose, Keys: []string{"esc"}},
		{Action: SearchUp, Keys: []string{"up", "ctrl+p"}},
		{Action: SearchDown, Keys: []string{"down", "ctrl+n"}},
		{Action: CommitSubmit, Keys: []string{"enter"}},
		{Action: CommitField, Keys: []string{"tab"}},
		{Action: CommitClose, Keys: []string{"esc"}},
		{Action: ListSearchDone, Keys: []string{"enter"}},
		{Action: ListSearchCancel, Keys: []string{"esc"}},
		{Action: PromptSubmit, Keys: []string{"enter"}},
		{Action: PromptClose, Keys: []string{"esc"}},
		{Action: MenuChoose, Keys: []string{"enter"}},
	}
}

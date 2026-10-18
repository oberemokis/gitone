package keymap_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gitone/internal/keymap"
)

func TestLoad(t *testing.T) {
	defaultQuit := []string{"q", "ctrl+c"}

	tests := []struct {
		name    string
		missing bool
		content string
		want    []string
		// подстрока ошибки, пусто значит ошибки нет
		wantErr string
	}{
		{name: "нет файла", missing: true, want: defaultQuit},
		{name: "переназначение", content: `{"keybinds":{"app.quit":"Q"}}`, want: []string{"Q"}},
		{name: "массив", content: `{"keybinds":{"app.quit":["Q","y"]}}`, want: []string{"Q", "y"}},
		{name: "повтор клавиши в действии", content: `{"keybinds":{"app.quit":["q","ctrl+c","q"]}}`, want: []string{"q", "ctrl+c"}},
		{name: "ctrl+c другому глобальному действию", content: `{"keybinds":{"app.quit":"Q","files.pick":"ctrl+c"}}`, want: defaultQuit, wantErr: "files.pick"},
		{name: "none отключает", content: `{"keybinds":{"app.quit":"none"}}`, want: nil},
		{name: "false отключает", content: `{"keybinds":{"app.quit":false}}`, want: nil},
		{name: "неизвестное действие", content: `{"keybinds":{"app.quitt":"w"}}`, want: defaultQuit, wantErr: "app.quitt"},
		{name: "конфликт клавиш", content: `{"keybinds":{"files.pick":"q"}}`, want: defaultQuit, wantErr: "files.pick"},
		{name: "число вместо клавиши", content: `{"keybinds":{"app.quit":5}}`, want: defaultQuit, wantErr: "app.quit"},
		{name: "пустая клавиша", content: `{"keybinds":{"app.quit":""}}`, want: defaultQuit, wantErr: "app.quit"},
		{name: "поле leader", content: `{"leader":"ctrl+x"}`, want: defaultQuit, wantErr: "leader"},
		{name: "leader в клавише", content: `{"keybinds":{"app.quit":"<leader>q"}}`, want: defaultQuit, wantErr: "leader"},
		{name: "не JSON", content: `{`, want: defaultQuit, wantErr: "keybinds.json"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tc.missing {
				if err := os.WriteFile(filepath.Join(dir, "keybinds.json"), []byte(tc.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			km, err := keymap.Load(dir)

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Load(keybinds.json %q) error = %v, want nil", tc.content, err)
				}
			} else {
				if err == nil {
					t.Fatalf("Load(keybinds.json %q) error = nil, want error with %q", tc.content, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "keybinds.json") {
					t.Errorf("Load(keybinds.json %q) error = %q, want error with keybinds.json and %q", tc.content, err, tc.wantErr)
				}
			}

			if got := km.KeysFor(keymap.AppQuit); !slices.Equal(got, tc.want) {
				t.Errorf("Load(keybinds.json %q).KeysFor(app.quit) = %q, want %q", tc.content, got, tc.want)
			}
		})
	}
}

// Ссылка на keybinds.json или на каталог настроек, которая существует и ведёт в
// никуда, не то же, что отсутствие файла: клавиши пользователя есть и не
// читаются, молча подменять их встроенными нельзя.
func TestLoadBrokenSymlinkIsError(t *testing.T) {
	tests := []struct {
		name string
		// ссылка от корня временного каталога, каталог настроек это settings
		link string
	}{
		{name: "файл", link: "settings/keybinds.json"},
		{name: "каталог настроек", link: "settings"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			link := filepath.Join(root, tc.link)
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(root, "gone"), link); err != nil {
				t.Fatal(err)
			}

			_, err := keymap.Load(filepath.Join(root, "settings"))

			if err == nil || !strings.HasPrefix(err.Error(), "keybinds.json") {
				t.Errorf("Load(ссылка %s на то, чего нет) error = %v, want error starting with keybinds.json", tc.link, err)
			}
		})
	}
}

// Ссылка на существующий каталог без keybinds.json остаётся отсутствием файла.
func TestLoadDirectorySymlinkWithoutFileIsNotError(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "dotfiles")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "settings")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	km, err := keymap.Load(link)

	if got := km.KeysFor(keymap.AppQuit); err != nil || !slices.Equal(got, []string{"q", "ctrl+c"}) {
		t.Fatalf("Load(ссылка на каталог без keybinds.json) = %q, %v; want встроенные клавиши без ошибки", got, err)
	}
}

// Имена клавиш сверяются с тем, что отдаёт KeyPressMsg.String(): клавиша,
// которой Bubble Tea не пришлёт, молча мертва, поэтому это ошибка загрузки.
// Ошибка называет действие и клавишу.
func TestLoadRejectsUnsendableKeys(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr []string
	}{
		{name: "escape вместо esc", content: `{"keybinds":{"commit.close":"escape"}}`, wantErr: []string{"commit.close", `"escape"`}},
		{name: "имя с заглавной", content: `{"keybinds":{"question.yes":"Enter"}}`, wantErr: []string{"question.yes", `"Enter"`}},
		{name: "none внутри массива", content: `{"keybinds":{"app.quit":["none"]}}`, wantErr: []string{"app.quit", `"none"`}},
		{name: "shift и буква", content: `{"keybinds":{"diff.toggleWrap":"shift+b"}}`, wantErr: []string{"diff.toggleWrap", `"shift+b"`, `write "B"`}},
		{name: "pagedown вместо pgdown", content: `{"keybinds":{"diff.scrollDown":"pagedown"}}`, wantErr: []string{"diff.scrollDown", `"pagedown"`}},
		{name: "пробел вместо space", content: `{"keybinds":{"files.stage":" "}}`, wantErr: []string{"files.stage", `" "`}},
		{name: "порядок модификаторов", content: `{"keybinds":{"files.pick":"alt+ctrl+k"}}`, wantErr: []string{"files.pick", `"alt+ctrl+k"`}},
		{name: "модификатор без клавиши", content: `{"keybinds":{"files.pick":"ctrl+"}}`, wantErr: []string{"files.pick", `"ctrl+"`}},
		{name: "заглавная с модификатором", content: `{"keybinds":{"files.pick":"ctrl+P"}}`, wantErr: []string{"files.pick", `"ctrl+P"`, "lowercase"}},
		{name: "два символа", content: `{"keybinds":{"files.pick":"ab"}}`, wantErr: []string{"files.pick", `"ab"`}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := keymap.Load(writeKeybinds(t, tc.content))

			if err == nil {
				t.Fatalf("Load(keybinds.json %q) error = nil, want error with %q", tc.content, tc.wantErr)
			}
			for _, part := range tc.wantErr {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("Load(keybinds.json %q) error = %q, want error with %q", tc.content, err, part)
				}
			}
		})
	}
}

// Имена, которые Bubble Tea присылает, загружаются как есть.
func TestLoadAcceptsSendableKeys(t *testing.T) {
	tests := []struct {
		name   string
		action keymap.Action
		key    string
	}{
		{name: "заглавная буква", action: keymap.DiffToggleWrap, key: "E"},
		{name: "ctrl и shift с буквой", action: keymap.FilesPick, key: "ctrl+shift+p"},
		{name: "pgdown", action: keymap.DiffScrollDown, key: "pgdown"},
		{name: "space", action: keymap.FilesStage, key: "space"},
		{name: "функциональная клавиша", action: keymap.DiffReload, key: "f5"},
		{name: "alt и имя", action: keymap.FilesCommit, key: "alt+enter"},
		{name: "shift и имя", action: keymap.FilesToggle, key: "shift+tab"},
		{name: "плюс", action: keymap.FilesNext, key: "+"},
		{name: "ctrl и плюс", action: keymap.FilesPrevious, key: "ctrl++"},
		{name: "составной символ", action: keymap.DiffTop, key: "👍🏻"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content := `{"keybinds":{"` + string(tc.action) + `":"` + tc.key + `"}}`

			km, err := keymap.Load(writeKeybinds(t, content))

			if err != nil {
				t.Fatalf("Load(keybinds.json %q) error = %v, want nil", content, err)
			}
			if got := km.KeysFor(tc.action); !slices.Equal(got, []string{tc.key}) {
				t.Errorf("Load(keybinds.json %q).KeysFor(%s) = %q, want %q", content, tc.action, got, []string{tc.key})
			}
		})
	}
}

// writeKeybinds кладёт keybinds.json с заданным содержимым в новый каталог.
func writeKeybinds(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keybinds.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

// Клавиши окон проверяются при загрузке: окно не должно стать ловушкой, а поле
// ввода не должно терять буквы. Ошибка называет действия, чтобы было видно,
// что править.
func TestLoadWindowKeys(t *testing.T) {
	tests := []struct {
		name    string
		content string
		// подстроки ошибки, пусто значит ошибки нет
		wantErr []string
	}{
		{name: "клавиши окон по умолчанию", content: `{}`},
		{name: "окно с глобальной клавишей", content: `{"keybinds":{"list.down":"q"}}`},
		{name: "переназначение навигации", content: `{"keybinds":{"list.down":"J"}}`},
		{name: "конфликт внутри окна", content: `{"keybinds":{"list.down":"a"}}`, wantErr: []string{"bound to both", "list.down", "stash.apply"}},
		{name: "конфликт в окне веток", content: `{"keybinds":{"branches.new":"/"}}`, wantErr: []string{"bound to both", "branches.new", "list.search"}},
		{name: "печатаемая клавиша в поле ввода", content: `{"keybinds":{"prompt.submit":"y"}}`, wantErr: []string{"types text", "prompt.submit"}},
		{name: "закрытие поля ввода не отключается", content: `{"keybinds":{"prompt.close":"none"}}`, wantErr: []string{"cannot be disabled", "prompt.close"}},
		{name: "печатаемая клавиша в поиске", content: `{"keybinds":{"search.down":"j"}}`, wantErr: []string{"types text", "search.down"}},
		{name: "переключение поля коммита", content: `{"keybinds":{"commit.field":"ctrl+t"}}`},
		{name: "переключение поля отключено", content: `{"keybinds":{"commit.field":"none"}}`},
		{name: "печатаемая клавиша переключает поле", content: `{"keybinds":{"commit.field":"x"}}`, wantErr: []string{"types text", "commit.field"}},
		{name: "переключение поля заняло клавишу закрытия", content: `{"keybinds":{"commit.field":"esc"}}`, wantErr: []string{"bound to both", "commit.field", "commit.close"}},
		{name: "переключение поля заняло стрелку редактора", content: `{"keybinds":{"commit.field":"up"}}`, wantErr: []string{"reserved", "commit.field"}},
		{name: "переключение поля заняло enter редактора", content: `{"keybinds":{"commit.field":"enter"}}`, wantErr: []string{"reserved", "commit.field"}},
		{name: "пробел в поле коммита", content: `{"keybinds":{"commit.submit":"space"}}`, wantErr: []string{"types text", "commit.submit"}},
		{name: "shift+space в поле коммита", content: `{"keybinds":{"commit.submit":"shift+space"}}`, wantErr: []string{"types text", "commit.submit"}},
		{name: "заглавная в поле поиска", content: `{"keybinds":{"search.choose":"J"}}`, wantErr: []string{"types text", "search.choose"}},
		{name: "многорунный символ в поле", content: `{"keybinds":{"search.choose":"👍🏻"}}`, wantErr: []string{"types text", "search.choose"}},
		{name: "непечатаемая клавиша в поиске", content: `{"keybinds":{"search.down":"ctrl+j"}}`},
		{name: "функциональная клавиша в поиске", content: `{"keybinds":{"search.down":"f2"}}`},
		{name: "поиск в списке занял клавишу лога", content: `{"keybinds":{"list.search":"enter","branches.switch":"o","worktrees.switch":"w"}}`, wantErr: []string{"bound to both", "list.search", "log.open"}},
		{name: "поиск в списке занял клавишу веток", content: `{"keybinds":{"list.search":"enter","log.open":"o","worktrees.switch":"w"}}`, wantErr: []string{"bound to both", "list.search", "branches.switch"}},
		{name: "поиск в списке занял клавишу worktree", content: `{"keybinds":{"list.search":"enter","log.open":"o","branches.switch":"b"}}`, wantErr: []string{"bound to both", "list.search", "worktrees.switch"}},
		{name: "закрытие отключено", content: `{"keybinds":{"list.close":"none"}}`, wantErr: []string{"cannot be disabled", "list.close"}},
		{name: "закрытие поиска отключено", content: `{"keybinds":{"search.close":"none"}}`, wantErr: []string{"cannot be disabled", "search.close"}},
		{name: "закрытие пустым списком", content: `{"keybinds":{"list.close":[]}}`, wantErr: []string{"cannot be disabled", "list.close"}},
		{name: "закрытие с заглавной в имени", content: `{"keybinds":{"commit.close":"Esc"}}`, wantErr: []string{"unknown key", "commit.close"}},
		{name: "закрытие заглавной буквой", content: `{"keybinds":{"list.close":"Q"}}`},
		{name: "ответ да отключён", content: `{"keybinds":{"question.yes":"none"}}`, wantErr: []string{"cannot be disabled", "question.yes", "cannot be confirmed"}},
		{name: "ответ да заглавной", content: `{"keybinds":{"question.yes":"Y"}}`},
		{name: "ctrl+c в окне", content: `{"keybinds":{"list.close":"ctrl+c"}}`, wantErr: []string{"reserved", "list.close"}},
		{name: "backspace в поле", content: `{"keybinds":{"search.close":"backspace"}}`, wantErr: []string{"reserved", "search.close"}},
		{name: "отключение действия окна", content: `{"keybinds":{"stash.apply":"none"}}`},
		{name: "поиск в списках", content: `{"keybinds":{"list.search":"s","listSearch.done":"tab"}}`},
		{name: "старое имя поиска", content: `{"keybinds":{"log.search":"s"}}`, wantErr: []string{"log.search"}},
		{name: "печатаемая клавиша в поле списка", content: `{"keybinds":{"listSearch.done":"x"}}`, wantErr: []string{"types text", "listSearch.done"}},
		{name: "сброс поиска отключён", content: `{"keybinds":{"listSearch.cancel":"none"}}`, wantErr: []string{"cannot be disabled", "listSearch.cancel"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "keybinds.json"), []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := keymap.Load(dir)

			if len(tc.wantErr) == 0 {
				if err != nil {
					t.Errorf("Load(keybinds.json %q) error = %v, want nil", tc.content, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Load(keybinds.json %q) error = nil, want error with %q", tc.content, tc.wantErr)
			}
			for _, part := range tc.wantErr {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("Load(keybinds.json %q) error = %q, want error with %q", tc.content, err, part)
				}
			}
		})
	}
}

// Pull и push стоят на p и P, как в lazygit, и ведут себя как остальные
// глобальные действия: переназначаются, отключаются, конфликтуют по клавише.
func TestPullAndPushKeys(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantPull []string
		wantPush []string
		// клавиша и действие, к которому она ведёт, пусто значит никакое
		key        string
		wantAction keymap.Action
		wantErr    []string
	}{
		{name: "по умолчанию", content: `{}`, wantPull: []string{"p"}, wantPush: []string{"P"}, key: "p", wantAction: keymap.GitPull},
		{name: "push на заглавной", content: `{}`, wantPull: []string{"p"}, wantPush: []string{"P"}, key: "P", wantAction: keymap.GitPush},
		{name: "переназначение", content: `{"keybinds":{"git.pull":"ctrl+l"}}`, wantPull: []string{"ctrl+l"}, wantPush: []string{"P"}, key: "ctrl+l", wantAction: keymap.GitPull},
		{name: "старая клавиша свободна", content: `{"keybinds":{"git.pull":"ctrl+l"}}`, wantPull: []string{"ctrl+l"}, wantPush: []string{"P"}, key: "p"},
		{name: "отключение pull", content: `{"keybinds":{"git.pull":"none"}}`, wantPush: []string{"P"}, key: "p"},
		{name: "отключение push", content: `{"keybinds":{"git.push":false}}`, wantPull: []string{"p"}, key: "P"},
		{name: "конфликт pull и push", content: `{"keybinds":{"git.pull":"P"}}`, wantErr: []string{"bound to both", "git.pull", "git.push"}},
		{name: "конфликт с другим действием", content: `{"keybinds":{"git.push":"z"}}`, wantErr: []string{"bound to both", "git.push", "stash.push"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			km, err := keymap.Load(writeKeybinds(t, tc.content))

			if len(tc.wantErr) > 0 {
				if err == nil {
					t.Fatalf("Load(keybinds.json %q) error = nil, want error with %q", tc.content, tc.wantErr)
				}
				for _, part := range tc.wantErr {
					if !strings.Contains(err.Error(), part) {
						t.Errorf("Load(keybinds.json %q) error = %q, want error with %q", tc.content, err, part)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("Load(keybinds.json %q) error = %v, want nil", tc.content, err)
			}
			if got := km.KeysFor(keymap.GitPull); !slices.Equal(got, tc.wantPull) {
				t.Errorf("KeysFor(git.pull) = %q, want %q", got, tc.wantPull)
			}
			if got := km.KeysFor(keymap.GitPush); !slices.Equal(got, tc.wantPush) {
				t.Errorf("KeysFor(git.push) = %q, want %q", got, tc.wantPush)
			}
			if got := km.ActionFor(tc.key); got != tc.wantAction {
				t.Errorf("ActionFor(%q) = %q, want %q", tc.key, got, tc.wantAction)
			}
		})
	}
}

// Вне окон клавиша ищется только среди глобальных действий: отключённая
// прокрутка не превращает j в навигацию списка.
func TestActionForIgnoresWindowActions(t *testing.T) {
	dir := t.TempDir()
	content := `{"keybinds":{"diff.scrollDown":"none"}}`
	if err := os.WriteFile(filepath.Join(dir, "keybinds.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	km, err := keymap.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := km.ActionFor("j"); got != "" {
		t.Errorf("ActionFor(%q) = %q, want ничего", "j", got)
	}
	if !km.Is(keymap.ListDown, "j") {
		t.Errorf("Is(%q, %q) = false, want true", keymap.ListDown, "j")
	}
}

// Клавиша из файла ведёт к своему действию, а не к встроенному.
func TestActionForFollowsOverride(t *testing.T) {
	dir := t.TempDir()
	content := `{"keybinds":{"app.quit":"Q"}}`
	if err := os.WriteFile(filepath.Join(dir, "keybinds.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	km, err := keymap.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := km.ActionFor("Q"); got != keymap.AppQuit {
		t.Errorf("Q ведёт к %q, ожидался %q", got, keymap.AppQuit)
	}
	if got := km.ActionFor("q"); got != "" {
		t.Errorf("q ведёт к %q, ожидалось ничего", got)
	}
}

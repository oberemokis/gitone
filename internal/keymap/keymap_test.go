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
		{name: "переназначение", content: `{"keybinds":{"app.quit":"e"}}`, want: []string{"e"}},
		{name: "массив", content: `{"keybinds":{"app.quit":["e","y"]}}`, want: []string{"e", "y"}},
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
		{name: "переназначение навигации", content: `{"keybinds":{"list.down":"n"}}`},
		{name: "конфликт внутри окна", content: `{"keybinds":{"list.down":"d"}}`, wantErr: []string{"bound to both", "list.down", "stash.drop"}},
		{name: "печатаемая клавиша в поиске", content: `{"keybinds":{"search.down":"j"}}`, wantErr: []string{"types text", "search.down"}},
		{name: "пробел в поле коммита", content: `{"keybinds":{"commit.submit":"space"}}`, wantErr: []string{"types text", "commit.submit"}},
		{name: "shift+space в поле коммита", content: `{"keybinds":{"commit.submit":"shift+space"}}`, wantErr: []string{"types text", "commit.submit"}},
		{name: "заглавная в поле поиска", content: `{"keybinds":{"search.choose":"J"}}`, wantErr: []string{"types text", "search.choose"}},
		{name: "многорунный символ в поле", content: `{"keybinds":{"search.choose":"👍🏻"}}`, wantErr: []string{"types text", "search.choose"}},
		{name: "непечатаемая клавиша в поиске", content: `{"keybinds":{"search.down":"ctrl+j"}}`},
		{name: "закрытие отключено", content: `{"keybinds":{"list.close":"none"}}`, wantErr: []string{"cannot be disabled", "list.close"}},
		{name: "закрытие пустым списком", content: `{"keybinds":{"list.close":[]}}`, wantErr: []string{"cannot be disabled", "list.close"}},
		{name: "закрытие с заглавной", content: `{"keybinds":{"commit.close":"Esc"}}`, wantErr: []string{"lowercase", "commit.close"}},
		{name: "ответ да отключён", content: `{"keybinds":{"question.yes":"none"}}`, wantErr: []string{"cannot be disabled", "question.yes"}},
		{name: "ответ да заглавной", content: `{"keybinds":{"question.yes":"Y"}}`},
		{name: "ctrl+c в окне", content: `{"keybinds":{"list.close":"ctrl+c"}}`, wantErr: []string{"reserved", "list.close"}},
		{name: "backspace в поле", content: `{"keybinds":{"search.close":"backspace"}}`, wantErr: []string{"reserved", "search.close"}},
		{name: "отключение действия окна", content: `{"keybinds":{"stash.apply":"none"}}`},
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
	content := `{"keybinds":{"app.quit":"e"}}`
	if err := os.WriteFile(filepath.Join(dir, "keybinds.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	km, err := keymap.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := km.ActionFor("e"); got != keymap.AppQuit {
		t.Errorf("x ведёт к %q, ожидался %q", got, keymap.AppQuit)
	}
	if got := km.ActionFor("q"); got != "" {
		t.Errorf("q ведёт к %q, ожидалось ничего", got)
	}
}

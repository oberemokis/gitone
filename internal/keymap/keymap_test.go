package keymap

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
		{name: "переназначение", content: `{"keybinds":{"app.quit":"w"}}`, want: []string{"w"}},
		{name: "массив", content: `{"keybinds":{"app.quit":["w","y"]}}`, want: []string{"w", "y"}},
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

			km, err := Load(dir)

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ошибка %v, ожидалась без ошибки", err)
				}
			} else {
				if err == nil {
					t.Fatalf("ошибки нет, ожидалась с текстом %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "keybinds.json") {
					t.Fatalf("ошибка %q не называет файл и %q", err, tc.wantErr)
				}
			}

			if got := km.KeysFor(AppQuit); !slices.Equal(got, tc.want) {
				t.Fatalf("клавиши app.quit %q, ожидались %q", got, tc.want)
			}
		})
	}
}

// Клавиша из файла ведёт к своему действию, а не к встроенному.
func TestActionForFollowsOverride(t *testing.T) {
	dir := t.TempDir()
	content := `{"keybinds":{"app.quit":"w"}}`
	if err := os.WriteFile(filepath.Join(dir, "keybinds.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	km, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := km.ActionFor("w"); got != AppQuit {
		t.Fatalf("x ведёт к %q, ожидался %q", got, AppQuit)
	}
	if got := km.ActionFor("q"); got != "" {
		t.Fatalf("q ведёт к %q, ожидалось ничего", got)
	}
}

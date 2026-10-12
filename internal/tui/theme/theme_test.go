package theme_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"gitone/internal/config"
	"gitone/internal/tui/theme"
)

func TestBuildOverlaysDark(t *testing.T) {
	dark := theme.Dark()

	tests := []struct {
		name string
		file config.ThemeFile
		// проверяет поля, которые должны измениться или остаться
		check func(t *testing.T, got theme.Theme)
		// подстрока ошибки, пусто значит ошибки нет
		wantErr string
	}{
		{
			name: "пустой файл дает dark",
			file: config.ThemeFile{},
			check: func(t *testing.T, got theme.Theme) {
				if got != dark {
					t.Fatalf("тема %+v, ожидалась dark", got)
				}
			},
		},
		{
			name: "имя из defs",
			file: config.ThemeFile{
				Defs:   map[string]config.Color{"green": "#3fb950"},
				Colors: map[string]config.Color{"diffAdded": "green"},
			},
			check: func(t *testing.T, got theme.Theme) {
				if got.DiffAdded != lipgloss.Color("#3fb950") {
					t.Fatalf("diffAdded %v, ожидался #3fb950", got.DiffAdded)
				}
				if got.DiffRemoved != dark.DiffRemoved {
					t.Fatalf("diffRemoved изменился без причины")
				}
			},
		},
		{
			name: "число как ANSI",
			file: config.ThemeFile{Colors: map[string]config.Color{"textMuted": "8"}},
			check: func(t *testing.T, got theme.Theme) {
				if got.TextMuted != lipgloss.Color("8") {
					t.Fatalf("textMuted %v, ожидался ANSI 8", got.TextMuted)
				}
			},
		},
		{
			name: "none это цвет терминала",
			file: config.ThemeFile{Colors: map[string]config.Color{"backgroundPanel": "none"}},
			check: func(t *testing.T, got theme.Theme) {
				if got.BackgroundPanel != (lipgloss.NoColor{}) {
					t.Fatalf("backgroundPanel %v, ожидался NoColor", got.BackgroundPanel)
				}
			},
		},
		{
			name:    "неверный hex",
			file:    config.ThemeFile{Colors: map[string]config.Color{"accent": "#zzz"}},
			wantErr: "accent",
		},
		{
			name:    "имя не из defs",
			file:    config.ThemeFile{Colors: map[string]config.Color{"accent": "nope"}},
			wantErr: "nope",
		},
		{
			name:    "ключ роли, которого в коде нет",
			file:    config.ThemeFile{Colors: map[string]config.Color{"wrong": "#000000"}},
			wantErr: "wrong",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := theme.Build(tc.file)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("ошибка %v, ожидалась с текстом %q", err, tc.wantErr)
				}
				if got != dark {
					t.Fatalf("при ошибке тема %+v, ожидалась dark", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ошибка %v, ожидалась без ошибки", err)
			}
			tc.check(t, got)
		})
	}
}

func TestLoadByName(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "themes"), 0o755); err != nil {
		t.Fatal(err)
	}
	midnight := `{"colors":{"accent":"#123456"}}`
	if err := os.WriteFile(filepath.Join(dir, "themes", "midnight.json"), []byte(midnight), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := theme.Load(dir, "midnight")
	if err != nil {
		t.Fatalf("Load midnight: %v", err)
	}
	if got.Accent != lipgloss.Color("#123456") {
		t.Fatalf("accent %v, ожидался #123456", got.Accent)
	}

	if got, err := theme.Load(dir, "dark"); err != nil || got != theme.Dark() {
		t.Fatalf("dark без файла: %v, %+v", err, got)
	}

	if _, err := theme.Load(dir, "ghost"); err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("тема без файла: ошибка %v, ожидалась с именем ghost", err)
	}
}

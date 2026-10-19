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
					t.Errorf("Build(file) = %+v, want dark", got)
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
					t.Errorf("Build(file).DiffAdded = %v, want #3fb950", got.DiffAdded)
				}
				if got.DiffRemoved != dark.DiffRemoved {
					t.Errorf("Build(file).DiffRemoved = %v, want как в dark %v", got.DiffRemoved, dark.DiffRemoved)
				}
			},
		},
		{
			name: "ключ worktree перекрашивает роль",
			file: config.ThemeFile{Colors: map[string]config.Color{"worktree": "#ff0000"}},
			check: func(t *testing.T, got theme.Theme) {
				if got.Worktree != lipgloss.Color("#ff0000") {
					t.Errorf("Build(file).Worktree = %v, want #ff0000", got.Worktree)
				}
			},
		},
		{
			name: "без ключа worktree роль жёлтая",
			file: config.ThemeFile{Colors: map[string]config.Color{"accent": "#123456"}},
			check: func(t *testing.T, got theme.Theme) {
				if got.Worktree != lipgloss.Color("#e5c07b") {
					t.Errorf("Build(file).Worktree = %v, want жёлтый #e5c07b", got.Worktree)
				}
			},
		},
		{
			name: "число как ANSI",
			file: config.ThemeFile{Colors: map[string]config.Color{"textMuted": "8"}},
			check: func(t *testing.T, got theme.Theme) {
				if got.TextMuted != lipgloss.Color("8") {
					t.Errorf("Build(file).TextMuted = %v, want ANSI 8", got.TextMuted)
				}
			},
		},
		{
			name: "255 последний цвет палитры",
			file: config.ThemeFile{Colors: map[string]config.Color{"textMuted": "255"}},
			check: func(t *testing.T, got theme.Theme) {
				if got.TextMuted != lipgloss.Color("255") {
					t.Errorf("Build(file).TextMuted = %v, want ANSI 255", got.TextMuted)
				}
			},
		},
		{
			name:    "256 за палитрой",
			file:    config.ThemeFile{Colors: map[string]config.Color{"textMuted": "256"}},
			wantErr: "256",
		},
		{
			name: "none это цвет терминала",
			file: config.ThemeFile{Colors: map[string]config.Color{"backgroundPanel": "none"}},
			check: func(t *testing.T, got theme.Theme) {
				if got.BackgroundPanel != (lipgloss.NoColor{}) {
					t.Errorf("Build(file).BackgroundPanel = %v, want NoColor", got.BackgroundPanel)
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
					t.Errorf("Build(file) error = %v, want с текстом %q", err, tc.wantErr)
				}
				if got != dark {
					t.Errorf("Build(file) при ошибке = %+v, want dark", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Build(file) error = %v, want nil", err)
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

package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/config"
)

func writeTheme(t *testing.T, dir, name, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(dir, "themes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "themes", name+".json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadTheme(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content string
		missing bool
		wantOK  bool
		// подстрока ошибки, пусто значит ошибки нет
		wantErr string
		check   func(t *testing.T, file config.ThemeFile)
	}{
		{
			name:    "нет файла",
			file:    "midnight",
			missing: true,
		},
		{
			name:    "строки, числа и ссылки на defs",
			file:    "midnight",
			content: `{"defs":{"green":"#3fb950"},"colors":{"diffAdded":"green","textMuted":8,"accent":"#9d7cd8"}}`,
			wantOK:  true,
			check: func(t *testing.T, file config.ThemeFile) {
				if file.Colors["diffAdded"] != "green" || file.Colors["textMuted"] != "8" || file.Defs["green"] != "#3fb950" {
					t.Errorf("ReadTheme(midnight) = %+v, want colors diffAdded green, textMuted 8 and defs green #3fb950", file)
				}
			},
		},
		{
			name:    "$schema допустим",
			file:    "midnight",
			content: `{"$schema":"./schemas/theme.schema.json","colors":{"accent":"#9d7cd8"}}`,
			wantOK:  true,
		},
		{
			name:    "логическое значение вместо цвета",
			file:    "midnight",
			content: `{"colors":{"accent":true}}`,
			wantErr: "colors.accent: a color must be a string or a number",
		},
		{
			name:    "логическое значение вместо цвета в defs",
			file:    "midnight",
			content: `{"defs":{"x":true}}`,
			wantErr: "defs.x: a color must be a string or a number",
		},
		{
			name:    "неизвестное поле верхнего уровня",
			file:    "midnight",
			content: `{"colour":{}}`,
			wantErr: "colour",
		},
		{
			name:    "не JSON",
			file:    "midnight",
			content: `{`,
			wantErr: "themes/midnight.json",
		},
		{
			name:    "путь в имени темы",
			file:    "../midnight",
			content: `{}`,
			wantErr: "contain a path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tc.missing {
				writeTheme(t, dir, filepath.Base(tc.file), tc.content)
			}

			file, found, err := config.ReadTheme(dir, tc.file)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("ReadTheme(%q) error = %v, want error with %q", tc.file, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReadTheme(%q) error = %v, want nil", tc.file, err)
			}
			if found != tc.wantOK {
				t.Errorf("ReadTheme(%q) found = %v, want %v", tc.file, found, tc.wantOK)
			}
			if tc.check != nil {
				tc.check(t, file)
			}
		})
	}
}

func TestConfigReadsThemeName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"theme":"midnight"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Theme != "midnight" {
		t.Errorf("тема %q, ожидалась midnight", cfg.Theme)
	}
	if config.Default().Theme != "dark" {
		t.Errorf("тема по умолчанию %q, ожидалась dark", config.Default().Theme)
	}
}

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		check   func(t *testing.T, file ThemeFile)
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
			check: func(t *testing.T, file ThemeFile) {
				if file.Colors["diffAdded"] != "green" || file.Colors["textMuted"] != "8" || file.Defs["green"] != "#3fb950" {
					t.Fatalf("значения %+v", file)
				}
			},
		},
		{
			name:    "логическое значение вместо цвета",
			file:    "midnight",
			content: `{"colors":{"accent":true}}`,
			wantErr: "accent",
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
			wantErr: "путь",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tc.missing {
				writeTheme(t, dir, filepath.Base(tc.file), tc.content)
			}

			file, found, err := ReadTheme(dir, tc.file)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("ошибка %v, ожидалась с текстом %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ошибка %v, ожидалась без ошибки", err)
			}
			if found != tc.wantOK {
				t.Fatalf("найден %v, ожидалось %v", found, tc.wantOK)
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

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Theme != "midnight" {
		t.Fatalf("тема %q, ожидалась midnight", cfg.Theme)
	}
	if Default().Theme != "dark" {
		t.Fatalf("тема по умолчанию %q, ожидалась dark", Default().Theme)
	}
}

package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/config"
)

func writeLayout(t *testing.T, dir, name, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(dir, "layouts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "layouts", name+".json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadLayout(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content string
		missing bool
		wantOK  bool
		// подстрока ошибки, пусто значит ошибки нет
		wantErr string
		check   func(t *testing.T, file config.LayoutFile)
	}{
		{name: "нет файла", file: "compact", missing: true},
		{
			name:    "дерево с процентом и числом",
			file:    "compact",
			content: `{"root":{"direction":"row","children":[{"pane":"files","size":"30%"},{"pane":"diff","size":6}]}}`,
			wantOK:  true,
			check: func(t *testing.T, file config.LayoutFile) {
				if file.Root.Direction != "row" || len(file.Root.Children) != 2 {
					t.Fatalf("ReadLayout(compact).Root = %+v, want row with 2 children", file.Root)
				}
				if file.Root.Children[0].Size != "30%" || file.Root.Children[1].Size != "6" {
					t.Errorf("ReadLayout(compact) sizes = %q, %q, want %q, %q", file.Root.Children[0].Size, file.Root.Children[1].Size, "30%", "6")
				}
			},
		},
		{
			name:    "размер не строка и не число",
			file:    "compact",
			content: `{"root":{"pane":"diff","size":true}}`,
			wantErr: "size",
		},
		{
			name:    "неизвестное поле узла",
			file:    "compact",
			content: `{"root":{"pane":"diff","colour":"red"}}`,
			wantErr: "colour",
		},
		{
			name:    "путь в имени раскладки",
			file:    "../compact",
			content: `{}`,
			wantErr: "contain a path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tc.missing {
				writeLayout(t, dir, filepath.Base(tc.file), tc.content)
			}

			file, found, err := config.ReadLayout(dir, tc.file)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("ReadLayout(%q) error = %v, want error with %q", tc.file, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReadLayout(%q) error = %v, want nil", tc.file, err)
			}
			if found != tc.wantOK {
				t.Errorf("ReadLayout(%q) found = %v, want %v", tc.file, found, tc.wantOK)
			}
			if tc.check != nil {
				tc.check(t, file)
			}
		})
	}
}

func TestConfigReadsLayoutName(t *testing.T) {
	if config.Default().Layout != "default" {
		t.Errorf("раскладка по умолчанию %q, ожидалась default", config.Default().Layout)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"layout":"focus"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil || cfg.Layout != "focus" {
		t.Errorf("раскладка %q, ошибка %v", cfg.Layout, err)
	}
}

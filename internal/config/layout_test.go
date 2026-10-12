package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		check   func(t *testing.T, file LayoutFile)
	}{
		{name: "нет файла", file: "compact", missing: true},
		{
			name:    "дерево с процентом и числом",
			file:    "compact",
			content: `{"root":{"direction":"row","children":[{"pane":"files","size":"30%"},{"pane":"diff","size":6}]}}`,
			wantOK:  true,
			check: func(t *testing.T, file LayoutFile) {
				if file.Root.Direction != "row" || len(file.Root.Children) != 2 {
					t.Fatalf("дерево %+v", file.Root)
				}
				if file.Root.Children[0].Size != "30%" || file.Root.Children[1].Size != "6" {
					t.Fatalf("размеры %q и %q", file.Root.Children[0].Size, file.Root.Children[1].Size)
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
			wantErr: "путь",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tc.missing {
				writeLayout(t, dir, filepath.Base(tc.file), tc.content)
			}

			file, found, err := ReadLayout(dir, tc.file)

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
				t.Fatalf("найдена %v, ожидалось %v", found, tc.wantOK)
			}
			if tc.check != nil {
				tc.check(t, file)
			}
		})
	}
}

func TestConfigReadsLayoutName(t *testing.T) {
	if Default().Layout != "default" {
		t.Fatalf("раскладка по умолчанию %q, ожидалась default", Default().Layout)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"layout":"focus"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil || cfg.Layout != "focus" {
		t.Fatalf("раскладка %q, ошибка %v", cfg.Layout, err)
	}
}

package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/config"
)

func TestLoad(t *testing.T) {
	withUI := func(ui config.UI) config.Config {
		cfg := config.Default()
		cfg.UI = ui
		return cfg
	}
	withMode := func(mode string) config.Config {
		cfg := config.Default()
		cfg.Diff.Mode = mode
		return cfg
	}

	tests := []struct {
		name    string
		missing bool
		content string
		want    config.Config
		// подстрока ошибки, пусто значит ошибки нет
		wantErr string
	}{
		{name: "нет файла", missing: true, want: config.Default()},
		{name: "режим из файла", content: `{"diff":{"mode":"split"}}`, want: withMode("split")},
		// перенос включён по умолчанию, файл его выключает
		{name: "без переноса строк", content: `{"diff":{"wrap":false}}`, want: func() config.Config {
			cfg := config.Default()
			cfg.Diff.Wrap = false
			return cfg
		}()},
		// файл с ошибкой отбрасывается целиком: рядом с ошибкой стоят верные
		// ключи, и ни один из них не должен дойти до результата
		{name: "неизвестное поле", content: `{"theme":"midnight","ui":{"keyHints":false},"diff":{"thme":"x"}}`, want: config.Default(), wantErr: "thme"},
		{name: "неверный тип", content: `{"theme":"midnight","diff":{"autoSplitMinWidth":"много"}}`, want: config.Default(), wantErr: "autoSplitMinWidth"},
		{name: "неверный режим", content: `{"theme":"midnight","diff":{"mode":"wide"}}`, want: config.Default(), wantErr: "wide"},
		{name: "не JSON", content: `{"theme":"midnight","diff":`, want: config.Default(), wantErr: "config.json"},
		{name: "текст после JSON", content: `{"theme":"midnight"} {}`, want: config.Default(), wantErr: "config.json"},
		{name: "ui: одно поле", content: `{"ui":{"paneTitles":false}}`, want: withUI(config.UI{KeyHints: true, PaneTitles: false, LineNumbers: true, AutoReload: true})},
		{name: "ui: autoReload выключен", content: `{"ui":{"autoReload":false}}`, want: withUI(config.UI{KeyHints: true, PaneTitles: true, LineNumbers: true, AutoReload: false})},
		{name: "ui: scrollbar пока не принимается", content: `{"ui":{"scrollbar":true}}`, want: config.Default(), wantErr: "scrollbar"},
		{name: "ui: statusBar пока не принимается", content: `{"ui":{"statusBar":false}}`, want: config.Default(), wantErr: "statusBar"},
		// язык проверяет i18n, он знает словари: неизвестный язык не сбрасывает
		// остальные настройки файла
		{name: "язык без проверки", content: `{"language":"de","theme":"midnight"}`, want: config.Config{
			Language: "de",
			Theme:    "midnight",
			Layout:   "default",
			UI:       config.UI{KeyHints: true, PaneTitles: true, LineNumbers: true, AutoReload: true},
			Diff:     config.Diff{Mode: "auto", AutoSplitMinWidth: 120, Wrap: true},
		}},
		{name: "$schema допустим", content: `{"$schema":"./schemas/config.schema.json","diff":{"mode":"unified"}}`, want: config.Config{
			Schema: "./schemas/config.schema.json",
			Theme:  "dark",
			Layout: "default",
			UI:     config.UI{KeyHints: true, PaneTitles: true, LineNumbers: true, AutoReload: true},
			Diff:   config.Diff{Mode: "unified", AutoSplitMinWidth: 120, Wrap: true},
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tc.missing {
				if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(tc.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			got, err := config.Load(dir)

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Load(config.json %q) error = %v, want nil", tc.content, err)
				}
			} else {
				if err == nil {
					t.Fatalf("Load(config.json %q) error = nil, want error with %q", tc.content, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "config.json") {
					t.Errorf("Load(config.json %q) error = %q, want error with config.json and %q", tc.content, err, tc.wantErr)
				}
			}

			if got != tc.want {
				t.Errorf("Load(config.json %q) = %+v, want %+v", tc.content, got, tc.want)
			}
		})
	}
}

// Битая ссылка (dotfiles через stow, перенесённый репозиторий) не то же, что
// отсутствующий файл: настройки пользователя есть, но не читаются, и молча
// подменять их встроенными нельзя.
func TestBrokenSymlinkIsError(t *testing.T) {
	tests := []struct {
		name string
		// путь ссылки от каталога настроек, он же префикс ошибки
		link string
		read func(dir string) error
	}{
		{name: "config.json", link: "config.json", read: func(dir string) error {
			_, err := config.Load(dir)
			return err
		}},
		{name: "тема", link: "themes/midnight.json", read: func(dir string) error {
			_, _, err := config.ReadTheme(dir, "midnight")
			return err
		}},
		{name: "раскладка", link: "layouts/compact.json", read: func(dir string) error {
			_, _, err := config.ReadLayout(dir, "compact")
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			link := filepath.Join(dir, tc.link)
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(dir, "gone.json"), link); err != nil {
				t.Fatal(err)
			}

			err := tc.read(dir)

			if err == nil || !strings.HasPrefix(err.Error(), tc.link) {
				t.Errorf("чтение %s (ссылка на отсутствующий файл) error = %v, want error starting with %q", tc.link, err, tc.link)
			}
		})
	}
}

func TestDir(t *testing.T) {
	tests := []struct {
		name string
		xdg  string
		home string
		want string
	}{
		{name: "XDG задан", xdg: "/x", home: "/h", want: "/x/gitone"},
		{name: "XDG не задан", home: "/h", want: "/h/.config/gitone"},
		// спецификация XDG велит игнорировать относительный путь: от cwd он
		// указал бы на просматриваемый репозиторий
		{name: "XDG относительный", xdg: "cfg", home: "/h", want: "/h/.config/gitone"},
		{name: "XDG с нераскрытой тильдой", xdg: "~/.config", home: "/h", want: "/h/.config/gitone"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := config.Dir(tc.xdg, tc.home); got != tc.want {
				t.Errorf("Dir(%q, %q) = %q, want %q", tc.xdg, tc.home, got, tc.want)
			}
		})
	}
}

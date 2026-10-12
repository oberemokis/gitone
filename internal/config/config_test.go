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
		{name: "неизвестное поле", content: `{"diff":{"thme":"x"}}`, want: config.Default(), wantErr: "thme"},
		{name: "неверный тип", content: `{"diff":{"autoSplitMinWidth":"много"}}`, want: config.Default(), wantErr: "autoSplitMinWidth"},
		{name: "неверный режим", content: `{"diff":{"mode":"wide"}}`, want: config.Default(), wantErr: "wide"},
		{name: "не JSON", content: `{"diff":`, want: config.Default(), wantErr: "config.json"},
		{name: "текст после JSON", content: `{} {}`, want: config.Default(), wantErr: "config.json"},
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
			Diff:     config.Diff{Mode: "auto", AutoSplitMinWidth: 120},
		}},
		{name: "$schema допустим", content: `{"$schema":"./schemas/config.schema.json","diff":{"mode":"unified"}}`, want: config.Config{
			Schema: "./schemas/config.schema.json",
			Theme:  "dark",
			Layout: "default",
			UI:     config.UI{KeyHints: true, PaneTitles: true, LineNumbers: true, AutoReload: true},
			Diff:   config.Diff{Mode: "unified", AutoSplitMinWidth: 120},
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

func TestDir(t *testing.T) {
	tests := []struct {
		name string
		xdg  string
		home string
		want string
	}{
		{name: "XDG задан", xdg: "/x", home: "/h", want: "/x/gitone"},
		{name: "XDG не задан", home: "/h", want: "/h/.config/gitone"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := config.Dir(tc.xdg, tc.home); got != tc.want {
				t.Errorf("Dir(%q, %q) = %q, want %q", tc.xdg, tc.home, got, tc.want)
			}
		})
	}
}

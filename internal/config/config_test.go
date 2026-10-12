package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	withUI := func(ui UI) Config {
		cfg := Default()
		cfg.UI = ui
		return cfg
	}
	withMode := func(mode string) Config {
		cfg := Default()
		cfg.Diff.Mode = mode
		return cfg
	}

	tests := []struct {
		name    string
		missing bool
		content string
		want    Config
		// подстрока ошибки, пусто значит ошибки нет
		wantErr string
	}{
		{name: "нет файла", missing: true, want: Default()},
		{name: "режим из файла", content: `{"diff":{"mode":"split"}}`, want: withMode("split")},
		{name: "неизвестное поле", content: `{"diff":{"thme":"x"}}`, want: Default(), wantErr: "thme"},
		{name: "неверный тип", content: `{"diff":{"autoSplitMinWidth":"много"}}`, want: Default(), wantErr: "autoSplitMinWidth"},
		{name: "неверный режим", content: `{"diff":{"mode":"wide"}}`, want: Default(), wantErr: "wide"},
		{name: "не JSON", content: `{"diff":`, want: Default(), wantErr: "config.json"},
		{name: "текст после JSON", content: `{} {}`, want: Default(), wantErr: "config.json"},
		{name: "ui: одно поле", content: `{"ui":{"paneTitles":false}}`, want: withUI(UI{KeyHints: true, PaneTitles: false, LineNumbers: true, AutoReload: true})},
		{name: "ui: autoReload выключен", content: `{"ui":{"autoReload":false}}`, want: withUI(UI{KeyHints: true, PaneTitles: true, LineNumbers: true, AutoReload: false})},
		{name: "ui: scrollbar пока не принимается", content: `{"ui":{"scrollbar":true}}`, want: Default(), wantErr: "scrollbar"},
		{name: "ui: statusBar пока не принимается", content: `{"ui":{"statusBar":false}}`, want: Default(), wantErr: "statusBar"},
		{name: "$schema допустим", content: `{"$schema":"./schemas/config.schema.json","diff":{"mode":"unified"}}`, want: Config{
			Schema: "./schemas/config.schema.json",
			Theme:  "dark",
			Layout: "default",
			UI:     UI{KeyHints: true, PaneTitles: true, LineNumbers: true, AutoReload: true},
			Diff:   Diff{Mode: "unified", AutoSplitMinWidth: 120},
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

			got, err := Load(dir)

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ошибка %v, ожидалась без ошибки", err)
				}
			} else {
				if err == nil {
					t.Fatalf("ошибки нет, ожидалась с текстом %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "config.json") {
					t.Fatalf("ошибка %q не называет файл и %q", err, tc.wantErr)
				}
			}

			if got != tc.want {
				t.Fatalf("значения %+v, ожидались %+v", got, tc.want)
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
			if got := Dir(tc.xdg, tc.home); got != tc.want {
				t.Fatalf("каталог %q, ожидался %q", got, tc.want)
			}
		})
	}
}

package main

import (
	"errors"
	"fmt"
	"testing"

	"gitone/internal/i18n"
)

func TestAddNotice(t *testing.T) {
	text := i18n.Custom("xx", map[string]string{"json.extraText": "лишний текст"})

	tests := []struct {
		name   string
		notice string
		errs   []error
		want   string
	}{
		{name: "нет ошибок", notice: "", errs: []error{nil, nil}, want: ""},
		{name: "nil пропускается", notice: "", errs: []error{nil, errors.New("тема"), nil}, want: "тема"},
		{name: "к тексту через точку с запятой", notice: "config.json", errs: []error{errors.New("раскладка")}, want: "config.json; раскладка"},
		{name: "текст без ошибок остаётся", notice: "config.json", errs: []error{nil}, want: "config.json"},
		{name: "своя ошибка на языке словаря", errs: []error{fmt.Errorf("config.json: %w", i18n.New("json.extraText", nil))}, want: "config.json: лишний текст"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := addNotice(text, tc.notice, tc.errs...); got != tc.want {
				t.Errorf("addNotice(%q, %v) = %q, want %q", tc.notice, tc.errs, got, tc.want)
			}
		})
	}
}

package main

import (
	"errors"
	"testing"
)

func TestAddNotice(t *testing.T) {
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := addNotice(tc.notice, tc.errs...); got != tc.want {
				t.Errorf("addNotice(%q, %v) = %q, want %q", tc.notice, tc.errs, got, tc.want)
			}
		})
	}
}

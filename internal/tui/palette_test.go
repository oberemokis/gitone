package tui_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

// Счётчики в заголовке красятся цветами темы: «+2» цветом добавления, «-1»
// цветом удаления, и цвет стоит прямо перед счётчиком в строке заголовка. Цвет
// добавления несут ещё номер и знак добавленной строки, поэтому поиск по всему
// выводу счётчик не проверял бы. #123456 в ANSI это 38;2;18;52;86, #654321 это
// 38;2;101;67;33.
func TestHeaderCountersUseChangeColors(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "uno\ntwo\n")

	custom := theme.Dark()
	custom.DiffAdded = lipgloss.Color("#123456")
	custom.DiffRemoved = lipgloss.Color("#654321")

	opts := options(repo, layout.Focus())
	opts.Theme = custom
	raw := resize(open(t, opts), 100, 20).View().Content

	tests := []struct{ counter, want string }{
		{counter: "+2", want: "38;2;18;52;86"},
		{counter: "-1", want: "38;2;101;67;33"},
	}

	for _, tc := range tests {
		if got := styleBefore(t, raw, "(1/1)", tc.counter); !strings.Contains(got, tc.want) {
			t.Errorf("перед счётчиком %s в заголовке %q, want цвет %s", tc.counter, got, tc.want)
		}
	}
}

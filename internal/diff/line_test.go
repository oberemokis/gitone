package diff_test

import (
	"testing"

	"gitone/internal/diff"
)

// Printable идёт на экран вместо имени файла: ESC, BEL, CR и LF из имени
// иначе управляли бы терминалом.
func TestPrintable(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "обычное имя без изменений", text: "internal/файл.go", want: "internal/файл.go"},
		{name: "управляющие символы становятся значками", text: "a\x1b]0;x\x07\r\n\x7fb", want: "a␛]0;x␇␍␊␡b"},
		{name: "символ C1 и битый байт становятся U+FFFD", text: "a\u009bb\xffc", want: "a�b�c"},
		{name: "таб остаётся", text: "a\tb", want: "a\tb"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := diff.Printable(tc.text); got != tc.want {
				t.Errorf("Printable(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

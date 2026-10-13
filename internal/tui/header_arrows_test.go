package tui_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"gitone/internal/layout"
	"gitone/internal/tui/theme"
)

// Стрелки стоят у краёв строки с именем файла, с полем в одну колонку, как
// подвал (#120). Рядом с ними клавиши действий: слева от ‹ первая клавиша
// «предыдущего файла», справа от › «следующего» (#195).
func TestHeaderArrowsAtEdges(t *testing.T) {
	_, model := threeFileRepo(t)
	model, _ = model.Update(key(']'))

	header := screenOf(model)[headerRow]
	if want := regexp.MustCompile(`^ \[ ‹ +b\.txt \+1 -1 \(2/3\) +› \] $`); !want.MatchString(header) {
		t.Fatalf("заголовок %q, want [ ‹ имя › ] с полем в одну колонку", header)
	}
	if width := ansi.StringWidth(header); width != 100 {
		t.Fatalf("заголовок шириной %d, want 100: %q", width, header)
	}
}

// Клавиша рядом со стрелкой берётся из keymap: переназначенная читается как в
// подвале («ctrl h»), отключённая оставляет стрелку без подписи.
func TestHeaderKeysFollowKeymap(t *testing.T) {
	tests := []struct {
		name     string
		keybinds string
		want     string
	}{
		{"переназначена", `{"keybinds":{"files.previous":"ctrl+h"}}`, `^ ctrl h ‹ +b\.txt \+1 -1 \(2/3\) +› \] $`},
		{"предыдущий отключён", `{"keybinds":{"files.previous":"none"}}`, `^ ‹ +b\.txt \+1 -1 \(2/3\) +› \] $`},
		{"следующий отключён", `{"keybinds":{"files.next":"none"}}`, `^ \[ ‹ +b\.txt \+1 -1 \(2/3\) +› $`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo, _ := threeFileRepo(t)
			model := openWithKeybinds(t, repo, test.keybinds, 100)
			// второй файл: клик по стрелке работает и при отключённой клавише
			header := screenOf(clickOn(t, model, "›"))[headerRow]
			if !regexp.MustCompile(test.want).MatchString(header) {
				t.Fatalf("заголовок %q, want по образцу %s", header, test.want)
			}
		})
	}
}

// Клик по клавише рядом со стрелкой открывает файл так же, как клик по
// стрелке: клавиша и стрелка одно место нажатия.
func TestClickHeaderKeyOpensFile(t *testing.T) {
	_, model := threeFileRepo(t)
	model, _ = model.Update(key(']'))

	if header := screenOf(clickOn(t, model, "]"))[headerRow]; !strings.Contains(header, "c.txt") || !strings.Contains(header, "(3/3)") {
		t.Fatalf("клик по ] не открыл третий файл: %q", header)
	}
	if header := screenOf(clickOn(t, model, "["))[headerRow]; !strings.Contains(header, "a.txt") || !strings.Contains(header, "(1/3)") {
		t.Fatalf("клик по [ не открыл первый файл: %q", header)
	}
}

// Клавиша приглушена, как подпись подсказки в подвале, а стрелка нет. У края
// списка приглушена и стрелка, и клик по клавише ничего не меняет.
func TestHeaderKeysMutedAtEnds(t *testing.T) {
	repo, _ := threeFileRepo(t)
	custom := theme.Dark()
	custom.TextMuted = lipgloss.Color("#030303")
	opts := options(repo, layout.Focus())
	opts.Theme = custom
	opts.Keys = loadKeys(t, `{"keybinds":{"files.previous":"ctrl+h","files.next":"ctrl+n"}}`)
	model := resize(open(t, opts), 100, 20)

	raw := model.View().Content
	for _, muted := range []string{"ctrl h", "‹", "ctrl n"} {
		if code := styleBefore(t, raw, "(1/3)", muted); !strings.Contains(code, mutedFg) {
			t.Errorf("%q на первом файле не приглушена: %q", muted, code)
		}
	}
	if code := styleBefore(t, raw, "(1/3)", "›"); strings.Contains(code, mutedFg) {
		t.Errorf("› на первом файле приглушена: %q", code)
	}
	if header := screenOf(clickOn(t, model, "ctrl h"))[headerRow]; !strings.Contains(header, "(1/3)") {
		t.Fatalf("клик по клавише у края сменил файл: %q", header)
	}
}

// На узком заголовке место отдаётся имени: клавиши пропадают, когда имя со
// счётчиками помещается без них, но не с ними. Совсем узкий экран обрезает
// имя «…», стрелки остаются, строка не шире экрана.
func TestHeaderNarrowDropsKeysFirst(t *testing.T) {
	tests := []struct {
		width int
		want  string
	}{
		{27, `^ \[ ‹ a\.txt \+1 -1 \(1/3\) › \] $`},
		{26, `^ ‹ +a\.txt \+1 -1 \(1/3\) +› $`},
		{15, `^ ‹ … \+1 -1 … › $`},
	}
	for _, test := range tests {
		t.Run(strconv.Itoa(test.width), func(t *testing.T) {
			_, model := threeFileRepo(t)

			header := render(model, test.width, 20)[headerRow]
			if !regexp.MustCompile(test.want).MatchString(header) {
				t.Fatalf("заголовок %q, want по образцу %s", header, test.want)
			}
			if width := ansi.StringWidth(header); width != test.width {
				t.Fatalf("заголовок шириной %d, want %d: %q", width, test.width, header)
			}
		})
	}
}

// Со второго из трёх файлов › открывает третий, ‹ первый.
func TestHeaderArrowsSwitchFiles(t *testing.T) {
	_, model := threeFileRepo(t)
	model, _ = model.Update(key(']'))

	if header := screenOf(clickOn(t, model, "›"))[headerRow]; !strings.Contains(header, "c.txt") || !strings.Contains(header, "(3/3)") {
		t.Fatalf("› не открыла третий файл: %q", header)
	}
	if header := screenOf(clickOn(t, model, "‹"))[headerRow]; !strings.Contains(header, "a.txt") || !strings.Contains(header, "(1/3)") {
		t.Fatalf("‹ не открыла первый файл: %q", header)
	}
}

// На первом файле ‹ приглушена, и клик по ней ничего не меняет; на последнем
// так же ›.
func TestHeaderArrowsMutedAtEnds(t *testing.T) {
	_, model := threeFileRepo(t)

	if code := styleBefore(t, model.View().Content, "(1/3)", "‹"); !strings.Contains(code, "90") {
		t.Errorf("‹ на первом файле не приглушена: %q", code)
	}
	if code := styleBefore(t, model.View().Content, "(1/3)", "›"); strings.Contains(code, "90") {
		t.Errorf("› на первом файле приглушена: %q", code)
	}
	if header := screenOf(clickOn(t, model, "‹"))[headerRow]; !strings.Contains(header, "(1/3)") {
		t.Fatalf("‹ на первом файле сменила файл: %q", header)
	}

	model, _ = model.Update(key(']'))
	model, _ = model.Update(key(']'))
	if code := styleBefore(t, model.View().Content, "(3/3)", "›"); !strings.Contains(code, "90") {
		t.Errorf("› на последнем файле не приглушена: %q", code)
	}
}

// Длинное имя обрезается «…» и не наезжает на стрелки.
func TestHeaderLongNameKeepsArrows(t *testing.T) {
	repo := newRepo(t)
	name := strings.Repeat("long-directory-name/", 8) + "file.txt"
	write(t, repo, name, "one\n")

	model := resize(open(t, options(repo, layout.Focus())), 100, 20)

	header := []rune(screenOf(model)[headerRow])
	if header[1] != '‹' || header[98] != '›' || !strings.Contains(string(header), "…") {
		t.Fatalf("длинное имя наехало на стрелки: %q", string(header))
	}
}

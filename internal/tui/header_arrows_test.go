package tui_test

import (
	"strings"
	"testing"

	"gitone/internal/layout"
)

// Стрелки стоят у краёв строки с именем файла: ‹ во второй колонке, › в
// предпоследней, с полем в одну колонку, как подвал (#120).
func TestHeaderArrowsAtEdges(t *testing.T) {
	_, model := threeFileRepo(t)

	header := []rune(screenOf(model)[headerRow])
	if len(header) != 100 || header[1] != '‹' || header[98] != '›' {
		t.Fatalf("стрелок нет у краёв: %q", string(header))
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

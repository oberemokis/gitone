package tui_test

import (
	"strings"
	"testing"

	"gitone/internal/layout"
)

// longLineScreen открывает дифф, где x заменён строкой из count букв Q, на
// экране 100 на 60: перенос по умолчанию раскладывает строку на экран целиком.
// Q нет ни в заголовке, ни в подвале, поэтому буквы на экране считаются
// напрямую.
func longLineScreen(t *testing.T, count int) string {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "x\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", strings.Repeat("Q", count)+"\n")

	opts := options(repo, layout.Focus())
	opts.Config.Diff.Mode = "unified"
	return screenText(resize(open(t, opts), 100, 60))
}

func TestLineOf3000CharactersIsFullyVisible(t *testing.T) {
	body := longLineScreen(t, 3000)

	if got := strings.Count(body, "Q"); got != 3000 {
		t.Errorf("на экране %d букв Q, want 3000", got)
	}
	if strings.Contains(body, "…") {
		t.Errorf("строка в пределах длины обрезана:\n%s", body)
	}
}

func TestLineOf5000CharactersIsCutWithEllipsis(t *testing.T) {
	body := longLineScreen(t, 5000)

	if got := strings.Count(body, "Q"); got != 4095 {
		t.Errorf("на экране %d букв Q, want 4095", got)
	}
	if got := strings.Count(body, "…"); got != 1 {
		t.Errorf("значков «…» %d, want 1:\n%s", got, body)
	}
}

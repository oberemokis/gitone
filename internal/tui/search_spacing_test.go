package tui_test

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// windowText это часть строки экрана внутри окна: от начала его заголовка
// до конца «close». По краям окна лога виден дифф под ним.
func windowText(screen []string, row int, title string) string {
	header := screen[slices.IndexFunc(screen, func(line string) bool { return strings.Contains(line, title) })]
	start, end := column(header, title), column(header, "close")+len("close")
	return strings.TrimSpace(string([]rune(screen[row])[start:end]))
}

// rowOf это номер первой строки экрана с text.
func rowOf(screen []string, text string) int {
	return slices.IndexFunc(screen, func(line string) bool { return strings.Contains(line, text) })
}

// Над строкой поиска и под ней пустые строки окна, дальше заголовки столбцов
// (#133).
func TestLogSearchHasSpaceAround(t *testing.T) {
	_, model := openLog(t, 3, 120, 40)
	screen := screenOf(typeText(model, "/"))

	search := rowOf(screen, "/ ")
	if title := rowOf(screen, "Log"); search != title+2 || windowText(screen, title+1, "Log") != "" {
		t.Fatalf("над поиском нет пустой строки:\n%s", strings.Join(screen, "\n"))
	}
	if windowText(screen, search+1, "Log") != "" || !strings.Contains(screen[search+2], "Commit") {
		t.Fatalf("под поиском нет пустой строки перед заголовками:\n%s", strings.Join(screen, "\n"))
	}
}

// В окне веток без заголовков-подсказок пустая строка стоит между поиском и
// заголовками столбцов.
func TestBranchSearchHasSpaceBelow(t *testing.T) {
	_, model := branchesWith(t, "feature")
	screen := screenOf(typeText(model, "/"))

	search := rowOf(screen, "/ ")
	if windowText(screen, search+1, "Branches") != "" || !strings.Contains(screen[search+2], "Branch") {
		t.Fatalf("под поиском нет пустой строки:\n%s", strings.Join(screen, "\n"))
	}
}

// Окно лога на весь экран с поиском той же высоты: подсказка внизу окна на
// той же строке.
func TestLogSearchKeepsWindowHeight(t *testing.T) {
	_, model := openLog(t, 40, 120, 40)
	before := rowOf(screenOf(model), "commit diff")

	var searching tea.Model = typeText(model, "/")
	if after := rowOf(screenOf(searching), "esc clear"); after != before {
		t.Fatalf("подсказка окна на строке %d, без поиска на %d", after, before)
	}
}

package gitone

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// Компилятор запрещает только циклы, импорт сбоку он пропустит. Таблица
// повторяет граф из docs/architecture.md. Пакет, которого в ней нет, тоже
// ошибка: новый пакет сначала получает место в графе.
var allowedImports = map[string][]string{
	"gitone":            {},
	"gitone/cmd/gitone": {"gitone/internal/tui"},
	"gitone/internal/tui": {
		"gitone/internal/git",
		"gitone/internal/diff",
		"gitone/internal/tui/diffview",
		"gitone/internal/tui/theme",
	},
	"gitone/internal/tui/diffview": {"gitone/internal/diff", "gitone/internal/tui/theme"},
	"gitone/internal/tui/theme":    {},
	"gitone/internal/config":       {},
	"gitone/internal/git":          {},
	"gitone/internal/diff":         {},
}

func TestImportDirections(t *testing.T) {
	out, err := exec.Command("go", "list", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}

	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		pkg, imports := fields[0], fields[1:]

		allowed, known := allowedImports[pkg]
		if !known {
			t.Errorf("%s: пакета нет в allowedImports", pkg)
			continue
		}

		for _, imported := range imports {
			if strings.HasPrefix(imported, "gitone/") && !slices.Contains(allowed, imported) {
				t.Errorf("%s импортирует %s, граф это запрещает", pkg, imported)
			}
		}
	}
}

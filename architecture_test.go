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
	"gitone/cmd/gitone": {"gitone/internal/tui", "gitone/internal/config", "gitone/internal/i18n", "gitone/internal/keymap", "gitone/internal/tui/theme", "gitone/internal/layout"},
	"gitone/internal/tui": {
		"gitone/internal/git",
		"gitone/internal/diff",
		"gitone/internal/config",
		"gitone/internal/i18n",
		"gitone/internal/keymap",
		"gitone/internal/layout",
		"gitone/internal/tui/diffview",
		"gitone/internal/tui/editor",
		"gitone/internal/tui/pane/files",
		"gitone/internal/tui/picker",
		"gitone/internal/tui/theme",
		"gitone/internal/tui/zone",
	},
	"gitone/internal/tui/diffview":   {"gitone/internal/diff", "gitone/internal/i18n", "gitone/internal/syntax", "gitone/internal/tui/theme"},
	"gitone/internal/tui/editor":     {},
	"gitone/internal/tui/pane/files": {"gitone/internal/tui/theme", "gitone/internal/tui/zone"},
	"gitone/internal/tui/picker":     {},
	"gitone/internal/tui/theme":      {"gitone/internal/config", "gitone/internal/i18n"},
	"gitone/internal/tui/zone":       {},
	"gitone/internal/layout":         {"gitone/internal/config", "gitone/internal/i18n"},
	"gitone/internal/config":         {"gitone/internal/i18n"},
	"gitone/internal/i18n":           {},
	"gitone/internal/keymap":         {"gitone/internal/i18n"},
	"gitone/internal/syntax":         {},
	"gitone/internal/git":            {"gitone/internal/i18n"},
	"gitone/internal/diff":           {},
}

// Листья config, git и diff не знают терминала (docs/architecture.md). Таблица
// выше видит только пакеты gitone, поэтому библиотеки терминала проверяются
// отдельно, по префиксу пути.
var (
	terminalFreeLeaves = []string{"gitone/internal/config", "gitone/internal/git", "gitone/internal/diff"}
	terminalPrefixes   = []string{"charm.land/", "github.com/charmbracelet/"}
)

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

			if slices.Contains(terminalFreeLeaves, pkg) && slices.ContainsFunc(terminalPrefixes, func(prefix string) bool { return strings.HasPrefix(imported, prefix) }) {
				t.Errorf("%s импортирует %s, листья не знают терминала", pkg, imported)
			}
		}
	}
}

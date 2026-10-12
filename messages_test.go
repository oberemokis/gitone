package gitone

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"gitone/internal/diff"
	"gitone/internal/i18n"
)

// sources разбирает Go-файлы программы без тестов: в тестах ключи и тексты
// бывают нарочно чужими.
func sources(t *testing.T) map[string]*ast.File {
	t.Helper()

	files := map[string]*ast.File{}
	fset := token.NewFileSet()
	for _, root := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}

			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			files[path] = file
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	return files
}

// keyLiteral отдаёт ключ словаря, если вызов это Text("ключ", …) у словаря или
// i18n.New("ключ", …).
func keyLiteral(call *ast.CallExpr) (string, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return "", false
	}

	switch selector.Sel.Name {
	case "Text":
	case "New":
		if pkg, ok := selector.X.(*ast.Ident); !ok || pkg.Name != "i18n" {
			return "", false
		}
	default:
		return "", false
	}

	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}

	key, err := strconv.Unquote(literal.Value)
	return key, err == nil
}

// Ключ, которого нет в en.json, вышел бы на экран как есть, например
// log.firstCommitHidden. Ключи keymap и категорий собираются из имён и
// проверяются справкой на псевдословаре в tui, ключи статусов здесь.
func TestMessageKeysExist(t *testing.T) {
	known := map[string]bool{}
	for _, key := range i18n.Keys() {
		known[key] = true
	}

	for path, file := range sources(t) {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if key, found := keyLiteral(call); found && !known[key] {
				t.Errorf("%s: ключа %q нет в en.json", path, key)
			}
			return true
		})
	}

	statuses := []diff.Status{diff.StatusAdded, diff.StatusDeleted, diff.StatusRenamed, diff.StatusCopied, diff.StatusRaw}
	for _, status := range statuses {
		if key := "status." + string(status); !known[key] {
			t.Errorf("ключа %q нет в en.json", key)
		}
	}
}

// Строка с кириллицей в коде это текст мимо словаря: так интерфейс и
// смешивал языки (#64). Комментарии не в счёт, только строковые литералы.
func TestNoCyrillicLiterals(t *testing.T) {
	for path, file := range sources(t) {
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if ok && literal.Kind == token.STRING && strings.ContainsFunc(literal.Value, isCyrillic) {
				t.Errorf("%s: строка с кириллицей %s, текст должен идти через словарь", path, literal.Value)
			}
			return true
		})
	}
}

func isCyrillic(r rune) bool {
	return unicode.Is(unicode.Cyrillic, r)
}

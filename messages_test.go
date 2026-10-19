package gitone

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"gitone/internal/diff"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
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

	// незавершённые операции из git.Location.Operation (#243, #245, #300)
	for _, operation := range []string{"merge", "rebase", "am", "cherry-pick", "revert", "bisect"} {
		if key := "operation." + operation; !known[key] {
			t.Errorf("ключа %q нет в en.json", key)
		}
	}
}

// keyShape это строка, похожая на ключ словаря: слова через точку, или
// приставка с точкой в конце, к которой ключ дописывается («status.» +
// статус). Имя файла настроек (config.json) ключом не считается.
var keyShape = regexp.MustCompile(`^[a-z][a-zA-Z]*((\.[a-zA-Z0-9-]+)+|(\.[a-zA-Z0-9-]+)*\.)$`)

// keyLiterals это строковые литералы программы, похожие на ключ одного из
// семейств en.json, и где они стоят. Ключ бывает не первым аргументом Text:
// его передают помощникам меню (item, option) и возвращают из функций.
// withNames false пропускает keymap и syntax: там строки с точками это ещё и
// имена действий и узлов разбора.
func keyLiterals(t *testing.T, families map[string]bool, withNames bool) map[string]string {
	t.Helper()

	found := map[string]string{}
	for path, file := range sources(t) {
		names := strings.HasPrefix(path, filepath.FromSlash("internal/keymap/")) || strings.HasPrefix(path, filepath.FromSlash("internal/syntax/"))
		if names && !withNames {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			text, err := strconv.Unquote(literal.Value)
			if err == nil && keyShape.MatchString(text) && !strings.HasSuffix(text, ".json") && families[strings.SplitN(text, ".", 2)[0]] {
				found[text] = path
			}
			return true
		})
	}
	return found
}

// keyFamilies это первые слова ключей en.json.
func keyFamilies(keys []string) map[string]bool {
	families := map[string]bool{}
	for _, key := range keys {
		families[strings.SplitN(key, ".", 2)[0]] = true
	}
	return families
}

// Каждый литерал, похожий на ключ словаря, есть в en.json: опечатка в ключе,
// который идёт в помощник меню, иначе проходила все тесты (#313). Приставка
// с точкой в конце должна начинать хоть один ключ.
func TestKeyLiteralsExist(t *testing.T) {
	keys := i18n.Keys()
	for literal, path := range keyLiterals(t, keyFamilies(keys), false) {
		if strings.HasSuffix(literal, ".") {
			if !slices.ContainsFunc(keys, func(key string) bool { return strings.HasPrefix(key, literal) }) {
				t.Errorf("%s: ни один ключ en.json не начинается с %q", path, literal)
			}
			continue
		}
		if !slices.Contains(keys, literal) {
			t.Errorf("%s: ключа %q нет в en.json", path, literal)
		}
	}
}

// Каждый ключ en.json используется: текст, который никто не читает, устаревает
// молча (#313). Ключи действий выводятся из keymap: help у действий справки,
// footer у действий подвала и экрана без изменений, hint у действий из
// подсказок окон; ключи категорий из категорий действий. Остальные ключи
// стоят литералом или начинаются с приставки-литерала.
func TestDictionaryKeysUsed(t *testing.T) {
	keys := i18n.Keys()
	literals := keyLiterals(t, keyFamilies(keys), true)

	used := map[string]bool{}
	for _, binding := range keymap.Default().Bindings() {
		action := string(binding.Action)
		used["category."+string(binding.Category)] = used["category."+string(binding.Category)] || binding.Category != ""
		used["action."+action+".help"] = binding.Help
		used["action."+action+".footer"] = binding.Footer
	}
	for _, action := range tuiActions(t, "windowHint") {
		used["action."+action+".hint"] = true
	}
	for _, action := range tuiActions(t, "emptyActions") {
		used["action."+action+".footer"] = true
	}

	for _, key := range keys {
		if strings.HasPrefix(key, "action.") || strings.HasPrefix(key, "category.") {
			if !used[key] {
				t.Errorf("ключ %q из en.json никто не читает", key)
			}
			continue
		}

		_, literal := literals[key]
		for other := range literals {
			literal = literal || strings.HasSuffix(other, ".") && strings.HasPrefix(key, other)
		}
		if !literal {
			t.Errorf("ключ %q из en.json никто не читает", key)
		}
	}
}

// tuiActions это действия keymap, которые tui передаёт в вызов функции name
// или перечисляет в переменной name, строками имён действий.
func tuiActions(t *testing.T, name string) []string {
	t.Helper()

	values := actionValues(t)
	var actions []string
	collect := func(exprs []ast.Expr) {
		for _, expr := range exprs {
			if selector, ok := expr.(*ast.SelectorExpr); ok {
				if value, found := values[selector.Sel.Name]; found {
					actions = append(actions, value)
				}
			}
		}
	}

	for path, file := range sources(t) {
		if !strings.HasPrefix(path, filepath.FromSlash("internal/tui/")) {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.CallExpr:
				if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == name {
					collect(node.Args)
				}
			case *ast.ValueSpec:
				for index, ident := range node.Names {
					if ident.Name == name && index < len(node.Values) {
						if list, ok := node.Values[index].(*ast.CompositeLit); ok {
							collect(list.Elts)
						}
					}
				}
			}
			return true
		})
	}
	return actions
}

// actionValues это имена констант keymap.Action и их строки.
func actionValues(t *testing.T) map[string]string {
	t.Helper()

	values := map[string]string{}
	for path, file := range sources(t) {
		if !strings.HasPrefix(path, filepath.FromSlash("internal/keymap/")) {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.ValueSpec)
			if !ok || len(spec.Values) != len(spec.Names) {
				return true
			}
			if kind, ok := spec.Type.(*ast.Ident); !ok || kind.Name != "Action" {
				return true
			}
			for index, ident := range spec.Names {
				if literal, ok := spec.Values[index].(*ast.BasicLit); ok {
					values[ident.Name], _ = strconv.Unquote(literal.Value)
				}
			}
			return true
		})
	}
	return values
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

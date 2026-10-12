// Package layout превращает дерево раскладки в размеры панелей. Геометрия здесь,
// отрисовка в tui: пакет ничего не рисует и не читает терминал.
package layout

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gitone/internal/config"
	"gitone/internal/i18n"
)

// Панели первой версии. Список растёт вместе с панелями.
var panes = []string{"files", "diff"}

type Node struct {
	Direction string
	Children  []Node
	Pane      string
	Size      Size
}

// Size это 0 без размера, Percent от оси контейнера или Cells в ячейках.
type Size struct {
	Percent int
	Cells   int
}

// Placed это размер панели после расчёта. Дети идут в том же порядке, что в
// дереве, и делят ось Direction.
type Placed struct {
	Pane      string
	Direction string
	Width     int
	Height    int
	Children  []Placed
}

var percentSize = regexp.MustCompile(`^[0-9]{1,3}%$`)
var cellSize = regexp.MustCompile(`^[0-9]+$`)

// Build проверяет дерево из файла и переводит его в Node. Ошибка называет путь
// внутри файла, например root.children[1].pane.
func Build(file config.LayoutFile) (Node, error) {
	return build(file.Root, "root", map[string]bool{})
}

func build(node config.LayoutNode, path string, seen map[string]bool) (Node, error) {
	if node.Pane != "" && len(node.Children) > 0 {
		return Node{}, i18n.New("layout.paneAndContainer", i18n.Args{"path": path})
	}

	size, err := parseSize(node.Size, path+".size")
	if err != nil {
		return Node{}, err
	}

	if node.Pane != "" {
		return buildPane(node.Pane, size, path, seen)
	}
	return buildContainer(node, size, path, seen)
}

// buildPane проверяет лист: панель известна и стоит в раскладке один раз.
func buildPane(pane string, size Size, path string, seen map[string]bool) (Node, error) {
	if !slices.Contains(panes, pane) {
		return Node{}, i18n.New("layout.unknownPane", i18n.Args{"path": path, "pane": pane, "panes": strings.Join(panes, ", ")})
	}
	if seen[pane] {
		return Node{}, i18n.New("layout.paneTwice", i18n.Args{"path": path, "pane": pane})
	}

	seen[pane] = true
	return Node{Pane: pane, Size: size}, nil
}

// buildContainer проверяет контейнер и строит детей. Проценты детей в сумме не
// больше 100: иначе им не хватит места.
func buildContainer(node config.LayoutNode, size Size, path string, seen map[string]bool) (Node, error) {
	if node.Direction != "row" && node.Direction != "column" {
		return Node{}, i18n.New("layout.badDirection", i18n.Args{"path": path, "direction": node.Direction})
	}
	if len(node.Children) == 0 {
		return Node{}, i18n.New("layout.noChildren", i18n.Args{"path": path})
	}

	result := Node{Direction: node.Direction, Size: size}
	percent := 0
	for index, child := range node.Children {
		built, err := build(child, fmt.Sprintf("%s.children[%d]", path, index), seen)
		if err != nil {
			return Node{}, err
		}
		result.Children = append(result.Children, built)
		percent += built.Size.Percent
	}

	if percent > 100 {
		return Node{}, i18n.New("layout.percentOver", i18n.Args{"path": path, "percent": percent})
	}

	return result, nil
}

func parseSize(text config.Size, path string) (Size, error) {
	switch {
	case text == "":
		return Size{}, nil
	case percentSize.MatchString(string(text)):
		value, _ := strconv.Atoi(strings.TrimSuffix(string(text), "%"))
		if value < 1 || value > 100 {
			return Size{}, i18n.New("layout.percentRange", i18n.Args{"path": path, "size": text})
		}
		return Size{Percent: value}, nil
	case cellSize.MatchString(string(text)):
		value, _ := strconv.Atoi(string(text))
		if value < 1 {
			return Size{}, i18n.New("layout.cellsPositive", i18n.Args{"path": path})
		}
		return Size{Cells: value}, nil
	}

	return Size{}, i18n.New("layout.badSize", i18n.Args{"path": path, "size": text})
}

// Split раскладывает дерево в прямоугольник width на height. Размер с
// процентом или ячейками получает своё, узлы без размера делят остаток
// поровну, а лишнее после округления уходит первым. Размеры не уходят в минус.
func Split(root Node, width, height int) Placed {
	placed := Placed{Pane: root.Pane, Width: width, Height: height}
	if len(root.Children) == 0 {
		return placed
	}

	placed.Direction = root.Direction
	axis := width
	if root.Direction == "column" {
		axis = height
	}

	for index, share := range shares(root.Children, axis) {
		childWidth, childHeight := width, height
		if root.Direction == "column" {
			childHeight = share
		} else {
			childWidth = share
		}
		placed.Children = append(placed.Children, Split(root.Children[index], childWidth, childHeight))
	}

	return placed
}

// shares возвращает длину каждого ребенка вдоль оси.
func shares(children []Node, axis int) []int {
	lengths := make([]int, len(children))
	left := axis
	free := 0

	for index, child := range children {
		switch {
		case child.Size.Percent > 0:
			lengths[index] = min(axis*child.Size.Percent/100, left)
		case child.Size.Cells > 0:
			lengths[index] = min(child.Size.Cells, left)
		default:
			free++
			continue
		}
		left -= lengths[index]
	}

	if free == 0 {
		return lengths
	}

	each, extra := left/free, left%free
	for index, child := range children {
		if child.Size.Percent > 0 || child.Size.Cells > 0 {
			continue
		}
		lengths[index] = each
		if extra > 0 {
			lengths[index]++
			extra--
		}
	}

	return lengths
}

// Default и Focus встроенные раскладки. Файл с тем же именем заменяет их целиком.
func Default() Node {
	return Node{
		Direction: "row",
		Children: []Node{
			{Pane: "files", Size: Size{Percent: 20}},
			{Pane: "diff"},
		},
	}
}

func Focus() Node {
	return Node{Pane: "diff"}
}

// Has говорит, стоит ли панель где-то в дереве.
func Has(root Node, pane string) bool {
	if root.Pane == pane {
		return true
	}

	return slices.ContainsFunc(root.Children, func(child Node) bool { return Has(child, pane) })
}

// Without убирает панель из дерева, например закрытую панель файлов.
// Контейнер, оставшийся без детей, уходит вместе с ней: пустой контейнер
// отнял бы у остальных место.
func Without(root Node, pane string) Node {
	if len(root.Children) == 0 {
		return root
	}

	var children []Node
	for _, child := range root.Children {
		child = Without(child, pane)
		if child.Pane == pane || (child.Pane == "" && len(child.Children) == 0) {
			continue
		}
		children = append(children, child)
	}

	root.Children = children
	return root
}

// Resized задаёт панели размер, где бы она ни стояла в дереве, например
// ширину панели файлов, которую пользователь вытянул мышью.
func Resized(root Node, pane string, size Size) Node {
	if root.Pane == pane {
		root.Size = size
		return root
	}

	children := make([]Node, len(root.Children))
	for index, child := range root.Children {
		children[index] = Resized(child, pane, size)
	}
	root.Children = children
	return root
}

// Load возвращает раскладку по имени из config.json. Встроенные default и focus
// файла не требуют, остальные имена читаются из layouts/<имя>.json.
func Load(dir, name string) (Node, error) {
	file, found, err := config.ReadLayout(dir, name)
	if err != nil {
		return Node{}, err
	}

	if !found {
		switch name {
		case "default":
			return Default(), nil
		case "focus":
			return Focus(), nil
		}
		return Node{}, i18n.New("layout.notFound", i18n.Args{"name": name})
	}

	root, err := Build(file)
	if err != nil {
		return Node{}, fmt.Errorf("layouts/%s.json: %w", name, err)
	}

	return root, nil
}

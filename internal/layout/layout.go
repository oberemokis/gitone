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
		return Node{}, fmt.Errorf("%s: узел не может быть панелью и контейнером одновременно", path)
	}

	size, err := parseSize(node.Size, path+".size")
	if err != nil {
		return Node{}, err
	}

	if node.Pane != "" {
		if !slices.Contains(panes, node.Pane) {
			return Node{}, fmt.Errorf("%s.pane: неизвестная панель «%s», есть %s", path, node.Pane, strings.Join(panes, ", "))
		}
		if seen[node.Pane] {
			return Node{}, fmt.Errorf("%s.pane: панель «%s» уже есть в раскладке", path, node.Pane)
		}
		seen[node.Pane] = true

		return Node{Pane: node.Pane, Size: size}, nil
	}

	if node.Direction != "row" && node.Direction != "column" {
		return Node{}, fmt.Errorf("%s.direction: «%s» не направление, допустимы row и column", path, node.Direction)
	}
	if len(node.Children) == 0 {
		return Node{}, fmt.Errorf("%s.children: пусто, контейнер нужен с детьми", path)
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
		return Node{}, fmt.Errorf("%s.children: проценты в сумме %d, больше 100", path, percent)
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
			return Size{}, fmt.Errorf("%s: «%s» вне диапазона 1%%-100%%", path, text)
		}
		return Size{Percent: value}, nil
	case cellSize.MatchString(string(text)):
		value, _ := strconv.Atoi(string(text))
		if value < 1 {
			return Size{}, fmt.Errorf("%s: ячейки должны быть больше нуля", path)
		}
		return Size{Cells: value}, nil
	}

	return Size{}, fmt.Errorf("%s: «%s» не размер, допустимы «30%%» или число ячеек", path, text)
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
			{Pane: "files", Size: Size{Percent: 30}},
			{Pane: "diff"},
		},
	}
}

func Focus() Node {
	return Node{Pane: "diff"}
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
		return Node{}, fmt.Errorf("раскладка «%s» не найдена: нет layouts/%s.json", name, name)
	}

	root, err := Build(file)
	if err != nil {
		return Node{}, fmt.Errorf("layouts/%s.json: %w", name, err)
	}

	return root, nil
}

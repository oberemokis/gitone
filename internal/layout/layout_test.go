package layout_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/config"
	"gitone/internal/layout"
)

func TestBuildChecksTree(t *testing.T) {
	files := config.LayoutNode{Pane: "files", Size: "30%"}
	diff := config.LayoutNode{Pane: "diff"}

	tests := []struct {
		name string
		root config.LayoutNode
		// подстрока ошибки, пусто значит ошибки нет
		wantErr string
	}{
		{
			name: "дерево из двух панелей",
			root: config.LayoutNode{Direction: "row", Children: []config.LayoutNode{files, diff}},
		},
		{
			name:    "неизвестная панель",
			root:    config.LayoutNode{Direction: "row", Children: []config.LayoutNode{files, {Pane: "branchs"}}},
			wantErr: `root.children[1].pane: unknown pane "branchs"`,
		},
		{
			name:    "панель повторилась",
			root:    config.LayoutNode{Direction: "row", Children: []config.LayoutNode{diff, diff}},
			wantErr: `pane "diff" is already in the layout`,
		},
		{
			name:    "пустые дети",
			root:    config.LayoutNode{Direction: "row", Children: []config.LayoutNode{}},
			wantErr: "root.children",
		},
		{
			name:    "неизвестное направление",
			root:    config.LayoutNode{Direction: "diagonal", Children: []config.LayoutNode{files, diff}},
			wantErr: "root.direction",
		},
		{
			name:    "панель и дети одновременно",
			root:    config.LayoutNode{Pane: "diff", Direction: "row", Children: []config.LayoutNode{files}},
			wantErr: "root",
		},
		{
			name:    "сумма процентов больше ста",
			root:    config.LayoutNode{Direction: "row", Children: []config.LayoutNode{{Pane: "files", Size: "60%"}, {Pane: "diff", Size: "50%"}}},
			wantErr: "more than 100",
		},
		{
			name:    "размер не похож на размер",
			root:    config.LayoutNode{Direction: "row", Children: []config.LayoutNode{{Pane: "files", Size: "wide"}, diff}},
			wantErr: "root.children[0].size",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := layout.Build(config.LayoutFile{Root: tc.root})

			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("Build(%+v) error = %v, want nil", tc.root, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Build(%+v) error = %v, want error with %q", tc.root, err, tc.wantErr)
			}
		})
	}
}

func TestSplitSizes(t *testing.T) {
	tests := []struct {
		name          string
		root          layout.Node
		width, height int
		// ширины и высоты детей по порядку
		want []layout.Placed
	}{
		{
			name:  "процент и остаток",
			root:  layout.Node{Direction: "row", Children: []layout.Node{{Pane: "files", Size: layout.Size{Percent: 30}}, {Pane: "diff"}}},
			width: 100, height: 20,
			want: []layout.Placed{{Pane: "files", Width: 30, Height: 20}, {Pane: "diff", Width: 70, Height: 20}},
		},
		{
			name:  "ячейки по высоте",
			root:  layout.Node{Direction: "column", Children: []layout.Node{{Pane: "diff"}, {Pane: "files", Size: layout.Size{Cells: 6}}}},
			width: 80, height: 20,
			want: []layout.Placed{{Pane: "diff", Width: 80, Height: 14}, {Pane: "files", Width: 80, Height: 6}},
		},
		{
			name:  "без размера делят поровну, остаток первому",
			root:  layout.Node{Direction: "row", Children: []layout.Node{{Pane: "files"}, {Pane: "diff"}}},
			width: 11, height: 5,
			want: []layout.Placed{{Pane: "files", Width: 6, Height: 5}, {Pane: "diff", Width: 5, Height: 5}},
		},
		{
			name:  "ячейки больше ширины не уходят в минус",
			root:  layout.Node{Direction: "row", Children: []layout.Node{{Pane: "files", Size: layout.Size{Cells: 20}}, {Pane: "diff"}}},
			width: 10, height: 5,
			want: []layout.Placed{{Pane: "files", Width: 10, Height: 5}, {Pane: "diff", Width: 0, Height: 5}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := layout.Split(tc.root, tc.width, tc.height)

			if len(got.Children) != len(tc.want) {
				t.Fatalf("Split(%+v, %d, %d) children = %d, want %d", tc.root, tc.width, tc.height, len(got.Children), len(tc.want))
			}
			for index, want := range tc.want {
				child := got.Children[index]
				if child.Pane != want.Pane || child.Width != want.Width || child.Height != want.Height {
					t.Errorf("Split(%+v, %d, %d).Children[%d] = %+v, want %+v", tc.root, tc.width, tc.height, index, child, want)
				}
			}
		})
	}
}

// Панель файлов в default занимает пятую часть ширины.
func TestDefaultFilesPaneIsFifth(t *testing.T) {
	got := layout.Split(layout.Default(), 100, 20)

	if files := got.Children[0]; files.Pane != "files" || files.Width != 20 {
		t.Errorf("default %+v, want files шириной 20 слева", got)
	}
}

func TestWithoutAndHas(t *testing.T) {
	nested := layout.Node{Direction: "row", Children: []layout.Node{
		{Pane: "diff"},
		{Direction: "column", Children: []layout.Node{{Pane: "files", Size: layout.Size{Percent: 30}}}},
	}}

	tests := []struct {
		name     string
		root     layout.Node
		wantDiff int
	}{
		{name: "default: дифф во всю ширину", root: layout.Default(), wantDiff: 100},
		{name: "контейнер без детей уходит вместе с панелью", root: nested, wantDiff: 100},
		{name: "focus без панели не меняется", root: layout.Focus(), wantDiff: 100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			without := layout.Without(tc.root, "files")
			if layout.Has(without, "files") {
				t.Fatalf("Without оставил files: %+v", without)
			}

			placed := layout.Split(without, 100, 20)
			if width := diffWidth(placed); width != tc.wantDiff {
				t.Errorf("дифф шириной %d, want %d: %+v", width, tc.wantDiff, placed)
			}
		})
	}

	if !layout.Has(nested, "files") || layout.Has(layout.Focus(), "files") {
		t.Error("Has не нашёл files во вложенном контейнере или нашёл в focus")
	}
}

// diffWidth ищет панель диффа в размещённом дереве.
func diffWidth(placed layout.Placed) int {
	if placed.Pane == "diff" {
		return placed.Width
	}
	for _, child := range placed.Children {
		if width := diffWidth(child); width > 0 {
			return width
		}
	}
	return 0
}

func TestSinglePaneTakesWholeScreen(t *testing.T) {
	got := layout.Split(layout.Focus(), 90, 12)

	if got.Pane != "diff" || got.Width != 90 || got.Height != 12 {
		t.Errorf("focus %+v, ожидался дифф на весь экран", got)
	}
}

func TestLoadByName(t *testing.T) {
	dir := t.TempDir()

	root, err := layout.Load(dir, "default")
	if err != nil || len(root.Children) != 2 {
		t.Errorf("default без файла: %v, %+v", err, root)
	}

	if err := os.MkdirAll(filepath.Join(dir, "layouts"), 0o755); err != nil {
		t.Fatal(err)
	}
	compact := `{"root":{"pane":"diff"}}`
	if err := os.WriteFile(filepath.Join(dir, "layouts", "default.json"), []byte(compact), 0o644); err != nil {
		t.Fatal(err)
	}

	// файл с именем встроенной заменяет её целиком
	root, err = layout.Load(dir, "default")
	if err != nil || root.Pane != "diff" || len(root.Children) != 0 {
		t.Errorf("default из файла: %v, %+v", err, root)
	}

	if _, err := layout.Load(dir, "ghost"); err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Errorf("раскладка без файла: ошибка %v, ожидалась с именем ghost", err)
	}
}

package layout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/config"
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
			wantErr: `root.children[1].pane: неизвестная панель «branchs»`,
		},
		{
			name:    "панель повторилась",
			root:    config.LayoutNode{Direction: "row", Children: []config.LayoutNode{diff, diff}},
			wantErr: "панель «diff» уже есть",
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
			wantErr: "больше 100",
		},
		{
			name:    "размер не похож на размер",
			root:    config.LayoutNode{Direction: "row", Children: []config.LayoutNode{{Pane: "files", Size: "wide"}, diff}},
			wantErr: "root.children[0].size",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Build(config.LayoutFile{Root: tc.root})

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ошибка %v, ожидалась без ошибки", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ошибка %v, ожидалась с текстом %q", err, tc.wantErr)
			}
		})
	}
}

func TestSplitSizes(t *testing.T) {
	tests := []struct {
		name          string
		root          Node
		width, height int
		// ширины и высоты детей по порядку
		want []Placed
	}{
		{
			name:  "процент и остаток",
			root:  Node{Direction: "row", Children: []Node{{Pane: "files", Size: Size{Percent: 30}}, {Pane: "diff"}}},
			width: 100, height: 20,
			want: []Placed{{Pane: "files", Width: 30, Height: 20}, {Pane: "diff", Width: 70, Height: 20}},
		},
		{
			name:  "ячейки по высоте",
			root:  Node{Direction: "column", Children: []Node{{Pane: "diff"}, {Pane: "files", Size: Size{Cells: 6}}}},
			width: 80, height: 20,
			want: []Placed{{Pane: "diff", Width: 80, Height: 14}, {Pane: "files", Width: 80, Height: 6}},
		},
		{
			name:  "без размера делят поровну, остаток первому",
			root:  Node{Direction: "row", Children: []Node{{Pane: "files"}, {Pane: "diff"}}},
			width: 11, height: 5,
			want: []Placed{{Pane: "files", Width: 6, Height: 5}, {Pane: "diff", Width: 5, Height: 5}},
		},
		{
			name:  "ячейки больше ширины не уходят в минус",
			root:  Node{Direction: "row", Children: []Node{{Pane: "files", Size: Size{Cells: 20}}, {Pane: "diff"}}},
			width: 10, height: 5,
			want: []Placed{{Pane: "files", Width: 10, Height: 5}, {Pane: "diff", Width: 0, Height: 5}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Split(tc.root, tc.width, tc.height)

			for index, want := range tc.want {
				child := got.Children[index]
				if child.Pane != want.Pane || child.Width != want.Width || child.Height != want.Height {
					t.Fatalf("панель %d: %+v, ожидалось %+v", index, child, want)
				}
			}
		})
	}
}

func TestSinglePaneTakesWholeScreen(t *testing.T) {
	got := Split(Focus(), 90, 12)

	if got.Pane != "diff" || got.Width != 90 || got.Height != 12 {
		t.Fatalf("focus %+v, ожидался дифф на весь экран", got)
	}
}

func TestLoadByName(t *testing.T) {
	dir := t.TempDir()

	root, err := Load(dir, "default")
	if err != nil || len(root.Children) != 2 {
		t.Fatalf("default без файла: %v, %+v", err, root)
	}

	if err := os.MkdirAll(filepath.Join(dir, "layouts"), 0o755); err != nil {
		t.Fatal(err)
	}
	compact := `{"root":{"pane":"diff"}}`
	if err := os.WriteFile(filepath.Join(dir, "layouts", "default.json"), []byte(compact), 0o644); err != nil {
		t.Fatal(err)
	}

	// файл с именем встроенной заменяет её целиком
	root, err = Load(dir, "default")
	if err != nil || root.Pane != "diff" || len(root.Children) != 0 {
		t.Fatalf("default из файла: %v, %+v", err, root)
	}

	if _, err := Load(dir, "ghost"); err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("раскладка без файла: ошибка %v, ожидалась с именем ghost", err)
	}
}

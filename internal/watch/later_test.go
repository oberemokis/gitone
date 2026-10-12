package watch

import (
	"os"
	"path/filepath"
	"testing"
)

// Каталог, появившийся после старта, наблюдается целиком, даже если один его
// подкаталог не открывается: обход пропускает такой подкаталог и идёт дальше.
// Дерево собирается вне корня и переносится одним rename, чтобы обход видел его
// готовым.
func TestLaterDirectorySkipsUnreadableChild(t *testing.T) {
	root := t.TempDir()
	signals := start(t, root, 100)

	staging := filepath.Join(t.TempDir(), "new")
	locked := filepath.Join(staging, "a")
	for _, dir := range []string{locked, filepath.Join(staging, "b")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(root, "new")
	t.Cleanup(func() { os.Chmod(filepath.Join(moved, "a"), 0o755) })

	if err := os.Rename(staging, moved); err != nil {
		t.Fatal(err)
	}
	wait(t, signals)

	if err := os.WriteFile(filepath.Join(moved, "b", "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	wait(t, signals)
}

package watch

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// wait ждёт сигнал и падает, если его нет. Окно с запасом: события доходят
// не мгновенно, а задержка схлопывания 200 мс.
func wait(t *testing.T, signals <-chan struct{}) {
	t.Helper()

	select {
	case <-signals:
	case <-time.After(3 * time.Second):
		t.Fatal("сигнала не было")
	}
}

// silent проверяет, что сигнала нет в течение окна дольше задержки.
func silent(t *testing.T, signals <-chan struct{}) {
	t.Helper()

	select {
	case <-signals:
		t.Fatal("пришёл сигнал, которого не должно было быть")
	case <-time.After(700 * time.Millisecond):
	}
}

func start(t *testing.T, root string, maxDirs int) <-chan struct{} {
	t.Helper()

	done := make(chan struct{})
	t.Cleanup(func() { close(done) })

	signals, err := Start(root, maxDirs, done)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return signals
}

func TestWriteSignals(t *testing.T) {
	root := t.TempDir()
	signals := start(t, root, 100)

	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	wait(t, signals)
}

// Ожидание шлёт один сигнал на пачку записей, а не на каждую.
func TestBurstGivesOneSignal(t *testing.T) {
	root := t.TempDir()
	signals := start(t, root, 100)

	for index := range 5 {
		name := filepath.Join(root, "f"+string(rune('a'+index))+".txt")
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	wait(t, signals)
	silent(t, signals)
}

// Каталог, созданный после запуска, тоже наблюдается.
func TestNewDirectoryIsWatched(t *testing.T) {
	root := t.TempDir()
	signals := start(t, root, 100)

	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	wait(t, signals)

	if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	wait(t, signals)
}

// Запись в .git не даёт сигнала: иначе каждый коммит и каждая запись индекса
// дергали бы перечитывание.
func TestGitDirectoryIsIgnored(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	signals := start(t, root, 100)

	if err := os.WriteFile(filepath.Join(root, ".git", "index"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	silent(t, signals)
}

func TestTooManyDirectoriesFails(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b", "c", "d"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	done := make(chan struct{})
	defer close(done)
	if _, err := Start(root, 2, done); !errors.Is(err, ErrTooManyDirs) {
		t.Fatalf("ошибка %v, ожидалась ErrTooManyDirs", err)
	}
}

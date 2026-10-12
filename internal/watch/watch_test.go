package watch_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitone/internal/watch"
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

// drain ждёт первый сигнал и дочитывает следующие, пока они идут. Окно
// схлопывания отсчитывается от первого события пачки и не сдвигается, поэтому
// пачка дольше окна (медленный диск, -race) честно даёт больше одного сигнала.
// Пачка записана до вызова, так что её последний сигнал приходит не позже
// задержки после последнего события: пауза в 500 мс значит, что сигналы от
// пачки кончились. Потолок в 3 с ловит сигналы, которые не прекращаются.
// Возвращает число съеденных сигналов.
func drain(t *testing.T, signals <-chan struct{}) int {
	t.Helper()

	wait(t, signals)
	count := 1
	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-signals:
			count++
		case <-time.After(500 * time.Millisecond):
			return count
		case <-deadline:
			t.Fatal("сигналы не прекращаются")
		}
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

	signals, err := watch.Start(root, maxDirs, done)
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

// Пачка записей схлопывается: сигналов меньше, чем записей, и после них
// тишина. Точное число зависит от длины пачки относительно окна, поэтому
// тест требует только «меньше», а тишину проверяет после того, как сигналы
// пачки съедены.
func TestBurstIsCoalesced(t *testing.T) {
	root := t.TempDir()
	signals := start(t, root, 100)

	const writes = 5
	for index := range writes {
		name := filepath.Join(root, "f"+string(rune('a'+index))+".txt")
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if got := drain(t, signals); got >= writes {
		t.Errorf("пачка из %d записей дала %d сигналов, want меньше", writes, got)
	}
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
	if _, err := watch.Start(root, 2, done); !errors.Is(err, watch.ErrTooManyDirs) {
		t.Errorf("ошибка %v, ожидалась ErrTooManyDirs", err)
	}
}

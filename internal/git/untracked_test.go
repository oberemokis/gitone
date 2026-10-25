package git_test

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"gitone/internal/diff"
)

// Неотслеженный файл, который нельзя прочитать (права 000 у файла root из
// docker-тома), в дифф не попадает, а остальной дифф на месте (#382, случай 1).
// Раньше git diff отказывал «cannot hash», и gitone не запускался.
func TestUnreadableUntrackedFileIsSkipped(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("права 000 не запрещают чтение")
	}

	repo := newRepo(t)
	write(t, repo, "tracked.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "tracked.txt", "two\n")
	write(t, repo, "new.txt", "fresh\n")
	write(t, repo, "secret.txt", "secret\n")
	secret := filepath.Join(repo, "secret.txt")
	if err := os.Chmod(secret, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(secret, 0o644) })

	wantStatuses(t, load(t, repo, nil), map[string]diff.Status{"tracked.txt": diff.StatusModified, "new.txt": diff.StatusAdded})
}

// Временный файл, который появляется и исчезает (vim пишет и удаляет «4913»
// при сохранении, vite кладёт временные .mjs), не роняет дифф (случай 2). Он
// исчезал между ls-files и add -N, и add отвечал fatal на весь дифф: здесь
// так падал примерно каждый четвёртый вызов. Файл живёт 2 мс и 2 мс его нет.
func TestVanishingUntrackedFilesDoNotFailDiff(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "tracked.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "tracked.txt", "two\n")

	stop := make(chan struct{})
	var churn sync.WaitGroup
	churn.Go(func() {
		path := filepath.Join(repo, "4913")
		for {
			select {
			case <-stop:
				return
			default:
			}
			os.WriteFile(path, []byte("x\n"), 0o644)
			time.Sleep(2 * time.Millisecond)
			os.Remove(path)
			time.Sleep(2 * time.Millisecond)
		}
	})
	defer churn.Wait()
	defer close(stop)

	for range 50 {
		if _, err := open(t, repo).Diff(nil); err != nil {
			t.Fatalf("Diff при создании и удалении файлов: %v", err)
		}
	}
}

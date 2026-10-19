package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"gitone/internal/config"
)

// RememberRepo ставит репозиторий первым без повтора и держит предел;
// состояние лежит в state.json, а config.json не трогается (#248, случаи 1
// и 4).
func TestRememberRepo(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "gitone")

	for _, root := range []string{"/a", "/b", "/a"} {
		if err := config.RememberRepo(dir, root); err != nil {
			t.Fatalf("RememberRepo: %v", err)
		}
	}
	if got := config.ReadState(dir).RecentRepos; !slices.Equal(got, []string{"/a", "/b"}) {
		t.Fatalf("недавние %q, ожидались /a и /b", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); !os.IsNotExist(err) {
		t.Fatalf("config.json появился: %v", err)
	}

	for number := range 30 {
		_ = config.RememberRepo(dir, fmt.Sprintf("/repo%d", number))
	}
	if got := config.ReadState(dir).RecentRepos; len(got) != 20 || got[0] != "/repo29" {
		t.Fatalf("недавние %q, ожидались 20 последних", got)
	}
}

// state.json не записать (на его месте каталог): отказ возвращается, а
// временных файлов в каталоге настроек не остаётся (#310, случай 2).
func TestRememberRepoCleansTempOnFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "state.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json", "keep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	for range 3 {
		if err := config.RememberRepo(dir, "/a"); err == nil {
			t.Fatal("RememberRepo записал state.json поверх каталога")
		}
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "state.json.*")); len(left) != 0 {
		t.Fatalf("остались временные файлы: %q", left)
	}
}

// Запущенные разом gitone не теряют записи друг друга (случай 3).
func TestRememberRepoConcurrent(t *testing.T) {
	dir := t.TempDir()

	var group sync.WaitGroup
	for number := range 4 {
		group.Go(func() {
			if err := config.RememberRepo(dir, fmt.Sprintf("/repo%d", number)); err != nil {
				t.Errorf("RememberRepo: %v", err)
			}
		})
	}
	group.Wait()

	if got := config.ReadState(dir).RecentRepos; len(got) != 4 {
		t.Fatalf("недавние %q, ожидались все четыре", got)
	}
}

// Без файла и с испорченным файлом состояние пустое.
func TestReadStateBroken(t *testing.T) {
	dir := t.TempDir()
	if got := config.ReadState(dir); len(got.RecentRepos) != 0 {
		t.Fatalf("без файла %+v", got)
	}

	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := config.ReadState(dir); len(got.RecentRepos) != 0 {
		t.Fatalf("испорченный файл %+v", got)
	}
}

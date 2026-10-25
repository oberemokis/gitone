package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

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

// Блокировку, брошенную упавшим gitone, следующий запуск снимает по возрасту:
// иначе список недавних больше никогда не записался бы.
func TestRememberRepoTakesStaleLock(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "state.json.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}

	if err := config.RememberRepo(dir, "/a"); err != nil {
		t.Fatalf("RememberRepo: %v", err)
	}
	if got := config.ReadState(dir).RecentRepos; !slices.Equal(got, []string{"/a"}) {
		t.Fatalf("недавние %q, запись пропала из-за брошенной блокировки", got)
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

// Вид и список недавних лежат в одном state.json, и запись одного не теряет
// другое (#369, случай 4).
func TestSaveViewKeepsRecentRepos(t *testing.T) {
	dir := t.TempDir()
	view := config.View{Mode: "split", Files: true, IgnoreWhitespace: true, Context: 5}

	for _, save := range []func() error{
		func() error { return config.RememberRepo(dir, "/a") },
		func() error { return config.SaveView(dir, view) },
		func() error { return config.RememberRepo(dir, "/b") },
	} {
		if err := save(); err != nil {
			t.Fatal(err)
		}
	}

	got := config.ReadView(dir)
	if got.Saved.IsZero() {
		t.Fatal("у записанного вида нет времени записи")
	}
	got.Saved = time.Time{}
	if got != view {
		t.Fatalf("вид %+v, ожидался %+v", got, view)
	}
	if recent := config.ReadState(dir).RecentRepos; !slices.Equal(recent, []string{"/b", "/a"}) {
		t.Fatalf("недавние %q, ожидались /b и /a", recent)
	}
}

// config.json, поправленный после записи вида, главнее: вид отбрасывается
// целиком. Поправленный раньше вид не трогает (случай 3).
func TestReadViewAfterConfigEdit(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "config.json")
	if err := os.WriteFile(settings, []byte(`{"diff":{"mode":"auto"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	touch(t, settings, time.Now().Add(-time.Hour))
	if err := config.SaveView(dir, config.View{Mode: "unified", Context: -1}); err != nil {
		t.Fatal(err)
	}

	if got := config.ReadView(dir); got.Mode != "unified" {
		t.Fatalf("вид %+v, ожидался записанный: конфиг правили до него", got)
	}

	touch(t, settings, time.Now().Add(time.Hour))
	if got := config.ReadView(dir); got != (config.View{}) {
		t.Fatalf("вид %+v, ожидался пустой: конфиг правили после него", got)
	}
}

// Испорченного вида и вида без времени записи нет, как нет его в файле без
// вида (случаи 5 и 6).
func TestReadViewBroken(t *testing.T) {
	cases := []struct {
		name  string
		state string
	}{
		{name: "вида нет", state: `{"recentRepos":["/a"]}`},
		{name: "неизвестный режим", state: `{"view":{"mode":"diagonal","context":-1,"saved":"2026-01-01T00:00:00Z"}}`},
		{name: "контекст меньше -1", state: `{"view":{"mode":"split","context":-2,"saved":"2026-01-01T00:00:00Z"}}`},
		{name: "без времени записи", state: `{"view":{"mode":"split","context":-1}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(tc.state), 0o644); err != nil {
				t.Fatal(err)
			}

			if got := config.ReadView(dir); got != (config.View{}) {
				t.Fatalf("вид %+v, ожидался пустой", got)
			}
		})
	}
}

// touch ставит файлу время правки.
func touch(t *testing.T, path string, at time.Time) {
	t.Helper()

	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

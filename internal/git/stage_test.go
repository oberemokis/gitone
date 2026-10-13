package git_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gitone/internal/git"
)

// Файл в индексе попадает в StagedPaths, а Unstage убирает его оттуда.
func TestStageAndUnstage(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	if err := git.Stage(repo, []string{"a.txt"}); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if staged := mustStaged(t, repo); !slices.Equal(staged, []string{"a.txt"}) {
		t.Fatalf("в индексе %q, ожидался a.txt", staged)
	}

	if err := git.Unstage(repo, []string{"a.txt"}); err != nil {
		t.Fatalf("Unstage: %v", err)
	}
	if staged := mustStaged(t, repo); len(staged) != 0 {
		t.Fatalf("в индексе %q, ожидалось пусто", staged)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "two\n" {
		t.Fatalf("правка в дереве пропала: %q", got)
	}
}

// Discard возвращает файл к HEAD, а правка в дереве уходит.
func TestDiscardRestoresTrackedFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	if err := git.Discard(repo, []string{"a.txt"}); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "one\n" {
		t.Fatalf("файл %q, ожидался возврат к HEAD", got)
	}
}

// Неотслеженный файл удаляется с диска.
func TestDiscardRemovesUntrackedFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "keep.txt", "k\n")
	commitAll(t, repo)
	write(t, repo, "new.txt", "fresh\n")

	if err := git.Discard(repo, []string{"new.txt"}); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("неотслеженный файл остался: %v", err)
	}
}

// Файл, добавленный в индекс и не бывший в HEAD, уходит и из индекса, и с диска.
func TestDiscardDropsStagedNewFile(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "keep.txt", "k\n")
	commitAll(t, repo)
	write(t, repo, "new.txt", "fresh\n")
	runGit(t, repo, "add", "new.txt")

	if err := git.Discard(repo, []string{"new.txt"}); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if staged := mustStaged(t, repo); len(staged) != 0 {
		t.Fatalf("в индексе %q, ожидалось пусто", staged)
	}
	if _, err := os.Stat(filepath.Join(repo, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("файл остался на диске: %v", err)
	}
}

// Правка в индексе уходит и из индекса, и с диска: тест не должен смотреть
// только на диск, иначе restore без --staged остался бы незамеченным.
func TestDiscardRestoresStagedChange(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "add", "a.txt")

	if err := git.Discard(repo, []string{"a.txt"}); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if staged := mustStaged(t, repo); len(staged) != 0 {
		t.Fatalf("в индексе %q, ожидалось пусто", staged)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "one\n" {
		t.Fatalf("файл %q, ожидался возврат к HEAD", got)
	}
}

// Путь из диффа считается от корня репозитория, а не от каталога запуска.
func TestDiscardRemovesUntrackedFileFromSubdirectory(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "sub/keep.txt", "k\n")
	commitAll(t, repo)
	write(t, repo, "sub/new.txt", "fresh\n")

	if err := git.Discard(filepath.Join(repo, "sub"), []string{"sub/new.txt"}); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "sub", "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("неотслеженный файл остался: %v", err)
	}
}

// Скобки в имени файла не шаблон: `pages/[id].vue` совпал бы и с `pages/i.vue`,
// и отмена одного стирала бы правку другого.
func TestDiscardTreatsPathAsLiteral(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "pages/[id].vue", "one\n")
	write(t, repo, "pages/i.vue", "one\n")
	commitAll(t, repo)
	write(t, repo, "pages/[id].vue", "two\n")
	write(t, repo, "pages/i.vue", "two\n")

	if err := git.Discard(repo, []string{"pages/[id].vue"}); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, "pages", "[id].vue")); got != "one\n" {
		t.Fatalf("pages/[id].vue = %q, ожидался возврат к HEAD", got)
	}
	if got := readFile(t, filepath.Join(repo, "pages", "i.vue")); got != "two\n" {
		t.Fatalf("pages/i.vue = %q, правка соседа пропала", got)
	}
}

// Неотслеженный `pages/[id].vue` убирается из индекса по имени: индекс соседа
// `pages/i.vue` остаётся как был.
func TestDiscardUntrackedKeepsNeighbourInIndex(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "pages/i.vue", "one\n")
	commitAll(t, repo)
	write(t, repo, "pages/i.vue", "two\n")
	runGit(t, repo, "add", "pages/i.vue")
	write(t, repo, "pages/[id].vue", "fresh\n")

	if err := git.Discard(repo, []string{"pages/[id].vue"}); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	// В индексе стоит правка, а не удаление файла: StagedPaths назвал бы
	// pages/i.vue и в том, и в другом случае.
	if got := runGit(t, repo, "show", ":pages/i.vue"); got != "two\n" {
		t.Fatalf("pages/i.vue в индексе = %q, ожидалась застейдженная правка", got)
	}
	if _, err := os.Stat(filepath.Join(repo, "pages", "[id].vue")); !os.IsNotExist(err) {
		t.Fatalf("pages/[id].vue остался на диске: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, "pages", "i.vue")); got != "two\n" {
		t.Fatalf("pages/i.vue = %q, правка соседа пропала", got)
	}
}

func TestStageTreatsPathAsLiteral(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "pages/[id].vue", "one\n")
	write(t, repo, "pages/i.vue", "one\n")
	commitAll(t, repo)
	write(t, repo, "pages/[id].vue", "two\n")
	write(t, repo, "pages/i.vue", "two\n")

	if err := git.Stage(repo, []string{"pages/[id].vue"}); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if staged := mustStaged(t, repo); !slices.Equal(staged, []string{"pages/[id].vue"}) {
		t.Fatalf("в индексе %q, ожидался только pages/[id].vue", staged)
	}
}

func TestUnstageTreatsPathAsLiteral(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "pages/[id].vue", "one\n")
	write(t, repo, "pages/i.vue", "one\n")
	commitAll(t, repo)
	write(t, repo, "pages/[id].vue", "two\n")
	write(t, repo, "pages/i.vue", "two\n")
	runGit(t, repo, "add", "-A")

	if err := git.Unstage(repo, []string{"pages/[id].vue"}); err != nil {
		t.Fatalf("Unstage: %v", err)
	}
	if staged := mustStaged(t, repo); !slices.Equal(staged, []string{"pages/i.vue"}) {
		t.Fatalf("в индексе %q, ожидался только pages/i.vue", staged)
	}
}

func mustStaged(t *testing.T, repo string) []string {
	t.Helper()

	staged, err := git.StagedPaths(repo)
	if err != nil {
		t.Fatalf("StagedPaths: %v", err)
	}
	return staged
}

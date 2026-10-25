package git_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"gitone/internal/git"
	"gitone/internal/i18n"
)

// Файл в индексе попадает в StagedPaths, а Unstage убирает его оттуда.
func TestStageAndUnstage(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")

	if err := open(t, repo).Stage([]string{"a.txt"}); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if staged := mustStaged(t, repo); !slices.Equal(staged, []string{"a.txt"}) {
		t.Fatalf("в индексе %q, ожидался a.txt", staged)
	}

	if err := open(t, repo).Unstage([]string{"a.txt"}); err != nil {
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

	if err := open(t, repo).Discard([]string{"a.txt"}); err != nil {
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

	if err := open(t, repo).Discard([]string{"new.txt"}); err != nil {
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

	if err := open(t, repo).Discard([]string{"new.txt"}); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if staged := mustStaged(t, repo); len(staged) != 0 {
		t.Fatalf("в индексе %q, ожидалось пусто", staged)
	}
	if _, err := os.Stat(filepath.Join(repo, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("файл остался на диске: %v", err)
	}
}

// Ведущее двоеточие ls-tree отверг бы как магию pathspec и этим отправил бы
// файл из HEAD в ветку «нового»: правка пропала бы вместе с файлом.
func TestDiscardRestoresTrackedFileWithLeadingColon(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, ":note.md", "one\n")
	commitAll(t, repo)
	write(t, repo, ":note.md", "two\n")

	if err := open(t, repo).Discard([]string{":note.md"}); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, ":note.md")); got != "one\n" {
		t.Fatalf(":note.md = %q, ожидался возврат к HEAD", got)
	}
}

// `git add -A --` без путей ставит в индекс всё дерево, поэтому пустой список
// отказывает до git. Нулевой Repo это доказывает: вызов git дал бы
// git.noRepository, а не git.noPaths.
func TestEmptyPathListIsRefusedBeforeGit(t *testing.T) {
	var none git.Repo
	tests := []struct {
		name string
		call func(paths []string) error
	}{
		{name: "Stage", call: none.Stage},
		{name: "Unstage", call: none.Unstage},
		{name: "Discard", call: none.Discard},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call(nil)

			var refusal *i18n.Error
			if !errors.As(err, &refusal) || refusal.Key != "git.noPaths" {
				t.Errorf("%s(nil) error = %v, ожидался отказ git.noPaths", tc.name, err)
			}
		})
	}
}

// Нулевой Repo не репозиторий: команда отказывает до запуска git, а не идёт в
// текущий каталог процесса, где лежит чужой репозиторий (#356).
func TestZeroRepoRefusesBeforeGit(t *testing.T) {
	var none git.Repo

	if err := none.Stage([]string{"a.txt"}); !isRefusal(err, "git.noRepository") {
		t.Errorf("Stage на нулевом Repo = %v, ожидался отказ git.noRepository", err)
	}
	if _, err := none.WorktreeFile("a.txt"); !isRefusal(err, "git.noRepository") {
		t.Errorf("WorktreeFile на нулевом Repo = %v, ожидался отказ git.noRepository", err)
	}
}

func isRefusal(err error, key string) bool {
	var refusal *i18n.Error
	return errors.As(err, &refusal) && refusal.Key == key
}

// До первого коммита restore --staged падает: восстанавливать не из чего. Файл
// всё равно должен уходить из индекса, а правка на диске оставаться.
func TestUnstageBeforeFirstCommit(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "a.txt")

	if err := open(t, repo).Unstage([]string{"a.txt"}); err != nil {
		t.Fatalf("Unstage: %v", err)
	}
	if staged := mustStaged(t, repo); len(staged) != 0 {
		t.Fatalf("в индексе %q, ожидалось пусто", staged)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "one\n" {
		t.Fatalf("файл на диске = %q, ожидалось нетронутое содержимое", got)
	}
}

// Файл, изменённый после add, rm --cached без -f снять не давал: «staged
// content different from both the file and the HEAD» (#384, случай 1).
func TestUnstageBeforeFirstCommitAfterEdit(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "a.txt")
	write(t, repo, "a.txt", "two\n")

	if err := open(t, repo).Unstage([]string{"a.txt"}); err != nil {
		t.Fatalf("Unstage: %v", err)
	}
	if got := runGit(t, repo, "ls-files"); got != "" {
		t.Fatalf("в индексе %q, ожидалось пусто", got)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "two\n" {
		t.Fatalf("файл на диске = %q, ожидалась правка", got)
	}
}

// Переименование только регистра: на нечувствительной к регистру ФС (macOS)
// Foo.txt и foo.txt это один файл, и restore до удаления «нового» стирал его
// (#384, случай 2). На чувствительной ФС ответ тот же.
func TestDiscardCaseOnlyRename(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "Foo.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "mv", "Foo.txt", "foo.txt")

	if err := open(t, repo).Discard([]string{"Foo.txt", "foo.txt"}); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	entries, err := os.ReadDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !slices.Equal(names, []string{".git", "Foo.txt"}) {
		t.Fatalf("на диске %q, ожидался Foo.txt", names)
	}
	if got := readFile(t, filepath.Join(repo, "Foo.txt")); got != "one\n" {
		t.Fatalf("Foo.txt = %q, ожидалось содержимое HEAD", got)
	}
	if got := runGit(t, repo, "status", "--porcelain"); got != "" {
		t.Fatalf("git status %q, ожидалось чисто", got)
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

	if err := open(t, repo).Discard([]string{"a.txt"}); err != nil {
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

	if err := open(t, filepath.Join(repo, "sub")).Discard([]string{"sub/new.txt"}); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "sub", "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("неотслеженный файл остался: %v", err)
	}
}

// У gitlink подмодуля в HEAD нет объекта для cat-file, поэтому файл «нового»
// вида вёл Discard в удаление подмодуля. Теперь отказ идёт до любой правки:
// индекс и диск остаются как были.
func TestDiscardRefusesSubmodule(t *testing.T) {
	upstream := newRepo(t)
	write(t, upstream, "lib.txt", "one\n")
	commitAll(t, upstream)
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", upstream, "sub")
	runGit(t, repo, "commit", "-q", "-m", "add submodule")
	sub := filepath.Join(repo, "sub")
	write(t, sub, "lib.txt", "two\n")
	runGit(t, sub, "commit", "-q", "-a", "-m", "next")
	write(t, repo, "a.txt", "two\n")
	before := runGit(t, repo, "status", "--porcelain=v2")

	err := open(t, repo).Discard([]string{"a.txt", "sub"})

	var refusal *i18n.Error
	if !errors.As(err, &refusal) || refusal.Key != "git.submoduleDiscard" {
		t.Fatalf("Discard error = %v, ожидался отказ git.submoduleDiscard", err)
	}
	if after := runGit(t, repo, "status", "--porcelain=v2"); after != before {
		t.Fatalf("git status изменился:\nдо:\n%s\nпосле:\n%s", before, after)
	}
}

// Подмодуль, которого нет в HEAD, а в индексе он уже есть (git submodule add),
// шёл в ветку «нового файла»: rm --cached менял индекс, потом os.Remove падал на
// непустом каталоге. Отказ идёт до любой правки, индекс и диск остаются как были.
func TestDiscardRefusesNewStagedSubmodule(t *testing.T) {
	upstream := newRepo(t)
	write(t, upstream, "lib.txt", "one\n")
	commitAll(t, upstream)
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", upstream, "sub")
	before := runGit(t, repo, "status", "--porcelain=v2")

	err := open(t, repo).Discard([]string{"sub"})

	var refusal *i18n.Error
	if !errors.As(err, &refusal) || refusal.Key != "git.submoduleDiscard" {
		t.Fatalf("Discard error = %v, ожидался отказ git.submoduleDiscard", err)
	}
	if after := runGit(t, repo, "status", "--porcelain=v2"); after != before {
		t.Fatalf("git status изменился:\nдо:\n%s\nпосле:\n%s", before, after)
	}
	if _, err := os.Stat(filepath.Join(repo, "sub", "lib.txt")); err != nil {
		t.Fatalf("файл подмодуля пропал с диска: %v", err)
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

	if err := open(t, repo).Discard([]string{"pages/[id].vue"}); err != nil {
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

	if err := open(t, repo).Discard([]string{"pages/[id].vue"}); err != nil {
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

	if err := open(t, repo).Stage([]string{"pages/[id].vue"}); err != nil {
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

	if err := open(t, repo).Unstage([]string{"pages/[id].vue"}); err != nil {
		t.Fatalf("Unstage: %v", err)
	}
	if staged := mustStaged(t, repo); !slices.Equal(staged, []string{"pages/i.vue"}) {
		t.Fatalf("в индексе %q, ожидался только pages/i.vue", staged)
	}
}

// Файл с конфликтом слияния числится в индексе неслитым, это не «застейджено».
// Иначе пробел звал бы Unstage, и restore --staged стирал стадии конфликта.
func TestStagedPathsSkipUnmergedFile(t *testing.T) {
	repo := conflictedRepo(t)
	write(t, repo, "b.txt", "staged\n")
	runGit(t, repo, "add", "b.txt")

	if staged := mustStaged(t, repo); !slices.Equal(staged, []string{"b.txt"}) {
		t.Fatalf("в индексе %q, ожидался только b.txt без неслитого a.txt", staged)
	}
}

// conflictedRepo возвращает репозиторий посреди слияния с конфликтом в a.txt.
func conflictedRepo(t *testing.T) string {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "base\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "topic")
	write(t, repo, "a.txt", "topic\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "main")
	write(t, repo, "a.txt", "main\n")
	commitAll(t, repo)

	merge := exec.Command("git", "merge", "topic")
	merge.Dir = repo
	if out, err := merge.CombinedOutput(); err == nil {
		t.Fatalf("слияние прошло без конфликта:\n%s", out)
	}

	return repo
}

func mustStaged(t *testing.T, repo string) []string {
	t.Helper()

	staged, err := open(t, repo).StagedPaths()
	if err != nil {
		t.Fatalf("StagedPaths: %v", err)
	}
	return staged
}

package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/diff"
	"gitone/internal/git"
)

func TestUntrackedFileIsShownAsAdded(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "tracked.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "new.txt", "fresh\n")

	files := load(t, repo, nil)

	wantStatuses(t, files, map[string]diff.Status{"new.txt": diff.StatusAdded})
}

func TestArgumentsGoToGitDiffWithoutUntrackedFiles(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "staged.txt", "one\n")
	write(t, repo, "unstaged.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "staged.txt", "two\n")
	runGit(t, repo, "add", "staged.txt")
	write(t, repo, "unstaged.txt", "two\n")
	write(t, repo, "new.txt", "fresh\n")

	files := load(t, repo, []string{"--staged"})

	wantStatuses(t, files, map[string]diff.Status{"staged.txt": diff.StatusModified})
}

func TestMoveInWorkingTreeIsShownAsRename(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\ntwo\nthree\nfour\nfive\n")
	commitAll(t, repo)
	if err := os.Rename(filepath.Join(repo, "a.txt"), filepath.Join(repo, "b.txt")); err != nil {
		t.Fatal(err)
	}

	files := load(t, repo, nil)

	if len(files) != 1 || files[0].Status != diff.StatusRenamed || files[0].OldPath != "a.txt" || files[0].NewPath != "b.txt" {
		t.Fatalf("ожидалось переименование a.txt → b.txt, получили %+v", files)
	}
}

func TestRunFromSubdirectoryShowsWholeRepositoryFromRoot(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "top.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "top.txt", "two\n")
	write(t, repo, "root.txt", "fresh\n")
	write(t, repo, "sub/inner.txt", "fresh\n")

	files := load(t, filepath.Join(repo, "sub"), nil)

	wantStatuses(t, files, map[string]diff.Status{
		"top.txt":       diff.StatusModified,
		"root.txt":      diff.StatusAdded,
		"sub/inner.txt": diff.StatusAdded,
	})
}

func TestFileNamedHeadDoesNotBreakLoad(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	write(t, repo, "HEAD", "not a revision\n")

	files := load(t, repo, nil)

	wantStatuses(t, files, map[string]diff.Status{"a.txt": diff.StatusModified, "HEAD": diff.StatusAdded})
}

func TestNestedRepositoryIsSkipped(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	nested := filepath.Join(repo, "nested")
	write(t, nested, "inner.txt", "inner\n")
	runGit(t, nested, "init", "-q", "-b", "main")

	files := load(t, repo, nil)

	wantStatuses(t, files, map[string]diff.Status{"a.txt": diff.StatusModified})
}

func TestRealIndexIsUntouched(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	write(t, repo, "new.txt", "fresh\n")
	before := readFile(t, filepath.Join(repo, ".git", "index"))

	load(t, repo, nil)

	if after := readFile(t, filepath.Join(repo, ".git", "index")); after != before {
		t.Fatal("загрузка диффа изменила настоящий индекс")
	}
}

func TestRepositoryWithoutCommitsShowsEverythingAsAdded(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "staged.txt", "one\n")
	runGit(t, repo, "add", "staged.txt")
	write(t, repo, "loose.txt", "two\n")

	files := load(t, repo, nil)

	wantStatuses(t, files, map[string]diff.Status{"staged.txt": diff.StatusAdded, "loose.txt": diff.StatusAdded})
}

func TestFileOverTenMegabytesComesAsBinary(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "big.txt", strings.Repeat("line\n", 2_200_000))

	files := load(t, repo, nil)

	if len(files) != 1 || !files[0].Binary || len(files[0].Hunks) != 0 {
		t.Fatalf("ожидался бинарный big.txt без хунков, получили %d файлов, первый бинарный: %v", len(files), len(files) > 0 && files[0].Binary)
	}
}

func TestErrorsCarryGitMessage(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	outside := t.TempDir()
	// Временный каталог не должен оказаться внутри чужого репозитория.
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(outside))

	cases := []struct {
		name string
		dir  string
		args []string
		want string
	}{
		{name: "вне репозитория", dir: outside, want: "not a git repository"},
		{name: "неверный аргумент", dir: repo, args: []string{"main..nope"}, want: "unknown revision"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := git.Diff(tc.dir, tc.args)

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Diff(%q, %q) error = %v, want с текстом %q", tc.dir, tc.args, err, tc.want)
			}
		})
	}
}

// newRepo создаёт репозиторий во временном каталоге. Глобальный и системный
// конфиги отключены: результат не должен зависеть от настроек машины.
func newRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	repo := t.TempDir()
	runGit(t, repo, "init", "-q", "-b", "main")
	return repo
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	command := exec.Command("git", args...)
	command.Dir = dir
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}

	return string(out)
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()

	target := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, name string) string {
	t.Helper()

	content, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}

	return string(content)
}

func commitAll(t *testing.T, repo string) {
	t.Helper()
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "commit")
}

func load(t *testing.T, dir string, args []string) []diff.File {
	t.Helper()

	text, err := git.Diff(dir, args)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	return diff.Parse(text)
}

// wantStatuses сверяет набор файлов диффа: путь и статус каждого, ни одного
// лишнего. Путь удалённого файла старый, остальных новый.
func wantStatuses(t *testing.T, files []diff.File, want map[string]diff.Status) {
	t.Helper()

	got := map[string]diff.Status{}
	for _, file := range files {
		name := file.NewPath
		if name == "" {
			name = file.OldPath
		}
		got[name] = file.Status
	}

	if len(got) != len(want) {
		t.Fatalf("файлы %v, ожидались %v", got, want)
	}
	for name, status := range want {
		if got[name] != status {
			t.Fatalf("файлы %v, ожидались %v", got, want)
		}
	}
}

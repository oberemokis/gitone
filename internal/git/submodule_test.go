package git_test

import (
	"context"
	"path/filepath"
	"testing"

	"gitone/internal/git"
)

// withSubmodule это репозиторий с подмодулем libs/lib из соседнего
// временного репозитория. Клон из локального пути git по умолчанию
// запрещает подмодулям, тест разрешает его через окружение.
func withSubmodule(t *testing.T) (repo, lib string) {
	t.Helper()
	return withSubmoduleAt(t, "libs/lib")
}

// withSubmoduleAt это withSubmodule с подмодулем по пути path.
func withSubmoduleAt(t *testing.T, path string) (repo, lib string) {
	t.Helper()

	lib = newRepo(t)
	write(t, lib, "x.txt", "lib\n")
	commitAll(t, lib)

	repo = newRepo(t)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "protocol.file.allow")
	t.Setenv("GIT_CONFIG_VALUE_0", "always")
	write(t, repo, "a.txt", "app\n")
	commitAll(t, repo)
	runGit(t, repo, "submodule", "add", "-q", lib, path)
	runGit(t, repo, "commit", "-q", "-m", "submodule")
	return repo, lib
}

// Список отдаёт путь, адрес и коммит; не клонированный подмодуль помечен, а
// не пропал; обновление клонирует его (#244, случаи 1–3).
func TestSubmodules(t *testing.T) {
	repo, lib := withSubmodule(t)
	commit := revision(t, lib, "HEAD")

	submodules, err := git.Submodules(repo)
	if err != nil || len(submodules) != 1 {
		t.Fatalf("Submodules = %+v, %v", submodules, err)
	}
	if got := submodules[0]; got.Path != "libs/lib" || got.URL != lib || got.Commit != commit || !got.Initialized || got.Moved {
		t.Fatalf("подмодуль %+v", got)
	}

	clone := filepath.Join(t.TempDir(), "clone")
	runGit(t, filepath.Dir(clone), "clone", "-q", repo, clone)
	submodules, _ = git.Submodules(clone)
	if len(submodules) != 1 || submodules[0].Initialized {
		t.Fatalf("в клоне без --recursive подмодуль %+v, ожидался не клонированный", submodules)
	}

	if err := git.UpdateSubmodule(context.Background(), clone, "libs/lib"); err != nil {
		t.Fatalf("UpdateSubmodule: %v", err)
	}
	if got := readFile(t, filepath.Join(clone, "libs", "lib", "x.txt")); got != "lib\n" {
		t.Fatalf("x.txt в подмодуле = %q", got)
	}
}

// Путь с пробелом приходит целиком и с адресом, клонированный и нет, и
// обновление клонирует подмодуль по нему (#295, случаи 1 и 2).
func TestSubmoduleWithSpaceInPath(t *testing.T) {
	repo, lib := withSubmoduleAt(t, "my lib")

	submodules, err := git.Submodules(repo)
	if err != nil || len(submodules) != 1 || submodules[0].Path != "my lib" || submodules[0].URL != lib {
		t.Fatalf("Submodules = %+v, %v", submodules, err)
	}

	clone := filepath.Join(t.TempDir(), "clone")
	runGit(t, filepath.Dir(clone), "clone", "-q", repo, clone)
	submodules, _ = git.Submodules(clone)
	if len(submodules) != 1 || submodules[0].Path != "my lib" || submodules[0].Initialized {
		t.Fatalf("в клоне подмодуль %+v, ожидался не клонированный «my lib»", submodules)
	}
	if err := git.UpdateSubmodule(context.Background(), clone, submodules[0].Path); err != nil {
		t.Fatalf("UpdateSubmodule: %v", err)
	}
	if got := readFile(t, filepath.Join(clone, "my lib", "x.txt")); got != "lib\n" {
		t.Fatalf("x.txt в подмодуле = %q", got)
	}
}

// Репозиторий без подмодулей даёт пустой список.
func TestSubmodulesEmpty(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	if submodules, err := git.Submodules(repo); err != nil || len(submodules) != 0 {
		t.Fatalf("Submodules = %+v, %v", submodules, err)
	}
}

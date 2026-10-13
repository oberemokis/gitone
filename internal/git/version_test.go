package git_test

import (
	"testing"

	"gitone/internal/diff"
	"gitone/internal/git"
)

// fileAt это файл диффа с новым путём path.
func fileAt(t *testing.T, files []diff.File, path string) diff.File {
	t.Helper()

	for _, file := range files {
		if file.NewPath == path {
			return file
		}
	}
	t.Fatalf("в диффе нет %s: %+v", path, files)
	return diff.File{}
}

// Версии файла читаются по хэшам из диффа (#140): старая из git, новая
// правки вне индекса из рабочего дерева, у нового файла старой нет.
func TestVersionReadsBothSides(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "App.vue", "old\n")
	commitAll(t, repo)
	write(t, repo, "App.vue", "new\n")
	write(t, repo, "fresh.vue", "fresh\n")
	files := load(t, repo, nil)

	changed := fileAt(t, files, "App.vue")
	if len(changed.OldBlob) != 40 {
		t.Fatalf("хэш старой версии %q, ожидался полный", changed.OldBlob)
	}
	for _, side := range []struct {
		hash, path, want string
	}{
		{hash: changed.OldBlob, want: "old\n"},
		{hash: changed.NewBlob, path: changed.NewPath, want: "new\n"},
	} {
		if got, err := git.Version(repo, side.hash, side.path); err != nil || string(got) != side.want {
			t.Errorf("Version(%s) = %q, %v; want %q", side.hash, got, err, side.want)
		}
	}

	fresh := fileAt(t, files, "fresh.vue")
	if fresh.OldBlob != "" {
		t.Errorf("у нового файла хэш старой версии %q", fresh.OldBlob)
	}
	if got, err := git.Version(repo, fresh.NewBlob, fresh.NewPath); err != nil || string(got) != "fresh\n" {
		t.Errorf("новый файл: %q, %v", got, err)
	}
}

// Новая версия есть в git (коммит из лога, stash, --staged): читается она, а
// не файл рабочего дерева, который мог измениться после. Иначе подсветка
// строилась бы по чужому тексту.
func TestVersionPrefersGitObjectOverWorkingTree(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "App.vue", "old\n")
	commitAll(t, repo)
	write(t, repo, "App.vue", "committed\n")
	commitAll(t, repo)
	write(t, repo, "App.vue", "dirty\n")
	changed := fileAt(t, load(t, repo, []string{"HEAD^!"}), "App.vue")

	got, err := git.Version(repo, changed.NewBlob, changed.NewPath)

	if err != nil || string(got) != "committed\n" {
		t.Errorf("Version(NewBlob, NewPath) = %q, %v; want %q", got, err, "committed\n")
	}
}

// Версии нет: ни содержимого, ни ошибки. Хэша нет в git и путь не задан:
// ошибка, а не пустой файл.
func TestVersionWithoutObject(t *testing.T) {
	repo := newRepo(t)

	if got, err := git.Version(repo, "", ""); got != nil || err != nil {
		t.Errorf("пустой хэш: %q, %v", got, err)
	}
	if _, err := git.Version(repo, "1234567890123456789012345678901234567890", ""); err == nil {
		t.Error("хэша нет в git, а ошибки нет")
	}
}

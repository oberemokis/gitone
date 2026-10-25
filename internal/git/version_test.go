package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"gitone/internal/diff"
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
		if got, err := open(t, repo).Version(side.hash, side.path); err != nil || string(got) != side.want {
			t.Errorf("Version(%s) = %q, %v; want %q", side.hash, got, err, side.want)
		}
	}

	fresh := fileAt(t, files, "fresh.vue")
	if fresh.OldBlob != "" {
		t.Errorf("у нового файла хэш старой версии %q", fresh.OldBlob)
	}
	if got, err := open(t, repo).Version(fresh.NewBlob, fresh.NewPath); err != nil || string(got) != "fresh\n" {
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

	got, err := open(t, repo).Version(changed.NewBlob, changed.NewPath)

	if err != nil || string(got) != "committed\n" {
		t.Errorf("Version(NewBlob, NewPath) = %q, %v; want %q", got, err, "committed\n")
	}
}

// Версии нет: ни содержимого, ни ошибки. Хэша нет в git и путь не задан:
// ошибка, а не пустой файл.
func TestVersionWithoutObject(t *testing.T) {
	repo := newRepo(t)

	if got, err := open(t, repo).Version("", ""); got != nil || err != nil {
		t.Errorf("пустой хэш: %q, %v", got, err)
	}
	if _, err := open(t, repo).Version("1234567890123456789012345678901234567890", ""); err == nil {
		t.Error("хэша нет в git, а ошибки нет")
	}
}

// У симлинка git хэширует текст ссылки, и версия из рабочего дерева тоже
// текст ссылки, а не файл, на который она ведёт: иначе подсветка ложилась бы
// на чужие строки (#389, случай 4).
func TestVersionOfSymlinkIsLinkText(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "real.vue", "<template>\n  <p>real</p>\n</template>\n")
	commitAll(t, repo)
	if err := os.Symlink("real.vue", filepath.Join(repo, "link.vue")); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "-N", "link.vue")
	link := fileAt(t, load(t, repo, nil), "link.vue")

	if got, err := open(t, repo).Version(link.NewBlob, link.NewPath); err != nil || string(got) != "real.vue" {
		t.Errorf("Version(link.vue) = %q, %v; want %q", got, err, "real.vue")
	}
}

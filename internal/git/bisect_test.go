package git_test

import (
	"fmt"
	"testing"
)

// eightCommits это ветка main из коммитов c1…c8, правящих a.txt.
func eightCommits(t *testing.T) string {
	t.Helper()

	repo := newRepo(t)
	for number := 1; number <= 8; number++ {
		write(t, repo, "a.txt", fmt.Sprintf("%d\n", number))
		runGit(t, repo, "add", "-A")
		runGit(t, repo, "commit", "-q", "-m", fmt.Sprintf("c%d", number))
	}
	return repo
}

// Плохой и хороший коммиты начинают поиск, git переходит на коммит для
// проверки (#245, случай 1); отметки ведут к найденному коммиту (случаи 2 и
// 3); сброс возвращает main (случай 4).
func TestBisect(t *testing.T) {
	repo := eightCommits(t)
	c1, c4 := revision(t, repo, "HEAD~7"), revision(t, repo, "HEAD~4")

	step, err := open(t, repo).BisectMark("HEAD", false)
	if err != nil || step.Commit != "" || step.Found {
		t.Fatalf("первая отметка = %+v, %v, ожидалось ожидание второй", step, err)
	}
	if got := operationOf(t, repo); got != "bisect" {
		t.Fatalf("Operation = %q, ожидался bisect", got)
	}

	step, err = open(t, repo).BisectMark(c1, true)
	if err != nil || step.Commit != c4 || step.Subject != "c4" || step.Left != 3 {
		t.Fatalf("вторая отметка = %+v, %v, ожидался переход на c4", step, err)
	}

	for _, good := range []bool{false, true} {
		if step, err = open(t, repo).BisectMark("HEAD", good); err != nil {
			t.Fatalf("BisectMark: %v", err)
		}
	}
	if !step.Found || step.Commit != c4 || step.Subject != "c4" {
		t.Fatalf("итог = %+v, ожидался найденный c4", step)
	}

	if err := open(t, repo).BisectReset(); err != nil {
		t.Fatalf("BisectReset: %v", err)
	}
	if got := runGit(t, repo, "branch", "--show-current"); got != "main\n" {
		t.Fatalf("после сброса ветка %q, ожидалась main", got)
	}
	if got := operationOf(t, repo); got != "" {
		t.Fatalf("Operation = %q после сброса", got)
	}
}

// Под русской локалью число оставшихся коммитов то же: вывод git разбирается
// по-английски (#300, случай 7).
func TestBisectInOtherLanguage(t *testing.T) {
	// newRepo ставит LC_ALL=C, тест её переопределяет; без русской локали в
	// системе git говорит по-английски, и тест проходит без проверки
	repo := eightCommits(t)
	t.Setenv("LANGUAGE", "ru")
	t.Setenv("LC_ALL", "ru_RU.UTF-8")

	if _, err := open(t, repo).BisectMark("HEAD", false); err != nil {
		t.Fatalf("BisectMark: %v", err)
	}
	step, err := open(t, repo).BisectMark("HEAD~7", true)
	if err != nil || step.Left != 3 || step.Subject != "c4" {
		t.Fatalf("вторая отметка = %+v, %v, ожидались c4 и 3", step, err)
	}
}

// Грязное дерево git не даёт перейти на коммит для проверки (случай 5).
func TestBisectDirtyTree(t *testing.T) {
	repo := eightCommits(t)
	c1 := revision(t, repo, "HEAD~7")
	write(t, repo, "a.txt", "dirty\n")

	if _, err := open(t, repo).BisectMark("HEAD", false); err != nil {
		t.Fatalf("первая отметка: %v", err)
	}
	if _, err := open(t, repo).BisectMark(c1, true); err == nil {
		t.Fatal("bisect перешёл на коммит поверх грязного дерева")
	}
}

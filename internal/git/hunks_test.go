package git_test

import (
	"path/filepath"
	"strings"
	"testing"

	"gitone/internal/diff"
	"gitone/internal/git"
)

// hunkRepo это файл из десяти строк, в котором правлены первая и девятая:
// дифф из двух хунков.
func hunkRepo(t *testing.T) string {
	t.Helper()

	repo := newRepo(t)
	lines := []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}
	write(t, repo, "a.txt", strings.Join(lines, "\n")+"\n")
	commitAll(t, repo)
	lines[0], lines[8] = "ONE", "NINE"
	write(t, repo, "a.txt", strings.Join(lines, "\n")+"\n")
	return repo
}

func selectRefs(refs ...diff.LineRef) map[diff.LineRef]bool {
	selected := map[diff.LineRef]bool{}
	for _, ref := range refs {
		selected[ref] = true
	}
	return selected
}

// Патч второго хунка уходит в индекс, первый остаётся в рабочем дереве
// (#212, случай 1); второй раз тот же патч git не принимает.
func TestApplyPatchStagesHunk(t *testing.T) {
	repo := hunkRepo(t)

	text, err := git.UnstagedDiff(repo, "a.txt")
	if err != nil {
		t.Fatalf("UnstagedDiff: %v", err)
	}
	patch := diff.Patch(text, selectRefs(diff.LineRef{Kind: diff.LineRemoved, Number: 9, Text: "nine"}, diff.LineRef{Kind: diff.LineAdded, Number: 9, Text: "NINE"}), false)
	if err := git.ApplyPatch(repo, patch, true, false); err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}

	cached := runGit(t, repo, "diff", "--cached")
	if !strings.Contains(cached, "+NINE") || strings.Contains(cached, "ONE") {
		t.Fatalf("в индексе не только второй хунк:\n%s", cached)
	}
	if unstaged := runGit(t, repo, "diff"); !strings.Contains(unstaged, "+ONE") || strings.Contains(unstaged, "NINE") {
		t.Fatalf("в рабочем дереве не только первый хунк:\n%s", unstaged)
	}

	if err := git.ApplyPatch(repo, patch, true, false); err == nil || !strings.HasPrefix(err.Error(), "apply: ") {
		t.Fatalf("повторный патч = %v, ожидался отказ git", err)
	}
	if paths, _ := git.UnstagedPaths(repo); len(paths) != 1 || paths[0] != "a.txt" {
		t.Fatalf("UnstagedPaths = %q", paths)
	}
}

// Отдельная строка хунка уходит в индекс, соседняя остаётся (случай 2), и
// следующая строка того же хунка ставится поверх первой.
func TestApplyPatchStagesLines(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "one\nfirst\nsecond\n")

	lines := map[int]string{2: "first", 3: "second"}
	for _, number := range []int{3, 2} {
		text, _ := git.UnstagedDiff(repo, "a.txt")
		patch := diff.Patch(text, selectRefs(diff.LineRef{Kind: diff.LineAdded, Number: number, Text: lines[number]}), false)
		if err := git.ApplyPatch(repo, patch, true, false); err != nil {
			t.Fatalf("строка %d: %v", number, err)
		}
		if number == 3 {
			if got := runGit(t, repo, "show", ":a.txt"); got != "one\nsecond\n" {
				t.Fatalf("индекс %q, ожидалась только вторая добавленная строка", got)
			}
		}
	}

	if got := runGit(t, repo, "show", ":a.txt"); got != "one\nfirst\nsecond\n" {
		t.Fatalf("индекс %q после обеих строк", got)
	}
}

// Строка без перевода в конце файла не склеивается со следующей: пометка
// «No newline» в патче стоит только за последней строкой своей стороны
// (#288, случаи 1–3).
func TestApplyPatchNoNewline(t *testing.T) {
	cases := []struct {
		name     string
		index    string
		worktree string
		selected diff.LineRef
		reverse  bool
		want     string
	}{
		{"добавленная за строкой без перевода", "a\nb", "a\nb\nc", diff.LineRef{Kind: diff.LineAdded, Number: 3, Text: "c"}, false, "a\nb\nc"},
		{"замена без перевода", "a\nb", "a\nB", diff.LineRef{Kind: diff.LineAdded, Number: 2, Text: "B"}, false, "a\nb\nB"},
		{"отмена удалённой без перевода", "a\nb", "a\nB", diff.LineRef{Kind: diff.LineRemoved, Number: 2, Text: "b"}, true, "a\nb\nB"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			write(t, repo, "a.txt", tc.index)
			commitAll(t, repo)
			write(t, repo, "a.txt", tc.worktree)

			text, _ := git.UnstagedDiff(repo, "a.txt")
			patch := diff.Patch(text, selectRefs(tc.selected), tc.reverse)
			if err := git.ApplyPatch(repo, patch, !tc.reverse, tc.reverse); err != nil {
				t.Fatalf("ApplyPatch: %v\n%s", err, patch)
			}

			got := runGit(t, repo, "show", ":a.txt")
			if tc.reverse {
				got = readFile(t, filepath.Join(repo, "a.txt"))
			}
			if got != tc.want {
				t.Fatalf("результат %q, ожидалось %q\n%s", got, tc.want, patch)
			}
		})
	}
}

// Обратный патч хунка возвращает его в рабочем дереве к индексу, второй хунк
// остаётся (#213, случай 1); обратный патч строки трогает только её (случай 2).
func TestApplyPatchDiscards(t *testing.T) {
	repo := hunkRepo(t)

	text, _ := git.UnstagedDiff(repo, "a.txt")
	patch := diff.Patch(text, selectRefs(diff.LineRef{Kind: diff.LineRemoved, Number: 1, Text: "one"}, diff.LineRef{Kind: diff.LineAdded, Number: 1, Text: "ONE"}), true)
	if err := git.ApplyPatch(repo, patch, false, true); err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); !strings.HasPrefix(got, "one\n") || !strings.Contains(got, "NINE\n") {
		t.Fatalf("a.txt = %q, ожидался откат только первого хунка", got)
	}

	write(t, repo, "a.txt", "one\nfirst\nsecond\n")
	runGit(t, repo, "add", "a.txt")
	write(t, repo, "a.txt", "one\nfirst\nextra\nsecond\nmore\n")
	text, _ = git.UnstagedDiff(repo, "a.txt")
	patch = diff.Patch(text, selectRefs(diff.LineRef{Kind: diff.LineAdded, Number: 3, Text: "extra"}), true)
	if err := git.ApplyPatch(repo, patch, false, true); err != nil {
		t.Fatalf("ApplyPatch строки: %v", err)
	}
	if got := readFile(t, filepath.Join(repo, "a.txt")); got != "one\nfirst\nsecond\nmore\n" {
		t.Fatalf("a.txt = %q, ожидался откат одной строки", got)
	}
}

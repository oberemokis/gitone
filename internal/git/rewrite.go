package git

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gitone/internal/i18n"
)

// Rewrite это правка истории текущей ветки над одним коммитом (#219).
type Rewrite string

const (
	// RewriteSquash сливает коммит в предыдущий, сообщения склеиваются
	RewriteSquash Rewrite = "squash"
	// RewriteFixup сливает коммит в предыдущий, его сообщение отбрасывается
	RewriteFixup Rewrite = "fixup"
	// RewriteDrop убирает коммит из истории
	RewriteDrop Rewrite = "drop"
	// RewriteReword меняет сообщение коммита
	RewriteReword Rewrite = "reword"
	// RewriteMoveUp меняет коммит местами со следующим, более новым
	RewriteMoveUp Rewrite = "up"
	// RewriteMoveDown меняет коммит местами с предыдущим, более старым
	RewriteMoveDown Rewrite = "down"
)

// RewriteHistory переписывает историю текущей ветки интерактивным rebase
// (#219), как действия над коммитами в lazygit. Список действий gitone
// собирает сам и подставляет вместо редактора (GIT_SEQUENCE_EDITOR), сообщение
// reword подставляет вместо редактора сообщений (GIT_EDITOR), у squash
// редактор ничего не меняет и сообщения просто склеиваются. Незакоммиченные
// правки откладываются в stash и возвращаются (--autostash). Merge-коммиты
// между целью и HEAD отвергаются: без --rebase-merges git их бы распрямил.
// published говорит, что переписаны коммиты из upstream. Конфликт останавливает
// rebase, ошибка тогда git.stoppedOnConflict.
func RewriteHistory(dir, hash string, action Rewrite, message string) (published bool, err error) {
	published, err = rewriteHistory(dir, hash, action, message)
	return published, failed("rebase", err)
}

func rewriteHistory(dir, hash string, action Rewrite, message string) (bool, error) {
	root, err := topLevel(dir)
	if err != nil {
		return false, err
	}

	commits, err := branchCommits(root)
	if err != nil {
		return false, err
	}
	at := slices.Index(commits, hash)
	if at < 0 {
		return false, i18n.New("git.rewriteNotInBranch", nil)
	}

	// first это самый старый коммит, который меняется: от его родителя и идёт
	// rebase
	first := at
	switch action {
	case RewriteSquash, RewriteFixup, RewriteMoveDown:
		if at == 0 {
			return false, i18n.New("git.rewriteNoParent", nil)
		}
		first = at - 1
	case RewriteMoveUp:
		if at == len(commits)-1 {
			return false, i18n.New("git.rewriteNoChild", nil)
		}
	}

	base, published := baseOf(root, commits[first])
	rewritten := "HEAD"
	if base != "" {
		rewritten = base + "..HEAD"
	}
	if merges, _ := run(call{dir: root}, "rev-list", "--merges", rewritten); strings.TrimSpace(merges) != "" {
		return false, i18n.New("git.rewriteMerges", nil)
	}

	return published, rebaseWith(root, base, todoLines(commits[first:], at-first, action), message)
}

// branchCommits это коммиты текущей ветки от старых к новым, первый
// родитель на каждом шаге.
func branchCommits(root string) ([]string, error) {
	out, err := run(call{dir: root}, "rev-list", "--first-parent", "--reverse", "HEAD")
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// todoLines это список действий rebase для commits, правка над коммитом под
// номером at.
func todoLines(commits []string, at int, action Rewrite) []string {
	lines := make([]string, len(commits))
	for index, commit := range commits {
		lines[index] = "pick " + commit
	}

	switch action {
	case RewriteSquash, RewriteFixup, RewriteDrop, RewriteReword:
		lines[at] = string(action) + " " + commits[at]
	case RewriteMoveUp:
		lines[at], lines[at+1] = lines[at+1], lines[at]
	case RewriteMoveDown:
		lines[at-1], lines[at] = lines[at], lines[at-1]
	}
	return lines
}

// baseOf это родитель коммита first, от которого идёт rebase, или пустая
// строка у корневого коммита: тогда rebase идёт с --root. published говорит,
// что переписанная часть ветки уже есть в upstream: с корня переписывается
// вся ветка, и опубликована она, если у неё с upstream есть общий коммит.
func baseOf(root, first string) (base string, published bool) {
	parent, err := run(call{dir: root}, "rev-parse", "--verify", "--quiet", first+"^")
	if err != nil {
		_, shared := run(call{dir: root}, "merge-base", "HEAD", "@{upstream}")
		return "", shared == nil
	}
	base = strings.TrimSpace(parent)
	return base, dropsPublished(root, base)
}

// rebaseWith запускает `git rebase -i` со своим списком действий и
// сообщением reword. Оба файла лежат во временном каталоге и подставляются
// командой cp вместо редактора: git зовёт редактор через sh с путём файла.
// Сообщение reword git не чистит (commit.cleanup=whitespace): строки с «#»
// остаются, как у Commit и Amend через -m. Без своего списка git строит его
// сам с --autosquash, и он принимается как есть.
func rebaseWith(root, base string, todo []string, message string) error {
	temp, err := os.MkdirTemp("", "gitone-rebase-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)

	args := []string{"rebase", "-i", "--autostash"}
	sequence := "true"
	if todo == nil {
		args = append(args, "--autosquash")
	} else {
		todoFile := filepath.Join(temp, "todo")
		if err := os.WriteFile(todoFile, []byte(strings.Join(todo, "\n")+"\n"), 0o600); err != nil {
			return err
		}
		sequence = "cp " + shellQuote(todoFile)
	}

	env := []string{"GIT_SEQUENCE_EDITOR=" + sequence, "GIT_EDITOR=true"}
	if message != "" {
		messageFile := filepath.Join(temp, "message")
		if err := os.WriteFile(messageFile, []byte(message+"\n"), 0o600); err != nil {
			return err
		}
		env = []string{"GIT_SEQUENCE_EDITOR=" + sequence, "GIT_EDITOR=cp " + shellQuote(messageFile),
			"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=commit.cleanup", "GIT_CONFIG_VALUE_0=whitespace"}
	}

	if base == "" {
		args = append(args, "--root")
	} else {
		args = append(args, base)
	}

	_, err = run(call{dir: root, env: env}, args...)
	if err != nil && unfinished(root) {
		return i18n.New("git.stoppedOnConflict", i18n.Args{"operation": "rebase"})
	}
	return headline(err)
}

// Fixup создаёт из индекса коммит `fixup! <тема hash>` (#225), как
// `git commit --fixup`: Autosquash потом сольёт его в hash. Пустой индекс и
// коммит не из текущей ветки отвергаются, git сам бы создал коммит и на чужой.
func Fixup(dir, hash string) error {
	return failed("commit", fixup(dir, hash))
}

func fixup(dir, hash string) error {
	root, err := topLevel(dir)
	if err != nil {
		return err
	}

	if _, err := run(call{dir: root}, "merge-base", "--is-ancestor", hash, "HEAD"); err != nil {
		return i18n.New("git.rewriteNotInBranch", nil)
	}
	if _, err := run(call{dir: root}, "diff", "--cached", "--quiet"); err == nil {
		return i18n.New("git.nothingToCommit", nil)
	}

	_, err = run(call{dir: root}, "commit", "--fixup="+hash)
	return err
}

// Autosquash сливает коммиты `fixup!` и `squash!` в их цели (#225), начиная с
// hash: `git rebase -i --autosquash` от его родителя, список действий git
// строит сам. published говорит, что переписаны коммиты из upstream.
func Autosquash(dir, hash string) (published bool, err error) {
	published, err = autosquash(dir, hash)
	return published, failed("rebase", err)
}

func autosquash(dir, hash string) (bool, error) {
	root, err := topLevel(dir)
	if err != nil {
		return false, err
	}

	base, published := baseOf(root, hash)
	return published, rebaseWith(root, base, nil, "")
}

// shellQuote берёт путь в одинарные кавычки для sh: пробел в каталоге
// пользователя не разорвёт команду редактора.
func shellQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

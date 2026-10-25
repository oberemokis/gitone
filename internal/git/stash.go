package git

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gitone/internal/i18n"
)

// Stash это запись stash: ссылка вида stash@{N}, хэш коммита stash и подпись
// git. Ссылка позиционная и сдвигается от любого git stash push, поэтому
// запись держит хэш: по нему находится именно тот stash, что видел пользователь.
type Stash struct {
	Ref     string
	Hash    string
	Subject string
}

// Stashes возвращает stash от новых к старым.
func (r Repo) Stashes() ([]Stash, error) {
	stashes, err := r.stashList()
	return stashes, failed("stash list", err)
}

func (r Repo) stashList() ([]Stash, error) {
	out, err := r.inRoot("stash", "list", "--format=%gd%x1f%H%x1f%gs")
	if err != nil {
		return nil, err
	}

	var stashes []Stash
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.SplitN(line, "\x1f", 3)
		if len(fields) < 3 || fields[0] == "" {
			continue
		}

		stashes = append(stashes, Stash{Ref: fields[0], Hash: fields[1], Subject: fields[2]})
	}

	return stashes, nil
}

// StashOptions это параметры stash (#236). Нулевое значение сохраняет
// отслеженные правки с сообщением git по умолчанию. Untracked берёт и
// неотслеженные файлы (`-u`), KeepIndex оставляет индекс на месте
// (`--keep-index`), пустое Message оставляет сообщение git.
type StashOptions struct {
	Message   string
	Untracked bool
	KeepIndex bool
}

// StashPush сохраняет правки в stash. Неотслеженные файлы без Untracked не
// трогаются. Без правок git сообщает об успехе, поэтому проверяем сами: иначе
// пользователь думал бы, что правки сохранены. Подмодули в проверке не
// считаются: git stash push их игнорирует и на одном сдвинутом указателе
// отвечает «No local changes to save» с кодом 0.
func (r Repo) StashPush(opts StashOptions) error {
	return failed("stash push", r.stashPush(opts))
}

func (r Repo) stashPush(opts StashOptions) error {
	if !hasChanges(r.root, opts.Untracked) {
		return i18n.New("git.nothingToStash", nil)
	}

	args := []string{"stash", "push"}
	if opts.Untracked {
		args = append(args, "--include-untracked")
	}
	if opts.KeepIndex {
		args = append(args, "--keep-index")
	}
	if opts.Message != "" {
		args = append(args, "--message", opts.Message)
	}

	_, err := r.inRoot(args...)
	return err
}

// hasChanges говорит, есть ли что сохранить в stash: отслеженные правки, а с
// untracked ещё и неотслеженные файлы.
func hasChanges(root string, untracked bool) bool {
	if _, err := run(call{dir: root}, "diff", "--quiet", "--ignore-submodules", "HEAD", "--"); err != nil {
		return true
	}
	if !untracked {
		return false
	}

	listed, err := run(call{dir: root}, "ls-files", "--others", "--exclude-standard")
	return err == nil && strings.TrimSpace(listed) != ""
}

// StashApply применяет stash и оставляет его в списке. Если stash@{N} уже
// другой коммит, чем у строки списка, отказывает и ничего не применяет. Отказ
// git сводится к одной строке: при конфликте git начинает вывод с
// «Auto-merging a.txt», и причина стоит в нём не первой.
func (r Repo) StashApply(stash Stash) error {
	return failed("stash apply", headline(r.stashAt(stash, "apply")))
}

// StashPop применяет stash и удаляет его одним действием, `git stash pop`
// (#235). На конфликте git правки применяет с маркерами, а запись оставляет:
// отказ тогда говорит именно это, строка CONFLICT называла бы один файл.
// Правку, которой мешает дерево, git отвергает до начала, и stash остаётся.
// Неслитые пути до pop (идёт слияние с конфликтом) отказывают до git: иначе
// его отказ выглядел бы конфликтом самого pop (#388).
func (r Repo) StashPop(stash Stash) error {
	if r.conflicted() {
		return failed("stash pop", i18n.New("git.stashUnmerged", nil))
	}

	err := headline(r.stashAt(stash, "pop"))
	if err != nil && r.conflicted() {
		err = i18n.New("git.stashKept", nil)
	}
	return failed("stash pop", err)
}

// StashRename меняет сообщение записи stash, порядок записей остаётся (#237).
// Править запись на месте git не умеет: записи от верхней до этой снимаются и
// кладутся обратно `git stash store` в прежнем порядке, эта с новым
// сообщением. Коммиты stash остаются те же. Пустое сообщение отвергается,
// запись, сдвинутая чужим git stash push, тоже.
func (r Repo) StashRename(stash Stash, message string) error {
	return failed("stash rename", r.stashRename(stash, message))
}

func (r Repo) stashRename(stash Stash, message string) error {
	if strings.TrimSpace(message) == "" {
		return i18n.New("git.emptyStashMessage", nil)
	}

	stashes, err := r.stashList()
	if err != nil {
		return err
	}
	index := slices.Index(stashes, stash)
	if index < 0 {
		return i18n.New("git.stashChanged", nil)
	}

	// ponytail: сбой посредине оставит снятые записи только в объектах git, их
	// хэши тогда находит git fsck --unreachable. Откат не пишем: и drop, и store
	// отказывают разве что при сломанном репозитории
	above := stashes[:index+1]
	for range above {
		if _, err := r.inRoot("stash", "drop", "--quiet", "stash@{0}"); err != nil {
			return err
		}
	}
	for at := index; at >= 0; at-- {
		subject := above[at].Subject
		if at == index {
			subject = message
		}
		if _, err := r.inRoot("stash", "store", "--message", subject, above[at].Hash); err != nil {
			return err
		}
	}

	return nil
}

// conflicted говорит, что в индексе есть неслитые пути.
func (r Repo) conflicted() bool {
	out, err := r.inRoot("diff", "--name-only", "--diff-filter=U")
	return err == nil && strings.TrimSpace(out) != ""
}

// headline оставляет в отказе git одну строку: первую с CONFLICT, fatal: или
// error:, а без неё первую вообще. Причина стоит не в начале: git начинает
// вывод со строки «Auto-merging» (stash apply), отчёта о fetch «From …»
// (pull) или «To …» (push), и подсказки hint: идут до итоговой строки. При
// сбое транспорта итоговая строка ничего не объясняет (unreadable), а причина,
// например отказ ssh по ключу, стоит строкой выше: берётся она. Код выхода
// остаётся в цепочке.
func headline(err error) error {
	var refusal *gitError
	if !errors.As(err, &refusal) {
		return err
	}

	// строка прогресса («Rebasing (1/1)») затирается возвратом каретки, а не
	// переводом строки: \r тоже граница строки
	lines := strings.FieldsFunc(refusal.message, func(r rune) bool { return r == '\n' || r == '\r' })
	reason := slices.IndexFunc(lines, func(line string) bool {
		return strings.HasPrefix(line, "CONFLICT") || strings.HasPrefix(line, "fatal:") || strings.HasPrefix(line, "error:")
	})
	if reason > 0 && lines[reason] == unreadable {
		reason--
	}

	return &gitError{message: lines[max(reason, 0)], exit: refusal.exit}
}

// unreadable это итоговая строка git, когда транспорт не смог прочитать
// remote: ssh отказал, нет сети, нет доступа.
const unreadable = "fatal: Could not read from remote repository."

// StashDrop удаляет stash без применения. Вернуть его можно только через
// reflog, поэтому удаление спрашивается заранее. Если stash@{N} уже другой
// коммит, чем у строки списка, отказывает и ничего не удаляет.
func (r Repo) StashDrop(stash Stash) error {
	return failed("stash drop", r.stashAt(stash, "drop"))
}

// stashAt выполняет git stash <действие> stash@{N}, только пока ссылка указывает
// на коммит из записи. refs/stash общий для всех worktree: git stash push из
// соседнего терминала сдвигает номера, и позиция из списка попала бы в чужой
// stash. Между проверкой и действием остаётся окно в миллисекунды, закрыть его
// git не даёт: drop принимает только ссылку.
func (r Repo) stashAt(stash Stash, action string) error {
	hash, err := r.inRoot("rev-parse", "--verify", "--quiet", stash.Ref)
	if err != nil || strings.TrimSpace(hash) != stash.Hash {
		return i18n.New("git.stashChanged", nil)
	}

	_, err = r.inRoot("stash", action, stash.Ref)
	return err
}

// StashTree это то, с чем сравнивать родителя stash в его диффе: сам stash, а
// если в нём неотслеженные файлы (третий родитель, stash -u), дерево stash
// вместе с ними. git diff по самому stash неотслеженных не видит. Дерево
// собирается во временном индексе, настоящий индекс не трогается.
func (r Repo) StashTree(hash string) (string, error) {
	tree, err := r.stashTree(hash)
	return tree, failed("stash", err)
}

func (r Repo) stashTree(hash string) (string, error) {
	untracked, err := r.inRoot("ls-tree", "-r", "-z", hash+"^3")
	if err != nil {
		return hash, nil
	}

	temp, err := os.MkdirTemp("", "gitone-stash-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)

	index := call{dir: r.root, env: []string{"GIT_INDEX_FILE=" + filepath.Join(temp, "index")}}
	if _, err := run(index, "read-tree", hash); err != nil {
		return "", err
	}
	index.stdin = untracked
	if _, err := run(index, "update-index", "-z", "--index-info"); err != nil {
		return "", err
	}
	index.stdin = ""
	tree, err := run(index, "write-tree")
	return strings.TrimSpace(tree), err
}

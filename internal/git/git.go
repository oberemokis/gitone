package git

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Ставятся до подкоманды. Пути в UTF-8 без восьмеричных кавычек. Файл больше
// 10 МБ git отдаёт как бинарный, без содержимого: неотслеженный CSV на 300 МБ
// при разборе занимал 1,6 ГБ памяти.
var configFlags = []string{"-c", "core.quotePath=false", "-c", "core.bigFileThreshold=10m"}

// Отключают то, что пользователь мог включить в ~/.gitconfig и что ломает
// разбор: внешний дифф, цвет, пути без префиксов, относительные пути, лог
// подмодулей вместо строки коммита. --full-index даёт полные хэши версий в
// строке index: по короткому git может не найти объект однозначно (#140).
var diffFlags = []string{
	"--no-color", "--no-ext-diff", "--no-relative", "--submodule=short",
	"--src-prefix=a/", "--dst-prefix=b/", "--full-index",
}

// Diff возвращает текст `git diff` для каталога. С аргументами они уходят в
// git как есть (`--staged`, `main..feature`, коммит, пути). Без аргументов
// это всё незакоммиченное вместе с неотслеженными файлами. flags это флаги вида
// (#217): `--ignore-all-space`, `--unified=N`. Они идут перед аргументами и не
// переводят дифф рабочего дерева в дифф по аргументам.
func Diff(dir string, args []string, flags ...string) (string, error) {
	text, err := diffText(dir, args, flags)
	return text, failed("diff", err)
}

func diffText(dir string, args, flags []string) (string, error) {
	if len(args) > 0 {
		return run(call{dir: dir, diffExit: true}, diffCommand(flags, args...)...)
	}

	root, err := topLevel(dir)
	if err != nil {
		return "", err
	}

	index, cleanup, err := indexWithUntracked(root)
	if err != nil {
		return "", err
	}
	defer cleanup()

	base, err := baseRevision(dir)
	if err != nil {
		return "", err
	}

	// `--` отделяет ревизию от путей: без него файл с именем HEAD в рабочем
	// дереве делает вызов неоднозначным.
	return run(call{dir: dir, env: []string{"GIT_INDEX_FILE=" + index}}, diffCommand(flags, base, "--")...)
}

// baseRevision это HEAD, а в репозитории без коммитов пустое дерево: тогда
// всё индексированное и неотслеженное показывается добавленным. Хеш пустого
// дерева спрашивается у git, он разный у репозиториев на SHA-1 и SHA-256.
func baseRevision(dir string) (string, error) {
	if hasCommits(dir) {
		return "HEAD", nil
	}

	emptyTree, err := run(call{dir: dir}, "hash-object", "-t", "tree", "--stdin")
	return strings.TrimSuffix(emptyTree, "\n"), err
}

// hasCommits говорит, есть ли у репозитория HEAD: до первого коммита его нет.
func hasCommits(dir string) bool {
	_, err := run(call{dir: dir}, "rev-parse", "--verify", "--quiet", "HEAD")
	return err == nil
}

func diffCommand(flags []string, args ...string) []string {
	return slices.Concat([]string{"diff"}, diffFlags, flags, args)
}

// indexWithUntracked копирует индекс во временный каталог и добавляет в копию
// неотслеженные файлы как намерение (`add -N`). Один `git diff` по копии
// показывает их вместе с остальными и видит переименование в рабочем дереве,
// а настоящий индекс остаётся нетронутым. Работает из корня репозитория:
// из подкаталога ls-files видит только его, и файлы снаружи пропали бы.
func indexWithUntracked(root string) (string, func(), error) {
	listed, err := run(call{dir: root}, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", nil, err
	}

	realIndex, err := run(call{dir: root}, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return "", nil, err
	}

	temp, err := os.MkdirTemp("", "gitone-index-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(temp) }

	index := filepath.Join(temp, "index")
	if err := fillIndex(root, strings.TrimSpace(realIndex), index, withoutDirectories(listed)); err != nil {
		cleanup()
		return "", nil, err
	}

	return index, cleanup, nil
}

// fillIndex собирает копию индекса: содержимое настоящего плюс untracked как
// намерение.
func fillIndex(root, realIndex, index, untracked string) error {
	realTime, err := copyIndex(realIndex, index)
	if err != nil {
		return err
	}

	if untracked != "" {
		addCall := call{dir: root, env: []string{"GIT_INDEX_FILE=" + index, literalPathspecs}, stdin: untracked}
		if _, err := run(addCall, "add", "--intent-to-add", "--pathspec-from-file=-", "--pathspec-file-nul"); err != nil {
			return err
		}
	}

	// git решает по времени записи индекса, какие записи могли измениться в ту же
	// тику и требуют сверки содержимого (racy git). Копия после add получает
	// время «сейчас», и правка с тем же размером в ту же тику пропадала. Время
	// исходного индекса возвращаем копии, тогда git сверяет те же записи, что и
	// сверил бы настоящий индекс.
	if realTime.IsZero() {
		return nil
	}
	return os.Chtimes(index, realTime, realTime)
}

// copyIndex копирует индекс и возвращает время его записи. Индекса ещё нет в
// свежем репозитории без единого add: тогда копии нет, и git создаст её сам.
// Пустой файл вместо него git отвергает.
func copyIndex(from, to string) (time.Time, error) {
	content, err := os.ReadFile(from)
	if errors.Is(err, fs.ErrNotExist) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}

	var modTime time.Time
	if info, err := os.Stat(from); err == nil {
		modTime = info.ModTime()
	}

	return modTime, os.WriteFile(to, content, 0o600)
}

// withoutDirectories убирает из вывода `ls-files -z` записи на `/`. Так
// ls-files называет вложенный репозиторий или worktree, и `add -N` на нём
// падает, а с ним вся загрузка.
func withoutDirectories(listed string) string {
	var kept []string
	for _, entry := range strings.Split(listed, "\x00") {
		if entry != "" && !strings.HasSuffix(entry, "/") {
			kept = append(kept, entry)
		}
	}

	return strings.Join(kept, "\x00")
}

// topLevel возвращает корень репозитория, в котором лежит dir. Пути в diff идут
// от корня, а gitone может быть запущен из подкаталога, поэтому команды правки
// идут из корня: иначе относительный путь указывал бы не на тот файл.
func topLevel(dir string) (string, error) {
	root, err := run(call{dir: dir}, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(root, "\n"), nil
}

// inRoot запускает одну команду git из корня репозитория, в котором лежит dir.
func inRoot(dir string, args ...string) (string, error) {
	root, err := topLevel(dir)
	if err != nil {
		return "", err
	}

	return run(call{dir: root}, args...)
}

type call struct {
	// ctx отменяет команду, убивая git вместе с детьми (stopGroup). Нужен долгим
	// вызовам, которые ходят по сети: pull и push. nil значит без отмены.
	ctx   context.Context
	dir   string
	env   []string
	stdin string
	// diffExit читает код выхода 1 с выводом как успех. Так отвечает `git diff`
	// с --no-index и --exit-code, когда различия есть: дифф лежит на stdout.
	// Нужен только вызову, куда пользователь передаёт свои аргументы.
	diffExit bool
}

// literalPathspecs заставляет git читать пути-аргументы как имена файлов, а не
// как шаблоны. Путь из диффа `app/[id]/page.tsx` без этого совпал бы и с
// `app/d/page.tsx`, а `:note.md` читался бы как магия pathspec. Включается
// только вызовам, куда пути отдаёт сам gitone: то, что пользователь пишет в
// `gitone -- 'src/*.go'`, остаётся шаблоном.
const literalPathspecs = "GIT_LITERAL_PATHSPECS=1"

// literal это вызов из корня репозитория с буквальными путями.
func literal(root string) call {
	return call{dir: root, env: []string{literalPathspecs}}
}

// gitError это отказ git. Текст из вывода git нужен пользователю: «not a git
// repository», а не «exit status 128». Исходная ошибка процесса остаётся в
// цепочке, и код выхода находится через errors.As.
type gitError struct {
	message string
	exit    *exec.ExitError
}

func (e *gitError) Error() string {
	return e.message
}

func (e *gitError) Unwrap() error {
	return e.exit
}

// failed добавляет к ошибке имя операции git. Вызывают его только
// экспортируемые функции: внутри пакета ошибка идёт как есть, поэтому имя
// операции в тексте одно. nil остаётся nil.
func failed(operation string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", operation, err)
}

// run запускает git и возвращает его вывод. Отказ git приходит как *gitError.
func run(options call, args ...string) (string, error) {
	args = append(append([]string{}, configFlags...), args...)
	command := exec.Command("git", args...)
	if options.ctx != nil {
		command = exec.CommandContext(options.ctx, "git", args...)
		stopGroup(command)
	}
	command.Dir = options.dir
	command.Stdin = strings.NewReader(options.stdin)
	// GIT_OPTIONAL_LOCKS=0: чтение диффа не берёт index.lock, иначе
	// параллельный git add пользователя падает. GIT_TERMINAL_PROMPT=0: git не
	// спрашивает пароль поверх интерфейса.
	command.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	command.Env = append(command.Env, options.env...)

	out, err := command.Output()

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		if options.diffExit && exitError.ExitCode() == 1 && len(out) > 0 {
			return string(out), nil
		}

		// часть сообщений git пишет в stdout: конфликт `stash apply` идёт туда.
		// Отказ без единой строки (хук с `exit 1`) называется кодом выхода.
		message := cmp.Or(strings.TrimSpace(string(exitError.Stderr)), strings.TrimSpace(string(out)), exitError.Error())
		return "", &gitError{message: message, exit: exitError}
	}

	return string(out), err
}

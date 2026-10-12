package git

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Ставятся до подкоманды. Пути в UTF-8 без восьмеричных кавычек. Файл больше
// 10 МБ git отдаёт как бинарный, без содержимого: неотслеженный CSV на 300 МБ
// при разборе занимал 1,6 ГБ памяти.
var configFlags = []string{"-c", "core.quotePath=false", "-c", "core.bigFileThreshold=10m"}

// Отключают то, что пользователь мог включить в ~/.gitconfig и что ломает
// разбор: внешний дифф, цвет, пути без префиксов, относительные пути, лог
// подмодулей вместо строки коммита.
var diffFlags = []string{
	"--no-color", "--no-ext-diff", "--no-relative", "--submodule=short",
	"--src-prefix=a/", "--dst-prefix=b/",
}

// Diff возвращает текст `git diff` для каталога. С аргументами они уходят в
// git как есть (`--staged`, `main..feature`, коммит, пути). Без аргументов
// это всё незакоммиченное вместе с неотслеженными файлами.
func Diff(dir string, args []string) (string, error) {
	if len(args) > 0 {
		return run(call{dir: dir}, diffCommand(args...)...)
	}

	root, err := run(call{dir: dir}, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}

	index, cleanup, err := indexWithUntracked(strings.TrimSuffix(root, "\n"))
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
	return run(call{dir: dir, env: []string{"GIT_INDEX_FILE=" + index}}, diffCommand(base, "--")...)
}

// baseRevision это HEAD, а в репозитории без коммитов пустое дерево: тогда
// всё индексированное и неотслеженное показывается добавленным. Хеш пустого
// дерева спрашивается у git, он разный у репозиториев на SHA-1 и SHA-256.
func baseRevision(dir string) (string, error) {
	if _, err := run(call{dir: dir}, "rev-parse", "--verify", "--quiet", "HEAD"); err == nil {
		return "HEAD", nil
	}

	emptyTree, err := run(call{dir: dir}, "hash-object", "-t", "tree", "--stdin")
	return strings.TrimSuffix(emptyTree, "\n"), err
}

func diffCommand(args ...string) []string {
	return append(append([]string{"diff"}, diffFlags...), args...)
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

	// Индекса ещё нет в свежем репозитории без единого add: тогда git
	// создаст копию сам. Пустой файл вместо него git отвергает.
	content, err := os.ReadFile(strings.TrimSpace(realIndex))
	switch {
	case err == nil:
		err = os.WriteFile(index, content, 0o600)
	case errors.Is(err, fs.ErrNotExist):
		err = nil
	}
	if err != nil {
		cleanup()
		return "", nil, err
	}

	if untracked := withoutDirectories(listed); untracked != "" {
		addCall := call{dir: root, env: []string{"GIT_INDEX_FILE=" + index}, stdin: untracked}
		if _, err := run(addCall, "add", "--intent-to-add", "--pathspec-from-file=-", "--pathspec-file-nul"); err != nil {
			cleanup()
			return "", nil, err
		}
	}

	return index, cleanup, nil
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

type call struct {
	dir   string
	env   []string
	stdin string
}

// run запускает git и возвращает его вывод. Ошибка несёт текст из stderr git,
// а не код выхода: пользователю нужно «not a git repository», а не
// «exit status 128».
func run(options call, args ...string) (string, error) {
	command := exec.Command("git", append(append([]string{}, configFlags...), args...)...)
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
		// часть сообщений git пишет в stdout: «nothing to commit» идёт туда же
		message := strings.TrimSpace(string(exitError.Stderr))
		if message == "" {
			message = strings.TrimSpace(string(out))
		}
		return "", errors.New(message)
	}

	return string(out), err
}

package git

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	"gitone/internal/i18n"
)

// Stage добавляет пути в индекс. Удалённый файл тоже: add -A берёт и удаление.
func (r Repo) Stage(paths []string) error {
	return failed("add", r.stage(paths))
}

// noPaths это отказ пустому списку путей до вызова git: `git add -A --` без
// путей ставит в индекс всё дерево.
func noPaths() error {
	return i18n.New("git.noPaths", nil)
}

func (r Repo) stage(paths []string) error {
	if len(paths) == 0 {
		return noPaths()
	}

	_, err := run(literal(r.root), append([]string{"add", "-A", "--"}, paths...)...)
	return err
}

// Unstage убирает пути из индекса. Правка в рабочем дереве остаётся на месте.
func (r Repo) Unstage(paths []string) error {
	return failed("restore", r.unstage(paths))
}

func (r Repo) unstage(paths []string) error {
	if len(paths) == 0 {
		return noPaths()
	}

	command := []string{"restore", "--staged", "--"}
	if !hasCommits(r.root) {
		// Без HEAD восстанавливать не из чего: запись просто выходит из индекса.
		// -f снимает только сверку индекса с файлом: без него файл, изменённый
		// после add, git не отпускал (#384), а рабочее дерево --cached не трогает.
		command = []string{"rm", "--cached", "-f", "--quiet", "--"}
	}

	_, err := run(literal(r.root), append(command, paths...)...)
	return err
}

// submoduleMode это режим записи подмодуля (gitlink) в дереве.
const submoduleMode = "160000"

// Discard отменяет правки путей. Файл, который есть в HEAD, возвращается к нему
// вместе с индексом. Файл, которого в HEAD нет, убирается из индекса и с диска.
// Подмодуль не отменяется: restore сдвинул бы его checkout и потерял коммиты, а
// у нового (его нет в HEAD) удаление оставило бы индекс изменённым при целом
// каталоге на диске.
// Ошибка git на файле из HEAD не трогает диск: решается по ls-tree, а не по
// сбою restore, чтобы сбой не стёр правку.
func (r Repo) Discard(paths []string) error {
	return failed("restore", r.discard(paths))
}

func (r Repo) discard(paths []string) error {
	if len(paths) == 0 {
		return noPaths()
	}

	root := r.root
	base, err := baseRevision(root)
	if err != nil {
		return err
	}

	// Отказ на подмодуле идёт до первой правки: иначе часть путей была бы уже
	// отменена.
	tracked := make([]bool, len(paths))
	for i, path := range paths {
		mode, err := treeMode(root, base, path)
		if err != nil {
			return err
		}
		tracked[i] = mode != ""

		// Нового подмодуля нет в HEAD, он только в индексе: без этой проверки он
		// шёл бы в ветку «нового файла», где rm --cached уже сменил индекс, а
		// удалить непустой каталог не выходит.
		if !tracked[i] {
			if mode, err = indexMode(root, path); err != nil {
				return err
			}
		}
		if mode == submoduleMode {
			return i18n.New("git.submoduleDiscard", i18n.Args{"path": path})
		}
	}

	// Сначала уходят новые пути, потом возвращаются пути из HEAD. При
	// переименовании только регистра (Foo.txt в foo.txt) на нечувствительной к
	// регистру ФС это один файл: restore, а за ним удаление «нового» стирали
	// его с диска (#384).
	for i, path := range paths {
		if !tracked[i] {
			if err := r.dropNew(path); err != nil {
				return err
			}
		}
	}
	for i, path := range paths {
		if tracked[i] {
			if _, err := run(literal(root), "restore", "--source=HEAD", "--staged", "--worktree", "--", path); err != nil {
				return err
			}
		}
	}

	return nil
}

// dropNew убирает путь, которого нет в HEAD, из индекса и с диска.
func (r Repo) dropNew(path string) error {
	if _, err := run(literal(r.root), "rm", "-f", "--cached", "--ignore-unmatch", "--quiet", "--", path); err != nil {
		return err
	}
	name, err := r.file(path)
	if err != nil {
		return err
	}
	if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// treeMode возвращает режим записи пути в дереве base, пустую строку, если пути
// там нет. cat-file для этого не годится: у gitlink подмодуля в этом
// репозитории нет объекта.
func treeMode(root, base, path string) (string, error) {
	out, err := run(literal(root), "ls-tree", base, "--", path)
	mode, _, _ := strings.Cut(out, " ")
	return mode, err
}

// indexMode возвращает режим записи пути в индексе, пустую строку, если пути
// там нет.
func indexMode(root, path string) (string, error) {
	out, err := run(literal(root), "ls-files", "--stage", "--", path)
	mode, _, _ := strings.Cut(out, " ")
	return mode, err
}

// StagedPaths возвращает пути, у которых индекс отличается от HEAD. Неслитые
// пути (конфликт слияния) не в счёте: их ещё нужно пометить разрешёнными, то
// есть поставить, а не убрать из индекса.
func (r Repo) StagedPaths() ([]string, error) {
	out, err := r.inRoot("diff", "--cached", "--name-only", "-z", "--diff-filter=u")
	if err != nil {
		return nil, failed("diff", err)
	}

	var paths []string
	for _, path := range strings.Split(out, "\x00") {
		if path != "" {
			paths = append(paths, path)
		}
	}

	return paths, nil
}

package git

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"strings"

	"gitone/internal/i18n"
)

// Ignore дописывает неотслеженный файл в корневой .gitignore (#214): точным
// путём от корня, а не шаблоном. Файла .gitignore нет, и он создаётся.
// Отслеженный файл git не игнорирует, Ignore тогда отказывает. Путь, который
// уже игнорируется, .gitignore не меняет.
func (r Repo) Ignore(path string) error {
	return failed("ignore", r.ignore(path))
}

func (r Repo) ignore(path string) (err error) {
	// строка .gitignore не назовёт такой путь: перевод строки разбил бы её на
	// два шаблона, а \r в конце git срезает (#389)
	if strings.Contains(path, "\n") || strings.HasSuffix(path, "\r") {
		return i18n.New("git.ignoreLineBreak", nil)
	}
	if _, err := run(literal(r.root), "ls-files", "--error-unmatch", "--", path); err == nil {
		return i18n.New("git.ignoreTracked", i18n.Args{"path": path})
	}
	// check-ignore не знает GIT_LITERAL_PATHSPECS, и «:» в начале пути он
	// читал бы магией pathspec: «./» её снимает
	if _, err := r.inRoot("check-ignore", "--quiet", "--no-index", "--", "./"+path); err == nil {
		return nil
	}

	name, err := r.file(".gitignore")
	if err != nil {
		return err
	}
	content, err := os.ReadFile(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	line := ignorePattern(path) + "\n"
	if len(content) > 0 && !bytes.HasSuffix(content, []byte("\n")) {
		line = "\n" + line
	}

	file, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()

	_, err = file.WriteString(line)
	return err
}

// ignorePattern это строка .gitignore, которая совпадает ровно с path: косая
// черта в начале привязывает её к корню, знаки шаблона экранируются, а
// последний пробел экранируется, иначе git отрезал бы пробелы в конце.
func ignorePattern(path string) string {
	var pattern strings.Builder
	pattern.WriteString("/")
	for _, char := range path {
		if strings.ContainsRune(`\*?[`, char) {
			pattern.WriteRune('\\')
		}
		pattern.WriteRune(char)
	}

	if line, found := strings.CutSuffix(pattern.String(), " "); found {
		return line + `\ `
	}
	return pattern.String()
}

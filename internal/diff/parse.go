package diff

import (
	"regexp"
	"strconv"
	"strings"
)

// Пропущенный счётчик строк значит одну строку: git пишет `@@ -1 +1 @@`.
var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

type parser struct {
	files []File
	// Строки хунка читаются по счётчикам из `@@`, а не по префиксу: удалённая
	// строка `-- comment` в диффе выглядит как заголовок `--- comment`.
	oldLeft int
	newLeft int
}

func Parse(text string) []File {
	var p parser
	if text == "" {
		return nil
	}

	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		switch {
		// `\ No newline at end of file` относится к предыдущей строке и в
		// счётчики хунка не входит. Приходит и внутри хунка, и после него.
		case strings.HasPrefix(line, `\`) && !p.inRaw():
			p.markNoNewline()
		case p.oldLeft > 0 || p.newLeft > 0:
			p.hunkLine(line)
		default:
			p.headerLine(line)
		}
	}

	return p.files
}

func (p *parser) headerLine(line string) {
	switch {
	case strings.HasPrefix(line, "diff --git "):
		oldPath, newPath := headerPaths(strings.TrimPrefix(line, "diff --git "))
		p.files = append(p.files, File{OldPath: oldPath, NewPath: newPath, Status: StatusModified})
	case strings.HasPrefix(line, "diff --cc "), strings.HasPrefix(line, "diff --combined "):
		_, combinedPath, _ := strings.Cut(strings.TrimPrefix(line, "diff --"), " ")
		p.files = append(p.files, File{NewPath: path(combinedPath, ""), Status: StatusRaw, Raw: clean(line)})
	case strings.HasPrefix(line, "* Unmerged path "):
		unmergedPath := strings.TrimPrefix(line, "* Unmerged path ")
		p.files = append(p.files, File{NewPath: path(unmergedPath, ""), Status: StatusRaw, Raw: clean(line)})
	case p.inRaw() || !p.fileHeader(line):
		p.appendRaw(line)
	}
}

// blobs это хэши версий из строки «index <старый>..<новый> <режим>». Хэш из
// нулей значит, что версии нет. Запятая в хэшах («a,b..c») бывает только у
// `diff --cc`, а он уходит в сырую запись раньше разбора заголовков.
func blobs(index string) (old, new string) {
	ids, _, _ := strings.Cut(index, " ")
	old, new, found := strings.Cut(ids, "..")
	if !found {
		return "", ""
	}

	present := func(hash string) string {
		if strings.Trim(hash, "0") == "" {
			return ""
		}
		return hash
	}
	return present(old), present(new)
}

// fileHeader разбирает строку заголовка обычного файла. Незнакомая строка
// возвращает false и уходит в сырую запись, чтобы не пропасть молча.
func (p *parser) fileHeader(line string) bool {
	file := p.current()
	if file == nil {
		return false
	}

	switch {
	case strings.HasPrefix(line, "index "):
		file.OldBlob, file.NewBlob = blobs(strings.TrimPrefix(line, "index "))
	case strings.HasPrefix(line, "similarity index "),
		strings.HasPrefix(line, "dissimilarity index "):
	case strings.HasPrefix(line, "new file mode "):
		file.Status = StatusAdded
		file.OldPath = ""
		file.NewMode = strings.TrimPrefix(line, "new file mode ")
	case strings.HasPrefix(line, "deleted file mode "):
		file.Status = StatusDeleted
		file.NewPath = ""
		file.OldMode = strings.TrimPrefix(line, "deleted file mode ")
	case strings.HasPrefix(line, "old mode "):
		file.OldMode = strings.TrimPrefix(line, "old mode ")
	case strings.HasPrefix(line, "new mode "):
		file.NewMode = strings.TrimPrefix(line, "new mode ")
	case strings.HasPrefix(line, "Binary files "):
		file.Binary = true
	case strings.HasPrefix(line, "rename from "):
		file.Status = StatusRenamed
		file.OldPath = path(strings.TrimPrefix(line, "rename from "), "")
	case strings.HasPrefix(line, "rename to "):
		file.NewPath = path(strings.TrimPrefix(line, "rename to "), "")
	case strings.HasPrefix(line, "copy from "):
		file.Status = StatusCopied
		file.OldPath = path(strings.TrimPrefix(line, "copy from "), "")
	case strings.HasPrefix(line, "copy to "):
		file.NewPath = path(strings.TrimPrefix(line, "copy to "), "")
	case strings.HasPrefix(line, "--- "):
		file.OldPath = path(strings.TrimPrefix(line, "--- "), "a/")
	case strings.HasPrefix(line, "+++ "):
		file.NewPath = path(strings.TrimPrefix(line, "+++ "), "b/")
	case strings.HasPrefix(line, "@@ "):
		match := hunkHeader.FindStringSubmatch(line)
		if match == nil {
			return false
		}

		file.Hunks = append(file.Hunks, Hunk{OldStart: number(match[1], 0), NewStart: number(match[3], 0)})
		p.oldLeft = number(match[2], 1)
		p.newLeft = number(match[4], 1)
	default:
		return false
	}

	return true
}

func (p *parser) hunkLine(line string) {
	file := p.current()
	hunk := &file.Hunks[len(file.Hunks)-1]

	switch {
	case strings.HasPrefix(line, "+"):
		hunk.Lines = append(hunk.Lines, Line{Kind: LineAdded, Text: clean(line[1:])})
		file.Added++
		p.newLeft--
	case strings.HasPrefix(line, "-"):
		hunk.Lines = append(hunk.Lines, Line{Kind: LineRemoved, Text: clean(line[1:])})
		file.Removed++
		p.oldLeft--
	default:
		hunk.Lines = append(hunk.Lines, Line{Kind: LineContext, Text: clean(strings.TrimPrefix(line, " "))})
		p.oldLeft--
		p.newLeft--
	}
}

func (p *parser) markNoNewline() {
	file := p.current()
	if file == nil || len(file.Hunks) == 0 {
		return
	}

	hunk := &file.Hunks[len(file.Hunks)-1]
	if len(hunk.Lines) > 0 {
		hunk.Lines[len(hunk.Lines)-1].NoNewline = true
	}
}

// appendRaw чистит строку так же, как строки хунков: сырой текст тоже идёт
// на экран, и escape-код из конфликта слияния иначе красил бы терминал.
func (p *parser) appendRaw(line string) {
	if p.inRaw() {
		file := p.current()
		file.Raw += "\n" + clean(line)
		return
	}

	p.files = append(p.files, File{Status: StatusRaw, Raw: clean(line)})
}

func (p *parser) inRaw() bool {
	file := p.current()
	return file != nil && file.Status == StatusRaw
}

func (p *parser) current() *File {
	if len(p.files) == 0 {
		return nil
	}

	return &p.files[len(p.files)-1]
}

func number(digits string, fallback int) int {
	if digits == "" {
		return fallback
	}

	value, _ := strconv.Atoi(digits)
	return value
}

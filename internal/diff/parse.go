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
	var state parser
	if text == "" {
		return nil
	}

	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		switch {
		// `\ No newline at end of file` относится к предыдущей строке и в
		// счётчики хунка не входит. Приходит и внутри хунка, и после него.
		case strings.HasPrefix(line, `\`) && !state.inRaw():
			state.markNoNewline()
		case state.oldLeft > 0 || state.newLeft > 0:
			state.hunkLine(line)
		default:
			state.headerLine(line)
		}
	}

	return state.files
}

func (state *parser) headerLine(line string) {
	switch {
	case strings.HasPrefix(line, "diff --git "):
		oldPath, newPath := headerPaths(strings.TrimPrefix(line, "diff --git "))
		state.files = append(state.files, File{OldPath: oldPath, NewPath: newPath, Status: StatusModified})
	case strings.HasPrefix(line, "diff --cc "), strings.HasPrefix(line, "diff --combined "):
		_, combinedPath, _ := strings.Cut(strings.TrimPrefix(line, "diff --"), " ")
		state.files = append(state.files, File{NewPath: path(combinedPath, ""), Status: StatusRaw, Raw: clean(line)})
	case strings.HasPrefix(line, "* Unmerged path "):
		unmergedPath := strings.TrimPrefix(line, "* Unmerged path ")
		state.files = append(state.files, File{NewPath: path(unmergedPath, ""), Status: StatusRaw, Raw: clean(line)})
	case state.inRaw() || !state.fileHeader(line):
		state.appendRaw(line)
	}
}

// fileHeader разбирает строку заголовка обычного файла. Незнакомая строка
// возвращает false и уходит в сырую запись, чтобы не пропасть молча.
func (state *parser) fileHeader(line string) bool {
	file := state.current()
	if file == nil {
		return false
	}

	switch {
	case strings.HasPrefix(line, "index "),
		strings.HasPrefix(line, "similarity index "),
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
		state.oldLeft = number(match[2], 1)
		state.newLeft = number(match[4], 1)
	default:
		return false
	}

	return true
}

func (state *parser) hunkLine(line string) {
	file := state.current()
	hunk := &file.Hunks[len(file.Hunks)-1]

	switch {
	case strings.HasPrefix(line, "+"):
		hunk.Lines = append(hunk.Lines, Line{Kind: LineAdded, Text: clean(line[1:])})
		file.Added++
		state.newLeft--
	case strings.HasPrefix(line, "-"):
		hunk.Lines = append(hunk.Lines, Line{Kind: LineRemoved, Text: clean(line[1:])})
		file.Removed++
		state.oldLeft--
	default:
		hunk.Lines = append(hunk.Lines, Line{Kind: LineContext, Text: clean(strings.TrimPrefix(line, " "))})
		state.oldLeft--
		state.newLeft--
	}
}

func (state *parser) markNoNewline() {
	file := state.current()
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
func (state *parser) appendRaw(line string) {
	if state.inRaw() {
		file := state.current()
		file.Raw += "\n" + clean(line)
		return
	}

	state.files = append(state.files, File{Status: StatusRaw, Raw: clean(line)})
}

func (state *parser) inRaw() bool {
	file := state.current()
	return file != nil && file.Status == StatusRaw
}

func (state *parser) current() *File {
	if len(state.files) == 0 {
		return nil
	}

	return &state.files[len(state.files)-1]
}

func number(digits string, fallback int) int {
	if digits == "" {
		return fallback
	}

	value, _ := strconv.Atoi(digits)
	return value
}

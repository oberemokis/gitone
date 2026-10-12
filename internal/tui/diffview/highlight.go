package diffview

import (
	"gitone/internal/diff"
	"gitone/internal/syntax"
)

// code это отрезки подсветки строк одной стороны файла по номеру строки.
type code map[int][]syntax.Span

// highlight размечает обе стороны каждого хунка: старую из контекста и
// удалённых строк, новую из контекста и добавленных. Сторона разбирается
// целиком, а не по строке: tree-sitter нужен контекст. Считается один раз при
// открытии файла, а не на каждый кадр и не при смене размера: разбор дороже
// отрисовки.
func highlight(lexer syntax.Lexer, file diff.File) (old, new code) {
	old, new = code{}, code{}
	for _, hunk := range file.Hunks {
		var oldLines, newLines []string
		for _, line := range hunk.Lines {
			if line.Kind != diff.LineAdded {
				oldLines = append(oldLines, line.Text)
			}
			if line.Kind != diff.LineRemoved {
				newLines = append(newLines, line.Text)
			}
		}

		for index, spans := range lexer.Lines(oldLines) {
			old[hunk.OldStart+index] = spans
		}
		for index, spans := range lexer.Lines(newLines) {
			new[hunk.NewStart+index] = spans
		}
	}

	return old, new
}

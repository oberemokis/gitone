package diffview

import (
	"cmp"
	"strings"

	"gitone/internal/diff"
	"gitone/internal/syntax"
)

// Подсветка по версиям файла целиком (#140). Однофайловому компоненту без
// открывающего тега в хунке не понять, на каком языке код, поэтому корень
// читает обе версии, а дифф красит их целиком и берёт цвета строк по номерам.
// Пока версий нет, подсветка по хункам.

// sides это подсветка обеих версий по номеру строки; nil у стороны, которой
// нет или которую не прочитали: она остаётся подсвеченной по хункам.
type sides struct {
	old, new code
}

// maxVersions ограничивает кеш: каждый сохранённый файл даёт новые хэши.
// ponytail: кеш сбрасывается целиком, LRU, если сбросы станут заметны.
const maxVersions = 32

// NeedsVersions говорит, что файл красится по версиям целиком: это
// однофайловый компонент, и в диффе есть хэши его версий.
func NeedsVersions(file diff.File) bool {
	return (file.OldBlob != "" || file.NewBlob != "") && syntax.Detect(cmp.Or(file.NewPath, file.OldPath)).Embeds()
}

// VersionKey это ключ версий, по которым раскрашен файл, или пустая строка,
// если файл раскрашен по хункам.
func (m Model) VersionKey() string {
	return m.versionKey
}

// StoreVersions раскрашивает версии файла path целиком и кладёт подсветку в
// кеш по ключу. nil у версии, которой нет или которую не прочитали. Открытый
// файл не меняется, пока не позовут UseVersions: пока версии читались, могли
// открыть другой файл.
func (m Model) StoreVersions(key, path string, old, new []byte) Model {
	if m.versions == nil || len(m.versions) >= maxVersions {
		m.versions = map[string]sides{}
	}

	lexer := syntax.Detect(path)
	m.versions[key] = sides{old: wholeFile(lexer, old), new: wholeFile(lexer, new)}
	return m
}

// UseVersions раскрашивает открытый файл подсветкой версий из кеша. found
// false, если версий по ключу ещё нет.
func (m Model) UseVersions(key string) (Model, bool) {
	cached, found := m.versions[key]
	if !found {
		return m, false
	}

	if cached.old != nil {
		m.oldCode = cached.old
	}
	if cached.new != nil {
		m.newCode = cached.new
	}
	m.versionKey = key
	return m, true
}

// wholeFile размечает версию файла целиком, строки нумеруются с единицы, как
// в хунках. Без версии nil. Строки очищаются так же, как строки диффа (конец
// CRLF срезан, управляющие символы значками): отрезки подсветки считаются в
// байтах и должны ложиться на текст, который стоит на экране, а значок в
// три байта сдвинул бы их.
func wholeFile(lexer syntax.Lexer, content []byte) code {
	if content == nil {
		return nil
	}

	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	for index, line := range lines {
		lines[index] = diff.Printable(strings.TrimSuffix(line, "\r"))
	}

	result := code{}
	for index, spans := range lexer.Lines(lines) {
		result[index+1] = spans
	}
	return result
}

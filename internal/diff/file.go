package diff

type Status string

const (
	StatusModified Status = "modified"
	StatusAdded    Status = "added"
	StatusDeleted  Status = "deleted"
	StatusRenamed  Status = "renamed"
	StatusCopied   Status = "copied"
	// Блок, который разбор не узнал. Его текст лежит в File.Raw и
	// показывается как есть: лучше сырой текст, чем молча пропавший файл.
	StatusRaw Status = "raw"
)

type LineKind string

const (
	LineContext LineKind = "context"
	LineAdded   LineKind = "added"
	LineRemoved LineKind = "removed"
)

type Line struct {
	Kind LineKind
	Text string
	// В файле за этой строкой нет перевода строки. Без отметки правка,
	// которая только добавила перевод в конец, выглядит как две
	// одинаковые строки.
	NoNewline bool
	// Строка файла кончалась `\r\n`: Text без `\r`, он остаётся признаком.
	// Правка, которая только меняет конец строки CRLF на LF, иначе давала бы
	// пары одинаковых строк без отличий.
	CR bool
}

type Hunk struct {
	OldStart int
	NewStart int
	Lines    []Line
}

type File struct {
	// Пустой путь значит, что файла с этой стороны нет: новый файл без
	// старого пути, удалённый без нового.
	OldPath string
	NewPath string
	Status  Status
	Binary  bool
	OldMode string
	NewMode string
	// Хэши версий из строки index, пусто у версии, которой нет. По ним git
	// отдаёт файл целиком для подсветки в контексте (#140). Объекта с хэшем
	// новой версии правки вне индекса в git нет: git его только вычисляет.
	OldBlob string
	NewBlob string
	Hunks   []Hunk
	Added   int
	Removed int
	Raw     string
}

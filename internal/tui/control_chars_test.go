package tui_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/layout"
)

// Имя файла из чужого репозитория не должно управлять терминалом: OSC меняет
// заголовок окна, CSI 2J стирает экран. Эти же последовательности без очистки
// ansi.Strip вырезал бы из экрана, поэтому тест ищет значки, а не отсутствие
// символов.
const (
	hostileName = "evil\x1b]0;PWNED\x07\x1b[2J.txt"
	shownName   = "evil␛]0;PWNED␇␛[2J.txt"
)

func hostileModel(t *testing.T, root layout.Node) tea.Model {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, hostileName, "old\n")
	commitAll(t, repo)
	write(t, repo, hostileName, "new\n")

	return resize(open(t, options(repo, root)), 100, 20)
}

// styling это SGR-последовательности, которыми интерфейс красит себя.
var styling = regexp.MustCompile("\x1b\\[[0-9;:]*m")

// noEscapes сообщает, что в сыром выводе нет ни одного управляющего символа,
// кроме перевода строки и собственной раскраски интерфейса: ESC, BEL, `\r` и
// C1 из чужих данных терминалу не достаются.
func noEscapes(t *testing.T, model tea.Model) {
	t.Helper()

	raw := styling.ReplaceAllString(model.View().Content, "")
	if at := strings.IndexFunc(raw, func(char rune) bool { return char != '\n' && unicode.IsControl(char) }); at >= 0 {
		t.Errorf("в выводе управляющий символ %q:\n%q", []rune(raw[at:])[0], raw)
	}
}

func TestFileNameWithEscapesIsPrintableInHeader(t *testing.T) {
	model := hostileModel(t, layout.Focus())

	if header := screenOf(model)[headerRow]; !strings.Contains(header, shownName) {
		t.Errorf("в заголовке нет %q: %q", shownName, header)
	}
	noEscapes(t, model)
}

func TestFileNameWithEscapesIsPrintableInFilesPane(t *testing.T) {
	model := toggleFiles(hostileModel(t, layout.Default()))

	// панель в 20 колонок обрезает имя, значки начала имени видны
	if body := screenText(model); !strings.Contains(body, "evil␛]0;PWNED") {
		t.Errorf("в панели файлов нет значков:\n%s", body)
	}
	noEscapes(t, model)
}

func TestFileNameWithEscapesIsPrintableInSearch(t *testing.T) {
	model := hostileModel(t, layout.Focus())

	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})

	if body := screenText(model); strings.Count(body, shownName) != 2 {
		t.Errorf("имя должно стоять в заголовке и в окне поиска:\n%s", body)
	}
	noEscapes(t, model)
}

// Остальные поля из git (тема и автор коммита, ветка, stash, текст ошибки,
// путь worktree) идут на экран теми же значками, что имя файла: окно или
// подвал выводят их через ansi.Truncate, который сохраняет последовательности.
const (
	hostileText = "fix\x1b]0;owned\x07\x1b[2J"
	shownText   = "fix␛]0;owned␇␛[2J"
)

func TestCommitSubjectWithEscapesIsPrintableInLog(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", hostileText)

	model := openAsync(resize(open(t, options(repo, layout.Focus())), 140, 40), key('l'))

	if body := screenText(model); !strings.Contains(body, shownText) {
		t.Errorf("в логе нет %q:\n%s", shownText, body)
	}
	noEscapes(t, model)
}

// `\r` в имени автора вернул бы курсор в начало строки и затёр коммит.
func TestAuthorWithCarriageReturnKeepsLogTable(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	t.Setenv("GIT_AUTHOR_NAME", "Eve\rEvil")
	commitAll(t, repo)

	model := openAsync(resize(open(t, options(repo, layout.Focus())), 140, 40), key('l'))
	screen := screenOf(model)
	header, row := rowWith(t, screen, "Message"), rowWith(t, screen, "Eve␍Evil")

	if got, want := column(row, "Eve␍Evil"), column(header, "Author"); got != want {
		t.Errorf("автор с колонки %d, заголовок с %d:\n%s", got, want, strings.Join(screen, "\n"))
	}
	if !relativeDate.MatchString(row) || !strings.Contains(row, "commit") {
		t.Errorf("в строке нет темы и даты: %q", row)
	}
	noEscapes(t, model)
}

// git отвергает имя ветки с ESC и любым символом ниже пробела, но принимает
// C1: U+009B это CSI терминала с 8-битными кодами. На экране он становится
// U+FFFD, как в содержимом файла.
func TestBranchNameWithControlIsPrintableInWindowAndFooter(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	runGit(t, repo, "switch", "-q", "-c", "evil\u009b2Jbranch")

	model := openAsync(resize(open(t, options(repo, layout.Focus())), 140, 40), key('B'))

	// ветка текущая: она стоит в подвале и в строке окна
	if body := screenText(model); strings.Count(body, "evil\ufffd2Jbranch") != 2 {
		t.Errorf("имя должно стоять в подвале и в окне веток:\n%s", body)
	}
	noEscapes(t, model)
}

// sideWorktree добавляет к репозиторию worktree на ветке branch в каталоге
// dir и открывает gitone в основном дереве. Каталог лежит под /tmp: окно
// обрезает путь по ширине, и из длинного пути временного каталога macOS
// каталог dir не виден.
func sideWorktree(t *testing.T, dir, branch string) tea.Model {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)

	parent, err := os.MkdirTemp("/tmp", "g")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(parent) })
	runGit(t, repo, "worktree", "add", "-q", "-b", branch, filepath.Join(parent, dir))

	return resize(open(t, options(repo, layout.Focus())), 140, 40)
}

// Путь worktree может нести ESC (каталог так назвать можно), ветка C1. Окно
// показывает значки, а после переключения их же показывает подвал.
func TestWorktreeWithControlsIsPrintableInWindowAndFooter(t *testing.T) {
	model := openAsync(sideWorktree(t, "w\x1b[2J", "side\u009bq"), key('W'))

	if body := screenText(model); !strings.Contains(body, "w␛[2J  side�q") || !strings.Contains(body, "/w␛[2J") {
		t.Errorf("в окне worktree нет каталога, ветки и пути значками:\n%s", body)
	}
	noEscapes(t, model)

	model, _ = model.Update(key('j'))
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 3)

	if footer := footerOf(model); !strings.HasPrefix(footer, " B side�q   W w␛[2J") {
		t.Errorf("подвал = %q, want ветку и каталог значками", footer)
	}
	noEscapes(t, model)
}

// Таб в имени каталога worktree занимает четыре колонки, как в ячейках лога.
func TestWorktreePathWithTabIsExpanded(t *testing.T) {
	model := openAsync(sideWorktree(t, "a\tb", "side"), key('W'))

	if body := screenText(model); !strings.Contains(body, "a    b  side") {
		t.Errorf("в окне worktree таб не раскрыт в четыре пробела:\n%s", body)
	}
	noEscapes(t, model)
}

// Текст отказа хука git пересказывает как есть: хук печатает в stderr что
// угодно, и до подвала это доходит без Truncate над ячейкой окна.
func TestGitErrorWithEscapesIsPrintableInFooter(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "add", "a.txt")
	hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
	write(t, repo, ".git/hooks/pre-commit", "#!/bin/sh\nprintf 'hook\\033]0;PWNED\\007\\033[2J failed\\n' >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}

	model := resize(open(t, options(repo, layout.Default())), 100, 20)
	model, _ = model.Update(key('c'))
	model = typeText(model, "x")
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = settle(model, cmd, 4)

	if footer := footerOf(model); !strings.Contains(footer, "commit: hook␛]0;PWNED␇␛[2J failed") {
		t.Errorf("подвал = %q, want отказ хука значками", footer)
	}
	noEscapes(t, model)
}

// stashRepo это репозиторий с одним stash, у которого сообщение message.
func stashRepo(t *testing.T, message string) string {
	t.Helper()

	repo := newRepo(t)
	write(t, repo, "a.txt", "one\n")
	commitAll(t, repo)
	write(t, repo, "a.txt", "two\n")
	runGit(t, repo, "stash", "push", "-q", "-m", message)
	return repo
}

func TestStashMessageWithEscapesIsPrintableInWindow(t *testing.T) {
	repo := stashRepo(t, hostileText)

	model := openAsync(resize(open(t, options(repo, layout.Focus())), 100, 20), key('Z'))

	if body := screenText(model); !strings.Contains(body, "On main: "+shownText) {
		t.Errorf("в окне stash нет %q:\n%s", shownText, body)
	}
	noEscapes(t, model)
}

// git сворачивает таб в сообщении stash при записи, но читает его из reflog
// как есть, поэтому таб попадает в строку правкой файла reflog. Окно в 64
// колонок: сообщение с табом занимает 62 колонки до раскрытия и 66 после.
func TestStashMessageWithTabIsTruncatedAfterExpansion(t *testing.T) {
	repo := stashRepo(t, "head"+strings.Repeat("x", 36))
	reflog := filepath.Join(repo, ".git", "logs", "refs", "stash")
	withTab := strings.Replace(readText(t, reflog), "On main: head", "On main: head\t", 1)
	if err := os.WriteFile(reflog, []byte(withTab), 0o644); err != nil {
		t.Fatal(err)
	}

	model := openAsync(resize(open(t, options(repo, layout.Focus())), 100, 20), key('Z'))

	want := "head    " + strings.Repeat("x", 33) + "…"
	if body := screenText(model); !strings.Contains(body, want) {
		t.Errorf("в окне stash нет строки %q, обрезанной после раскрытия таба:\n%s", want, body)
	}
}

package tui_test

import (
	"strings"
	"testing"

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

// noEscapes сообщает, что в выводе нет последовательностей из имени файла.
// Собственный вывод интерфейса может содержать CSI для цвета, поэтому ищется
// именно OSC и BEL.
func noEscapes(t *testing.T, model tea.Model) {
	t.Helper()

	raw := model.View().Content
	for _, sequence := range []string{"\x07", "\x1b]", "\x1b[2J"} {
		if strings.Contains(raw, sequence) {
			t.Errorf("в выводе есть %q:\n%q", sequence, raw)
		}
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

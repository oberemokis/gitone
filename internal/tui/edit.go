package tui

import (
	"cmp"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/i18n"
)

// editorMsg несёт команду редактора для файла или причину, по которой его не
// запустить.
type editorMsg struct {
	command *exec.Cmd
	err     error
}

func (msg editorMsg) apply(m Model) (tea.Model, tea.Cmd) { return m.startEditor(msg) }

// editFile открывает текущий файл в редакторе (#215), как e в lazygit: на
// любом диффе, ведь правится файл рабочего дерева. Запись без пути редактору
// отдать нечего.
func (m Model) editFile() (tea.Model, tea.Cmd) {
	if len(m.files) == 0 {
		return m, nil
	}

	path := stagedKey(m.files[m.current])
	if path == "" {
		return m.putNotice(m.text.Text("stage.noFile", nil)), nil
	}
	return m, editorCmd(m.repo.Root(), path)
}

// editorCmd находит редактор и файл в команде, а не в Update: и окружение, и
// файловая система это ввод-вывод. Редактор берётся из VISUAL, потом из
// EDITOR, как у git. Файл, удалённый после выбора, редактор создал бы заново,
// поэтому его отсутствие это ошибка. path идёт от корня репозитория root.
func editorCmd(root, path string) tea.Cmd {
	return func() tea.Msg {
		editor := strings.TrimSpace(cmp.Or(os.Getenv("VISUAL"), os.Getenv("EDITOR")))
		if editor == "" {
			return editorMsg{err: i18n.New("editor.notSet", nil)}
		}

		// путь от корня, а не полный: полный из os.Stat растянул бы тост на
		// несколько строк
		file := filepath.Join(root, path)
		if _, err := os.Stat(file); errors.Is(err, fs.ErrNotExist) {
			return editorMsg{err: i18n.New("editor.missing", i18n.Args{"path": path})}
		} else if err != nil {
			return editorMsg{err: err}
		}

		return editorMsg{command: editorCommand(editor, file)}
	}
}

// editorCommand запускает редактор через sh, как git: в VISUAL бывают
// аргументы и путь с пробелом в кавычках («code --wait»). На Windows sh нет,
// там редактор делится по пробелам.
func editorCommand(editor, file string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		fields := strings.Fields(editor)
		return exec.Command(fields[0], append(fields[1:], file)...)
	}
	return exec.Command("sh", "-c", editor+` "$@"`, editor, file)
}

// startEditor отдаёт терминал редактору, а после его выхода перечитывает дифф:
// правка видна сразу, а не к тику опроса. Код выхода редактора, кроме нуля,
// идёт в тост.
func (m Model) startEditor(msg editorMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m.say(m.text.Error(msg.err)), nil
	}

	return m, m.exec(msg.command, func(err error) tea.Msg {
		return actionDoneMsg{err: err}
	})
}

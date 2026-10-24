package tui

import (
	"bytes"
	"cmp"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// runCommand запускает свою команду пользователя (#246) из корня репозитория
// через shell, как customCommands в lazygit: `sh -c`, а в Windows `cmd /C`.
// {path} заменяется переменной окружения с путём открытого файла в кавычках
// shell: значение переменной shell повторно не разбирает, и пробелы,
// кавычки и `$` в пути команду не ломают. stdin пустой: команда, которая
// ждёт ввода, получает конец файла, а не висит. Подвал говорит первую строку
// вывода, а при ошибке код выхода и первую строку stderr. После команды
// дифф перечитывается: она могла поменять репозиторий.
func (m Model) runCommand(command keymap.Command) (tea.Model, tea.Cmd) {
	if next, refused := m.refuseWhileRemote(); refused {
		return next, nil
	}

	name := cmp.Or(command.Name, command.Run)
	script, path := command.Run, ""
	if strings.Contains(script, "{path}") {
		if len(m.files) == 0 || stagedKey(m.files[m.current]) == "" {
			return m.putNotice(m.text.Text("command.noFile", i18n.Args{"name": visible(name)})), nil
		}
		script = strings.ReplaceAll(script, "{path}", pathVariable)
		path = stagedKey(m.files[m.current])
	}

	root, done := m.repo.Root(), m.text.Text("command.done", i18n.Args{"name": visible(name)})
	return m, func() tea.Msg {
		output, err := runShell(root, script, path)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		if line := firstLine(output); line != "" {
			done = visible(name) + ": " + visible(line)
		}
		return actionDoneMsg{note: done}
	}
}

// runShell выполняет script в каталоге dir с путём path в GITONE_PATH и
// отдаёт stdout. Выход с кодом не ноль это ошибка из словаря с кодом и первой
// строкой stderr, а без неё stdout. Итог не ждёт фоновых процессов команды
// (`gitk &`): они держали бы stdout открытым, пока не закроются сами.
func runShell(dir, script, path string) (string, error) {
	shell := shellCommand(script)
	shell.Dir = dir
	shell.Env = append(os.Environ(), "GITONE_PATH="+path)
	shell.WaitDelay = time.Second

	var stdout, stderr bytes.Buffer
	shell.Stdout, shell.Stderr = &stdout, &stderr
	err := shell.Run()

	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		line := cmp.Or(firstLine(stderr.String()), firstLine(stdout.String()))
		return "", i18n.New("command.failed", i18n.Args{"code": exit.ExitCode(), "line": visible(line)})
	case errors.Is(err, exec.ErrWaitDelay):
		return stdout.String(), nil
	}
	return stdout.String(), err
}

// firstLine это первая непустая строка текста без пробелов по краям.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

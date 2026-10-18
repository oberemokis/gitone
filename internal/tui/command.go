package tui

import (
	"bytes"
	"cmp"
	"errors"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"gitone/internal/git"
	"gitone/internal/i18n"
	"gitone/internal/keymap"
)

// runCommand запускает свою команду пользователя (#246) из корня репозитория
// через shell, как customCommands в lazygit: `sh -c`, а в Windows `cmd /C`.
// {path} заменяется путём открытого файла в кавычках shell, пробелы и
// кавычки в пути команду не ломают. stdin пустой: команда, которая ждёт
// ввода, получает конец файла, а не висит. Подвал говорит первую строку
// вывода, а при ошибке код выхода и первую строку stderr. После команды
// дифф перечитывается: она могла поменять репозиторий.
func (m Model) runCommand(command keymap.Command) (tea.Model, tea.Cmd) {
	if next, refused := m.refuseWhileRemote(); refused {
		return next, nil
	}

	name := cmp.Or(command.Name, command.Run)
	script := command.Run
	if strings.Contains(script, "{path}") {
		if len(m.files) == 0 || stagedKey(m.files[m.current]) == "" {
			return m.putNotice(m.text.Text("command.noFile", i18n.Args{"name": visible(name)}), true), nil
		}
		script = strings.ReplaceAll(script, "{path}", shellQuote(stagedKey(m.files[m.current])))
	}

	dir, done := m.dir, m.text.Text("command.done", i18n.Args{"name": visible(name)})
	return m, func() tea.Msg {
		root, err := git.Root(dir)
		if err != nil {
			return actionDoneMsg{err: err}
		}

		output, err := runShell(root, script)
		if err != nil {
			return actionDoneMsg{err: err}
		}
		if line := firstLine(output); line != "" {
			done = visible(name) + ": " + visible(line)
		}
		return actionDoneMsg{note: done}
	}
}

// runShell выполняет script в каталоге dir и отдаёт stdout. Выход с кодом
// не ноль это ошибка из словаря с кодом и первой строкой stderr, а без неё
// stdout.
func runShell(dir, script string) (string, error) {
	shell := exec.Command("sh", "-c", script)
	if runtime.GOOS == "windows" {
		shell = exec.Command("cmd", "/C", script)
	}
	shell.Dir = dir

	var stdout, stderr bytes.Buffer
	shell.Stdout, shell.Stderr = &stdout, &stderr
	err := shell.Run()

	var exit *exec.ExitError
	if errors.As(err, &exit) {
		line := cmp.Or(firstLine(stderr.String()), firstLine(stdout.String()))
		return "", i18n.New("command.failed", i18n.Args{"code": exit.ExitCode(), "line": visible(line)})
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

// shellQuote берёт путь в кавычки shell: в sh одинарные, где одинарная
// кавычка пути закрывается и экранируется, в cmd двойные.
func shellQuote(path string) string {
	if runtime.GOOS == "windows" {
		return `"` + path + `"`
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

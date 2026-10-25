//go:build unix

package tui

import (
	"os/exec"
	"syscall"
)

// pathVariable это подстановка {path} в sh.
const pathVariable = `"$GITONE_PATH"`

// shellCommand запускает script через sh в своей сессии (Setsid): у неё нет
// управляющего терминала, и ssh или другая программа, читающая /dev/tty,
// получает отказ, а не ждёт ответа на терминале, который читает интерфейс
// (#392).
func shellCommand(script string) *exec.Cmd {
	shell := exec.Command("sh", "-c", script)
	shell.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return shell
}

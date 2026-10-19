package tui

import (
	"os/exec"
	"syscall"
)

// pathVariable это подстановка {path} в cmd.
const pathVariable = `"%GITONE_PATH%"`

// shellCommand запускает script через cmd. Командная строка задана целиком:
// Go экранировал бы кавычки как \", а cmd таких экранов не понимает. /S
// снимает только внешние кавычки, остальное cmd разбирает как написано.
func shellCommand(script string) *exec.Cmd {
	shell := exec.Command("cmd")
	shell.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd /S /C "` + script + `"`}
	return shell
}

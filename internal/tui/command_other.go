//go:build !windows

package tui

import "os/exec"

// pathVariable это подстановка {path} в sh.
const pathVariable = `"$GITONE_PATH"`

// shellCommand запускает script через sh.
func shellCommand(script string) *exec.Cmd {
	return exec.Command("sh", "-c", script)
}

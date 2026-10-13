//go:build unix

package git

import (
	"os/exec"
	"syscall"
	"time"
)

// cancelGrace это время, которое git получает на выход после TERM: снять
// index.lock и закрыть соединение. Потом ожидание обрывается и git убивается.
const cancelGrace = 3 * time.Second

// stopGroup заставляет отмену убивать всю группу процессов git. `git pull` не
// пересылает сигнал своему `git fetch`, а `git push` ssh и хукам: убитый один
// родитель оставил бы их работать в фоне, и отменённый push мог бы дойти до
// origin. Группа своя (Setpgid), поэтому сигнал не заденет gitone. TERM, а не
// KILL: git при нём убирает свои lock-файлы.
func stopGroup(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		return syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
	}
	command.WaitDelay = cancelGrace
}

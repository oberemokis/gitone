//go:build !unix

package git

import (
	"os/exec"
	"time"
)

const cancelGrace = 3 * time.Second

// stopGroup на системах без групп процессов оставляет отмене то, что умеет
// os/exec: убить сам git. Его дети (fetch, ssh) могут дожить до конца.
func stopGroup(command *exec.Cmd) {
	command.WaitDelay = cancelGrace
}

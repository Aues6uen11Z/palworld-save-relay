package palworld

import (
	"os/exec"
	"syscall"
)

// hideCmdWindow sets SysProcAttr to prevent child processes from flashing a
// console window. Call on every exec.Command that runs a console program
// (tasklist, reg, cmd, etc.) before .Run()/.Output()/.Start().
func hideCmdWindow(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

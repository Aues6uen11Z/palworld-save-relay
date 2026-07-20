package updater

import (
	"os/exec"
)

func execCmd(cmd string) error {
	return exec.Command("cmd", "/c", cmd).Start()
}

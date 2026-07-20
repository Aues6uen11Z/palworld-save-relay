package updater

import (
	"os/exec"
	"path/filepath"
	"syscall"
)

const _CREATE_NO_WINDOW = 0x08000000
const _CREATE_NEW_PROCESS_GROUP = 0x00000200

func startDetachedBat(batPath string) error {
	c := exec.Command("cmd", "/c", batPath)
	c.Dir = filepath.Dir(batPath)
	c.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: _CREATE_NO_WINDOW | _CREATE_NEW_PROCESS_GROUP,
	}
	return c.Start()
}

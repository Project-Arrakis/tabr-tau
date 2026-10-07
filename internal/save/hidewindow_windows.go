package save

import (
	"os/exec"
	"syscall"
)

// hideWindow stops a child console program from opening its own window over the app.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

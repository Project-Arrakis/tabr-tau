//go:build !windows

package save

import (
	"errors"
	"syscall"
)

// processAlive says whether a process with this id is running. Only "no such process" counts as gone.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || !errors.Is(err, syscall.ESRCH)
}

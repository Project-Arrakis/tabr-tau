package save

import "syscall"

const (
	processQueryLimitedInformation = 0x1000
	stillActive                    = 259
)

// processAlive says whether a process with this id is running. A process that cannot be opened for a reason other than "no such process" counts as running: a copy is only removed
// when the process is known to be gone.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return err != syscall.Errno(87) // ERROR_INVALID_PARAMETER: there is no such process
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return true
	}
	return code == stillActive
}

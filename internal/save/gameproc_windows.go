package save

import (
	"fmt"
	"syscall"
	"unsafe"
)

// processImages lists the image name of every running process from the system's process snapshot (Toolhelp32). It reads the list in memory: no program is started, so
// there is no console, no window and no exit status to go wrong. Names are cut at the length the snapshot gives (260 characters), which is far past the prefix the
// caller matches on.
func processImages() ([]string, error) {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("process snapshot: %w", err)
	}
	defer syscall.CloseHandle(snap)
	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	if err := syscall.Process32First(snap, &e); err != nil {
		if err == syscall.ERROR_NO_MORE_FILES {
			return nil, nil
		}
		return nil, fmt.Errorf("process snapshot: %w", err)
	}
	var names []string
	for {
		names = append(names, syscall.UTF16ToString(e.ExeFile[:]))
		if err := syscall.Process32Next(snap, &e); err != nil {
			if err == syscall.ERROR_NO_MORE_FILES {
				return names, nil
			}
			return nil, fmt.Errorf("process snapshot: %w", err)
		}
	}
}

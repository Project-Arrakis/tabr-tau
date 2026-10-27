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

// saveFileHeld reports whether another program has the save file open in a way that stops it being replaced. It opens the file for reading with DELETE access and shares
// everything (read, write, delete) with others, then closes it at once. Windows refuses that open with a sharing violation when any other handle on the file was opened without
// FILE_SHARE_DELETE, which is exactly what makes replacing the file fail, so the answer needs no guess and, because this open allows every kind of sharing, it can never make
// the other program's own open or write fail. What it does not see: a handle opened with FILE_SHARE_DELETE, and a program that has read the file into memory and closed it.
// Any other failure (the file is gone, access is denied) is returned as an error: the caller treats a check that cannot be made as "held".
func saveFileHeld(path string) (bool, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	const deleteAccess = 0x00010000
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|deleteAccess,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		if errno, ok := err.(syscall.Errno); ok && (errno == errSharingViolation || errno == errLockViolation) {
			return true, nil
		}
		return false, err
	}
	syscall.CloseHandle(h)
	return false, nil
}

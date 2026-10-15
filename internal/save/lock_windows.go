package save

import (
	"errors"
	"syscall"
)

const (
	fileFlagDeleteOnClose = 0x04000000
	errSharingViolation   = syscall.Errno(32) // ERROR_SHARING_VIOLATION
	errLockViolation      = syscall.Errno(33) // ERROR_LOCK_VIOLATION
)

// lockFile opens the file with no sharing: a second open, by this or another process, fails with a sharing violation for as long as the
// first handle lives. FILE_FLAG_DELETE_ON_CLOSE removes the file when the handle closes, which Windows also does when the process dies.
func lockFile(path string) (func(), error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return func() {}, nil
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_ALWAYS,
		syscall.FILE_ATTRIBUTE_NORMAL|fileFlagDeleteOnClose, 0)
	if err != nil {
		if errors.Is(err, errSharingViolation) || errors.Is(err, errLockViolation) {
			return nil, ErrSaveInUse
		}
		return func() {}, nil
	}
	return func() { syscall.CloseHandle(h) }, nil
}

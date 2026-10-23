package save

import (
	"errors"
	"syscall"
	"time"
)

const (
	fileFlagDeleteOnClose = 0x04000000
	errSharingViolation   = syscall.Errno(32) // ERROR_SHARING_VIOLATION
	errLockViolation      = syscall.Errno(33) // ERROR_LOCK_VIOLATION
)

// lockFile opens the file with no sharing: a second open, by this or another process, fails with a sharing violation for as long as the
// first handle lives. FILE_FLAG_DELETE_ON_CLOSE removes the file when the handle closes, which Windows also does when the process dies.
func lockFile(path string) (release func(), held bool, err error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return func() {}, false, nil
	}
	var h syscall.Handle
	// A virus scanner or indexer can hold the file open for a moment, which looks the same as another editor holding it; a few short retries
	// tell the two apart (a real editor keeps it for as long as it runs).
	for try := 0; try < 6; try++ {
		h, err = syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_ALWAYS,
			syscall.FILE_ATTRIBUTE_NORMAL|fileFlagDeleteOnClose, 0)
		if err == nil || !(errors.Is(err, errSharingViolation) || errors.Is(err, errLockViolation)) {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	if err != nil {
		if errors.Is(err, errSharingViolation) || errors.Is(err, errLockViolation) {
			return nil, false, ErrSaveInUse
		}
		return func() {}, false, nil // not made for another reason: the editor goes on, holding no lock
	}
	return func() { syscall.CloseHandle(h) }, true, nil
}

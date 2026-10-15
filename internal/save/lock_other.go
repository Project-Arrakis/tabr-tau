//go:build !windows

package save

import (
	"errors"
	"os"
	"syscall"
)

// lockFile takes a non-blocking exclusive flock on the file. The kernel drops the lock when the process ends, so a file left behind by a
// crash locks nothing.
func lockFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return func() {}, nil
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrSaveInUse
		}
		return func() {}, nil
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close(); os.Remove(path) }, nil
}

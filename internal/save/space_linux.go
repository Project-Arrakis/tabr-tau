package save

import "syscall"

// freeBytesOf is the space available to an unprivileged program on the volume that holds dir.
func freeBytesOf(dir string) (uint64, bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, false
	}
	return uint64(st.Bavail) * uint64(st.Bsize), true
}

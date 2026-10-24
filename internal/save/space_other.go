//go:build !windows && !linux

package save

// freeBytesOf cannot tell here; the write is tried.
func freeBytesOf(string) (uint64, bool) { return 0, false }

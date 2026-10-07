//go:build windows

package gui

import (
	"testing"
	"unsafe"
)

// OPENFILENAMEW must match the C layout exactly or GetOpenFileNameW fails with CDERR_STRUCTSIZE: 152 bytes on 64-bit
// Windows, 88 on 32-bit (pointers are 8 or 4 bytes).
func TestOpenFileNameSizeMatchesWin32(t *testing.T) {
	want := uintptr(152)
	if unsafe.Sizeof(uintptr(0)) == 4 {
		want = 88
	}
	if got := unsafe.Sizeof(openFileName{}); got != want {
		t.Fatalf("sizeof(openFileName) = %d, want %d", got, want)
	}
}

func TestUTF16ZKeepsEmbeddedNULsAndEncodesSurrogates(t *testing.T) {
	got := utf16Z("a\x00b")
	if len(got) != 4 || got[1] != 0 || got[3] != 0 {
		t.Fatalf("utf16Z = %v", got)
	}
	if got := utf16Z("\U0001F600"); len(got) != 3 { // surrogate pair + terminator
		t.Fatalf("non-BMP rune must become two units, got %v", got)
	}
}

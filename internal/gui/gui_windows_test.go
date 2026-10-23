//go:build windows

package gui

import (
	"testing"
	"unsafe"

	"github.com/jchv/go-webview2/pkg/edge"
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

type focuser interface{ Focus() }

type notChromium struct{}

func (notChromium) Focus() {}

// The window's browser is found behind the wrapper's unexported interface field, by type; anything else gives nil and never panics. (A real window cannot be opened in a test;
// this covers the lookup the navigation guard depends on.)
func TestChromiumOfFindsTheBrowserBehindTheWrapperField(t *testing.T) {
	type wrapper struct {
		hwnd    uintptr
		other   interface{ String() string }
		browser focuser
	}
	ch := edge.NewChromium()
	if got := chromiumOf(&wrapper{browser: ch}); got != ch {
		t.Fatalf("the Chromium held in an interface field is found, got %v", got)
	}
	for name, v := range map[string]any{
		"another browser":  &wrapper{browser: notChromium{}},
		"an empty field":   &wrapper{},
		"nil":              nil,
		"a nil pointer":    (*wrapper)(nil),
		"not a pointer":    wrapper{browser: ch},
		"not a struct":     new(int),
		"a typed nil held": &wrapper{browser: (*edge.Chromium)(nil)},
	} {
		if got := chromiumOf(v); got != nil {
			t.Errorf("%s: want nil, got %v", name, got)
		}
	}
}

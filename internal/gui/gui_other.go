//go:build !windows

package gui

// Supported reports whether a native window can be opened on this platform.
func Supported() bool { return false }

// Run is not available off Windows.
func Run(url, title string, width, height uint) error { return ErrUnsupported }

// MessageBox prints nothing off Windows; callers also write to stderr.
func MessageBox(title, text string) {}

// PickFile is not available off Windows.
func PickFile(title string) (string, error) { return "", ErrUnsupported }

// AttachConsole is a no-op off Windows.
func AttachConsole() {}

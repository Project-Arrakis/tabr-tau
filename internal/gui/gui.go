// Package gui is the native Windows shell around the local web UI: a real window (WebView2) instead of a browser
// tab, a native file-open dialog and message boxes for errors that would otherwise go to an invisible console.
//
// The Windows implementation lives in gui_windows.go; every other platform gets the stubs in gui_other.go and keeps
// using the browser. The window only ever shows the editor's own loopback address.
package gui

import "errors"

// ErrUnsupported is returned on platforms without a native window.
var ErrUnsupported = errors.New("a native window is not available on this platform")

// ErrNoRuntime is returned when the Microsoft Edge WebView2 runtime is not installed.
var ErrNoRuntime = errors.New("the Microsoft Edge WebView2 runtime is not installed (https://developer.microsoft.com/microsoft-edge/webview2/)")

// WantWindow decides whether to open the native window: only on Windows, only when the platform supports it, only
// when the operator did not ask for the browser, and never for a command-line subcommand.
func WantWindow(goos string, supported, forceWeb bool, subcommand bool) bool {
	return goos == "windows" && supported && !forceWeb && !subcommand
}

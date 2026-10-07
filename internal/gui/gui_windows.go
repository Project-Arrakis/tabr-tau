//go:build windows

package gui

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2"
)

// Supported reports whether a native window can be opened on this platform.
func Supported() bool { return true }

// Run opens a window showing url and blocks until it is closed. WebView2 keeps its profile (cache, cookies) under
// %LOCALAPPDATA%\tabr-tau\webview2, never next to the exe (which may sit in the game's save folder). Developer tools
// and the context menu are off. It returns ErrNoRuntime when WebView2 is not installed.
func Run(url, title string, width, height uint) error {
	data := filepath.Join(os.Getenv("LOCALAPPDATA"), "tabr-tau", "webview2")
	if err := os.MkdirAll(data, 0o700); err != nil {
		return err
	}
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		DataPath:  data,
		WindowOptions: webview2.WindowOptions{
			Title: title, Width: width, Height: height, Center: true,
		},
	})
	if w == nil {
		return ErrNoRuntime
	}
	defer w.Destroy()
	w.Navigate(url)
	w.Run()
	return nil
}

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	comdlg32        = syscall.NewLazyDLL("comdlg32.dll")
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procMessageBoxW = user32.NewProc("MessageBoxW")
	procGetOpenFile = comdlg32.NewProc("GetOpenFileNameW")
	procAttach      = kernel32.NewProc("AttachConsole")
)

// MessageBox shows a modal error box. Used because a GUI-subsystem exe has no console to print to.
func MessageBox(title, text string) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	const mbOK, mbIconError = 0x0, 0x10
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), mbOK|mbIconError)
}

// AttachConsole connects a GUI-subsystem exe to the console it was started from (ATTACH_PARENT_PROCESS), so
// subcommands such as `tabr-tau diff` still print when run from cmd or PowerShell. It does nothing when started by
// double-click, and the standard handles are rebound so fmt.Print* reaches the console.
func AttachConsole() {
	const attachParentProcess = ^uintptr(0) // (DWORD)-1
	if r, _, _ := procAttach.Call(attachParentProcess); r == 0 {
		return
	}
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout, os.Stderr = f, f
	}
	if f, err := os.OpenFile("CONIN$", os.O_RDONLY, 0); err == nil {
		os.Stdin = f
	}
}

// openFileName mirrors OPENFILENAMEW (Win32). Go's natural struct alignment matches the C layout on both 386 and amd64.
type openFileName struct {
	lStructSize       uint32
	hwndOwner         uintptr
	hInstance         uintptr
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	flagsEx           uint32
}

// PickFile shows the standard Open dialog for a save file (game.db or an autosave) and returns the chosen path, or
// "" when the user cancels.
func PickFile(title string) (string, error) {
	buf := make([]uint16, 32768)
	// Filter pairs are separated by NULs and the list ends with a double NUL.
	filter := utf16Z("Dune save (game.db, *.bak)\x00game.db;*.bak\x00All files\x00*.*\x00")
	t, _ := syscall.UTF16PtrFromString(title)
	var dir *uint16
	if base := os.Getenv("LOCALAPPDATA"); base != "" {
		dir, _ = syscall.UTF16PtrFromString(filepath.Join(base, "DuneSandbox", "Saved", "Cloud", "PlayerClientStorage"))
	}
	const ofnFileMustExist, ofnPathMustExist, ofnNoChangeDir, ofnExplorer = 0x1000, 0x800, 0x8, 0x80000
	ofn := openFileName{
		lpstrFilter:     &filter[0],
		nFilterIndex:    1,
		lpstrFile:       &buf[0],
		nMaxFile:        uint32(len(buf)),
		lpstrInitialDir: dir,
		lpstrTitle:      t,
		flags:           ofnFileMustExist | ofnPathMustExist | ofnNoChangeDir | ofnExplorer,
	}
	ofn.lStructSize = uint32(unsafe.Sizeof(ofn))
	r, _, _ := procGetOpenFile.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return "", nil // cancelled (CommDlgExtendedError would be 0); other failures also read as "no file chosen"
	}
	return syscall.UTF16ToString(buf), nil
}

// utf16Z converts s, which may contain NULs, to UTF-16 without truncating at the first NUL, and appends the final NUL.
func utf16Z(s string) []uint16 {
	out := make([]uint16, 0, len(s)+1)
	for _, r := range s {
		out = append(out, uint16(r))
	}
	return append(out, 0)
}

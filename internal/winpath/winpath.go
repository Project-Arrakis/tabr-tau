// Package winpath names Windows system programs by absolute path.
package winpath

import (
	"os"
	"path/filepath"
)

// System32 is the full path of a program in the Windows system folder, for example System32("tasklist.exe"). Starting a program by bare
// name searches the folders in PATH (and, for some callers, the current folder), so a tasklist.exe planted earlier in the search could
// report "no game is running", and a planted rundll32.exe would run with the editor's rights (SEC-8 of the 2026-10-15 audit). The system
// folder comes from the environment variable Windows itself sets, with C:\Windows as the last resort.
func System32(exe string) string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = os.Getenv("windir")
	}
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", exe)
}

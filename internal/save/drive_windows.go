package save

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

var procGetDriveType = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDriveTypeW")

const driveFixed = 3 // DRIVE_FIXED

// driveProblem says why the drive holding p is not a fixed local disk (a removable, network or optical drive, or one that cannot be
// told), or "" when it is. A path with no drive letter is left to pathProblem.
func driveProblem(p string) string {
	vol := filepath.VolumeName(p)
	if len(vol) != 2 || vol[1] != ':' {
		return ""
	}
	root, err := syscall.UTF16PtrFromString(vol + `\`)
	if err != nil {
		return "the drive the save is on cannot be identified"
	}
	if t, _, _ := procGetDriveType.Call(uintptr(unsafe.Pointer(root))); t != driveFixed {
		return "the save is on a removable or network drive"
	}
	return ""
}

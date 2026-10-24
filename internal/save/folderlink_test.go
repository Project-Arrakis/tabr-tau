package save

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

// makeFolderLink makes link point at target: a symbolic link, or on Windows (where that needs a privilege) a junction, which is what a person without the privilege can make.
// It skips the test only when neither can be made.
func makeFolderLink(t *testing.T, link, target string) {
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return
	}
	if runtime.GOOS == "windows" {
		if out, jerr := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); jerr == nil {
			return
		} else {
			t.Skipf("no link could be made: %v; %v %s", err, jerr, out)
		}
	}
	t.Skip(err)
}

package ops

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// An acceptance run on a real pair of saves (#1127): a save before the game's own base backup and the save the game wrote after it. It is for the person who has such a pair, and it
// skips unless the three variables name one. It copies the "before" save, backs up the base there with this editor and leaves the result where TABR_REMOVE_OUT says, so that
// `tabr-tau diff` can compare it with the "after" save the game wrote. The saves are never read from the repository and nothing of them is kept here.
//
//	TABR_REMOVE_BEFORE  path of the save before the game's backup
//	TABR_REMOVE_TOTEM   the totem of the base the game backed up
//	TABR_REMOVE_OUT     path of the copy to write (must not exist)
func TestBackUpAndRemoveOnARealSaveBeforeTheGamesOwnBackup(t *testing.T) {
	src, out, tid := os.Getenv("TABR_REMOVE_BEFORE"), os.Getenv("TABR_REMOVE_OUT"), os.Getenv("TABR_REMOVE_TOTEM")
	if src == "" || out == "" || tid == "" {
		t.Skip("no real before-save named (TABR_REMOVE_BEFORE, TABR_REMOVE_TOTEM, TABR_REMOVE_OUT)")
	}
	totem, err := strconv.ParseInt(tid, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("the output file exists; name one that does not")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(src)
	if err != nil {
		t.Skipf("the before-save is not readable here: %v", err)
	}
	g, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(g, f); err != nil {
		t.Fatal(err)
	}
	f.Close()
	g.Close()
	_ = testsave.HostileText // the package's init makes the process check report no game
	s, err := save.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	o := &Ops{S: s}
	pv, err := o.PreviewBaseRemoval(totem)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	fp := pv.(map[string]any)["fingerprint"].(string)
	res, err := o.BackUpAndRemoveBase(totem, fp)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	t.Logf("result: %v", res)
	if _, err := s.CommitForTests(); err != nil {
		t.Fatal(err)
	}
}

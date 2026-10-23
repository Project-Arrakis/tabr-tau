package save

import (
	"path/filepath"
	"strings"
	"testing"
)

// Review 2026-10-23 B3: a description in the write log must not carry the primary-key value of an edited row, a long hexadecimal id or GUID, or a file path (the last can come
// with a warning's text).
func TestWriteLogDescriptionsCarryNoKeysHexIdsOrPaths(t *testing.T) {
	for in, want := range map[string]string{
		"update vendor_stock_state[0A1B2C3D4E5F60718293A4B5C6D7E8F9,Spice].amount_bought": "update vendor_stock_state[<key>].amount_bought",
		"update items[42].stack_size":                                                "update items[<key>].stack_size",
		"moved vehicle 201 next to player":                                           "moved vehicle 201 next to player",
		"Give 500 x Spice to container 12":                                           "Give 500 x Spice to container 12",
		"claim 0A1B2C3D4E5F60718293A4B5C6D7E8F9 renamed":                             "claim <id> renamed",
		"owner 123e4567-e89b-12d3-a456-426614174000 set":                             "owner <id> set",
		`saved, but reloading failed: open C:\Users\someone\AppData\game.db: denied`: `saved, but reloading failed: open <path> denied`,
		`read \\server\share\Saved\game.db failed`:                                   `read <path> failed`,
		"item of account 12345678901234567":                                          "item of account <number>",
	} {
		got := logDesc(in)
		// the path pattern takes the colon after the file name along with the name; compare without it
		got, want = strings.ReplaceAll(got, "<path>:", "<path>"), strings.ReplaceAll(want, "<path>:", "<path>")
		if got != want {
			t.Errorf("logDesc(%q) = %q, want %q", in, got, want)
		}
	}
}

// A restore replaces the game's file, so it is recorded in the write log like any other write, with the backup that holds the file as it was before.
func TestRestoreAddsALineToTheWriteLog(t *testing.T) {
	defer NoGameRunningForTests()()
	s, lp := loggedSave(t)
	edit(t, s, "Rename row one", `update jt set name='uno' where a=1`)
	f := s.Flush()
	if !f.Saved {
		t.Fatalf("setup: %+v", f)
	}
	name := filepath.Base(f.Backup)
	if err := s.Restore(name); err != nil {
		t.Fatal(err)
	}
	lines := readLog(t, lp)
	if len(lines) != 2 {
		t.Fatalf("the write and the restore are two lines: %+v", lines)
	}
	if got := lines[1]; len(got.Ops) != 1 || got.Ops[0] != "restored the backup "+name || got.Backup == "" || got.Backup == name {
		t.Fatalf("the restore line names what was restored and the backup taken before it: %+v", got)
	}
}

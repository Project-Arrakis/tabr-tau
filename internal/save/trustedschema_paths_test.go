package save

import (
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every connection that opens a save file runs with trusted_schema OFF (register 2026-09-30 F-03, review R6 L3): the working copy, its read-only handle, the baseline copy, the
// copy that Restore and the integrity check inspect (the current save and every backup chosen for Restore), and the read-only open of the diff. These tests ask each of them.

func pragmaTrusted(t *testing.T, db *sql.DB, what string) {
	t.Helper()
	var v int
	if err := db.QueryRow(`pragma trusted_schema`).Scan(&v); err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	if v != 0 {
		t.Errorf("%s trusts the schema of the save (trusted_schema=%d)", what, v)
	}
}

func TestEveryConnectionThatOpensASaveDoesNotTrustItsSchema(t *testing.T) {
	p := makeSave(t, `create table ev(n integer); insert into ev values (0);`)

	// inspect: the copy checked before a Restore and when the current file is compared with a backup
	f, err := tempCopy("tabr-tau-inspect-*.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	defer os.Remove(name)
	blob, _ := os.ReadFile(p)
	raw, err := Decode(blob)
	if err != nil {
		t.Fatal(err)
	}
	f.Write(raw)
	f.Close()
	idb, err := openInspect(name)
	if err != nil {
		t.Fatal(err)
	}
	defer idb.Close()
	pragmaTrusted(t, idb, "inspect")
	if _, err := inspect(raw); err != nil {
		t.Fatalf("inspect still works on a sound save: %v", err)
	}

	// OpenReadOnly (the diff)
	rdb, closeRO, err := OpenReadOnly(p)
	if err != nil {
		t.Fatal(err)
	}
	defer closeRO()
	pragmaTrusted(t, rdb, "the read-only open")

	// the working copy, its read handle and the baseline copy
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.mu.Lock()
	pragmaTrusted(t, s.db, "the working copy")
	pragmaTrusted(t, s.rdb, "the read handle")
	s.mu.Unlock()
	if err := s.WithBaseline(func(orig, cur *sql.DB) error {
		pragmaTrusted(t, orig, "the baseline copy")
		pragmaTrusted(t, cur, "the current copy")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// A new way to open a save file must say what it does about the schema: every sql.Open of a file in shipped code, and every openCopy, names trusted_schema(0). (The in-memory
// databases of the fingerprint and of the tests open no save.)
func TestNoShippedCodeOpensASaveWithoutSwitchingTheSchemaTrustOff(t *testing.T) {
	root := filepath.Join("..", "..")
	open := regexp.MustCompile(`sql\.Open\("sqlite",`)
	copyCall := regexp.MustCompile(`\bopenCopy\(`)
	checked := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".claude", "node_modules", "web-tests", "testsave":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			code := line
			if c := strings.Index(code, "//"); c >= 0 && !strings.Contains(code[:c], `"`) {
				code = code[:c]
			}
			switch {
			case open.MatchString(code) && !strings.Contains(code, `":memory:"`):
				checked++
				if !strings.Contains(code, "trusted_schema(0)") && !strings.Contains(code, `"?"+opts`) {
					t.Errorf("%s:%d opens a database file without trusted_schema(0): %s", path, i+1, strings.TrimSpace(line))
				}
			case copyCall.MatchString(code) && !strings.Contains(code, "func openCopy"):
				checked++
				if !strings.Contains(code, "trusted_schema(0)") {
					t.Errorf("%s:%d opens a copy of a save without trusted_schema(0): %s", path, i+1, strings.TrimSpace(line))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 5 {
		t.Fatalf("the guard looked at %d places; it no longer finds the code it guards", checked)
	}
}

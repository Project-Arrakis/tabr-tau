package save

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// #419: the editor's automatic pass wrote the save at the character-selection screen, 23 seconds after the player left the world, while the game process was still
// running. The old gate allowed a write whenever the game's log showed no single-player session. The rule now is the plain one: nothing writes the save while a game
// process exists, or while the process list cannot be read, whatever the log says.

// gameSituations are the ways a game process can be there. None of them may let a write through.
var gameSituations = map[string]func(t *testing.T){
	"the game at its menu (no session in the log)": func(t *testing.T) { fakeGame(t, duneProc, "", spStatus{}, nil) },
	"logged out to character selection 23 s ago (the #419 case)": func(t *testing.T) {
		fakeGame(t, duneProc, "", spStatus{EndedAt: time.Now().Add(-23 * time.Second)}, nil)
	},
	"the session ended an hour ago":     func(t *testing.T) { fakeGame(t, duneProc, "", spStatus{EndedAt: time.Now().Add(-time.Hour)}, nil) },
	"a single-player session is active": func(t *testing.T) { fakeGame(t, duneProc, "", spStatus{Active: true}, nil) },
	"the log cannot be read":            func(t *testing.T) { fakeGame(t, duneProc, "", spStatus{}, errors.New("no log")) },
	"the process list cannot be read":   func(t *testing.T) { fakeGame(t, nil, "process snapshot: access denied", spStatus{}, nil) },
	"a launcher process only":           func(t *testing.T) { fakeGame(t, []string{"DuneAwakening.exe"}, "", spStatus{}, nil) },
}

// writeEntryPoints lists every route by which the editor can put bytes into the save file. A new route must be added here, and
// TestTheSaveFileHasOneWriterBehindTheGate fails when the code has a route this list does not know about.
var writeEntryPoints = map[string]func(t *testing.T, s *Save, p string, backup string) (wrote bool){
	"Flush (the automatic write after every edit, the 3 s retry, the pass at launch)": func(t *testing.T, s *Save, _ string, _ string) bool {
		return s.Flush().Saved
	},
	"commit quick (what Flush calls)": func(t *testing.T, s *Save, _ string, _ string) bool {
		res, err := s.commit(true)
		return err == nil && res["saved"] == true
	},
	"commit full (Write now)": func(t *testing.T, s *Save, _ string, _ string) bool {
		res, err := s.CommitForTests()
		return err == nil && res["saved"] == true
	},
	"Restore a backup": func(t *testing.T, s *Save, _ string, backup string) bool {
		s.Discard() // restoring refuses with edits pending for another reason; the gate is what is under test
		return s.Restore(backup) == nil
	},
}

func TestNoWriteRouteWritesWhileAGameProcessExists(t *testing.T) {
	for sname, setup := range gameSituations {
		for ename, enter := range writeEntryPoints {
			t.Run(sname+" / "+ename, func(t *testing.T) {
				p := makeSave(t, `create table t(a integer);`)
				s, err := Open(p)
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()
				// a backup to restore, made while the game is closed
				if _, err := s.ExecScript(`insert into t values (1)`); err != nil {
					t.Fatal(err)
				}
				if _, err := s.CommitForTests(); err != nil {
					t.Fatal(err)
				}
				backup := s.Backups()[0]["name"].(string)
				if _, err := s.ExecScript(`insert into t values (2)`); err != nil {
					t.Fatal(err)
				}
				before, _ := os.ReadFile(p)
				nBackups := len(s.Backups())
				setup(t)
				if enter(t, s, p, backup) {
					t.Fatal("the route wrote while a game process exists or the check cannot say")
				}
				after, _ := os.ReadFile(p)
				if !bytes.Equal(before, after) {
					t.Fatal("the save file changed")
				}
				// commit(true) is what Flush calls after its own check; it may take a backup before installBlob refuses at the gate, so only the file is compared there
				if len(s.Backups()) != nBackups && !strings.HasPrefix(ename, "commit quick") {
					t.Fatalf("a refused write made a backup: %d", len(s.Backups()))
				}
			})
		}
	}
}

// With the game gone the same edit is written (the positive control) and says it was queued while the game ran.
func TestTheQueuedEditIsWrittenOnceTheGameHasClosed(t *testing.T) {
	p := makeSave(t, `create table t(a integer);`)
	s, _ := Open(p)
	defer s.Close()
	s.ExecScript(`insert into t values (1)`)
	fakeGame(t, duneProc, "", spStatus{}, nil)
	f := s.Flush()
	if f.Saved || f.Queued != 1 || !strings.Contains(f.Why, "is running") || !strings.Contains(f.Why, "written when the game closes") {
		t.Fatalf("while the game runs: %+v", f)
	}
	fakeGame(t, nil, "", spStatus{}, nil)
	if f := s.Flush(); !f.Saved || f.Queued != 0 {
		t.Fatalf("after the game closed: %+v", f)
	}
}

// The journal of edits left by an earlier run comes back as waiting edits and is written by the same gate.
func TestJournalReplayDoesNotWriteWhileAGameProcessExists(t *testing.T) {
	for sname, setup := range gameSituations {
		t.Run(sname, func(t *testing.T) {
			p := makeSave(t, journalDDL)
			dir := t.TempDir()
			s1 := openJournaled(t, p, dir)
			if _, err := s1.Mutate("rename one", func(m *Mut) error {
				_, err := m.Exec(`update jt set name='uno' where a=1`)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			s1.Close()
			before, _ := os.ReadFile(p)
			setup(t)
			s2 := openJournaled(t, p, dir)
			if _, err := s2.RestoreJournal(); err != nil {
				t.Fatal(err)
			}
			if s2.Flush().Saved {
				t.Fatal("the replayed edits were written under the game")
			}
			after, _ := os.ReadFile(p)
			if !bytes.Equal(before, after) {
				t.Fatal("the save file changed")
			}
			if !s2.Dirty() {
				t.Fatal("the replayed edit must stay queued")
			}
		})
	}
}

// The static half: the only code that puts bytes into the save path is installBlob, it is reached from commit and Restore only, and it runs the game gate before the rename.
// A new route to the file (a second rename, another caller of installBlob, a direct write of the save path) fails here until it is put behind the gate and listed.
func TestTheSaveFileHasOneWriterBehindTheGate(t *testing.T) {
	root := filepath.Join("..", "..")
	callers := map[string][]string{} // function called -> "pkg.Func" of the functions that call it
	watch := map[string]bool{"installBlob": true, "renameRetry": true, "commit": true, "Rename": true, "WriteFile": true, "OpenFile": true, "Create": true, "Truncate": true}
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
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		dir := filepath.ToSlash(filepath.Dir(rel))
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var name string
				switch fun := c.Fun.(type) {
				case *ast.SelectorExpr:
					name = fun.Sel.Name
					if name == "Rename" || name == "WriteFile" || name == "OpenFile" || name == "Create" || name == "Truncate" {
						if x, ok := fun.X.(*ast.Ident); !ok || x.Name != "os" {
							return true // a method of something else with that name, not the os function
						}
					}
				case *ast.Ident:
					name = fun.Name
				}
				if watch[name] {
					callers[name] = append(callers[name], dir+"."+fn.Name.Name)
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for k := range callers {
		sort.Strings(callers[k])
	}
	// who may reach installBlob, and who may call commit (Flush is the automatic write; CommitForTests only in tests)
	wantExact := map[string][]string{
		"installBlob": {"internal/save.Restore", "internal/save.commit"},
		"commit":      {"internal/save.CommitForTests", "internal/save.Flush"},
		"renameRetry": {"internal/save.installBlob"},
	}
	for name, want := range wantExact {
		got := callers[name]
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s is called from %v, want exactly %v: a new route to the save file must go through the game gate and be listed here and in writeEntryPoints", name, got, want)
		}
	}
	// the other writers of files are the known ones: none of them is given the save's path (the journal, the write log, the backups, the work copy, the
	// lock, the config files, the command line's decode and encode of a file the person names)
	for _, name := range []string{"Rename", "WriteFile", "OpenFile", "Create", "Truncate"} {
		for _, c := range callers[name] {
			if !knownFileWriters[c] {
				t.Errorf("os.%s is called from %s, which is not a known file writer: if it can reach the save file it must go through installBlob", name, c)
			}
		}
	}
	// installBlob asks the gate before the rename, in source order
	b, err := os.ReadFile("durable.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	i := strings.Index(src, "func (s *Save) installBlob(")
	if i < 0 {
		t.Fatal("installBlob not found")
	}
	body := src[i:]
	g, r := strings.Index(body, "SaveBlocked(s.Path)"), strings.Index(body, "renameRetry(tmp, s.Path)")
	if g < 0 || r < 0 || g > r {
		t.Fatalf("installBlob must ask SaveBlocked before the rename (gate at %d, rename at %d)", g, r)
	}
}

// knownFileWriters are the functions that create or replace files other than the save: none of them writes the save path.
var knownFileWriters = map[string]bool{
	"internal/save.writeDurable":   true, // the temp file next to the save and the journal's temp file; the rename into the save is installBlob's
	"internal/save.skipJournal":    true, // moves an unreadable journal aside
	"internal/save.writeLogAppend": true, // the editor's own write log
	"internal/save.lockFile":       true, // the lock file
	"internal/save.markIncomplete": true, // the journal's incomplete-write mark
	"internal/config.write":        true, // the game's config files, never the save
	"internal/settings.save":       true, // the editor's own settings file
	"internal/gui.AttachConsole":   true, // the console handles of the process
	"cmd/tabr-tau.runCommand":      true, // decode and encode write the file the person names
}

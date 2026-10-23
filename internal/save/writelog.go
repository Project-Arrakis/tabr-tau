package save

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// The write log is the editor's own record of what it wrote to the game's file: one line (JSON) per write, with when, how many edits, what they were in words, and which backup
// holds the file as it was before. It is appended after the file has been written, so a line means the write happened; it is not the way back (the backup is) and the editor
// does not read it. It is kept in the editor's own folder (created with mode 0600, which Windows ignores: there the file has the permissions of the profile folder). It does hold
// some of what the save holds: the descriptions are the sentences the page lists as waiting changes, which carry game ids and amounts ("Give 500 x ... to container 12"). Not copied:
// a statement typed into the SQL console (only that one was run), the primary-key value of a row edited in the Database view, a number of fifteen digits or more (an account or
// platform id), a long hexadecimal id or GUID, and a file path. It is capped: when it reaches writeLogMax bytes it becomes write-log.1.jsonl (the older generation is dropped) and
// a new file starts, so there are never more than two files.

var (
	writeLogMax     int64 = 256 << 10 // bytes in one generation
	writeLogDescMax       = 200       // characters of one description
	writeLogOpsMax        = 100       // descriptions of one line; the rest are counted
)

var (
	longNumber = regexp.MustCompile(`\d{15,}`)
	hexID      = regexp.MustCompile(`\b[0-9a-fA-F]{16,}\b|\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)
	filePath   = regexp.MustCompile(`(?:[A-Za-z]:[\\/]|\\\\)[^\s"'<>|]+|/(?:home|Users|tmp|var|mnt|private)/[^\s"'<>|]+`)
	cellEdit   = regexp.MustCompile(`^(update [^\[\]\s]+)\[.*\](\.\S+)$`) // "update table[primary-key values].column" of the Database view
)

type writeLogLine struct {
	At      string   `json:"at"`
	Edits   int      `json:"edits"`
	Ops     []string `json:"ops,omitempty"`
	More    int      `json:"more,omitempty"` // edits beyond the descriptions kept
	Backup  string   `json:"backup,omitempty"`
	Warning string   `json:"warning,omitempty"`
}

// EnableWriteLog records every write in the file at path from now on. It is called once, after Open, by whoever owns the editor's own folder.
func (s *Save) EnableWriteLog(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logPath = path
}

// WriteLogProblem says why the last write could not be recorded ("" when it was, or when there is no log).
func (s *Save) WriteLogProblem() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logProblem
}

// logDesc is a description fit for the log: no SQL text, no primary-key values, no long numbers or hexadecimal ids, no file paths, a bounded length.
func logDesc(d string) string {
	if strings.HasPrefix(d, "sql:") {
		return "SQL console statement (its text is not recorded)"
	}
	d = cellEdit.ReplaceAllString(d, "$1[<key>]$2")
	d = filePath.ReplaceAllString(d, "<path>")
	d = longNumber.ReplaceAllString(d, "<number>")
	d = hexID.ReplaceAllString(d, "<id>")
	if utf8.RuneCountInString(d) > writeLogDescMax {
		d = string([]rune(d)[:writeLogDescMax]) + "..."
	}
	return d
}

// appendWriteLog adds the line for a write that has just happened. A failure is kept (WriteLogProblem) and never undoes or hides the write.
func (s *Save) appendWriteLog(f Flush) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appendWriteLogLocked(f)
}

// appendWriteLogLocked is appendWriteLog for a caller that holds s.mu (Restore).
func (s *Save) appendWriteLogLocked(f Flush) {
	if s.logPath == "" {
		return
	}
	if err := writeLogAppend(s.logPath, f); err != nil {
		s.logProblem = "the write could not be recorded in the write log (" + shortErr(err) + ")"
	} else {
		s.logProblem = ""
	}
}

func writeLogAppend(path string, f Flush) error {
	line := writeLogLine{At: f.At.UTC().Format(time.RFC3339), Edits: len(f.Ops), Backup: filepath.Base(f.Backup)}
	if f.Backup == "" {
		line.Backup = ""
	}
	for i, d := range f.Ops {
		if i >= writeLogOpsMax {
			line.More = len(f.Ops) - i
			break
		}
		line.Ops = append(line.Ops, logDesc(d))
	}
	if f.Warning != "" {
		line.Warning = logDesc(f.Warning)
	}
	b, err := json.Marshal(line)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if st, err := os.Stat(path); err == nil && st.Size()+int64(len(b)) > writeLogMax {
		old := strings.TrimSuffix(path, filepath.Ext(path)) + ".1" + filepath.Ext(path)
		os.Remove(old)
		if err := os.Rename(path, old); err != nil {
			return err
		}
	}
	w, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		w.Close()
		return err
	}
	if err := syncFile(w); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

package save

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
)

// OpenReadOnly decodes a save (game.db, an autosave .bak, a folder holding game.db, or an already plain SQLite
// file) into a private temp file and opens it read-only. The returned function closes the handle and deletes the
// temp file. Nothing is ever written to path.
func OpenReadOnly(path string) (*sql.DB, func(), error) {
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		path = filepath.Join(path, "game.db")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	raw := b
	if !bytes.HasPrefix(b, []byte("SQLite format 3\x00")) {
		if raw, err = Decode(b); err != nil {
			return nil, nil, err
		}
	}
	f, err := os.CreateTemp("", "tabr-tau-ro-*.sqlite")
	if err != nil {
		return nil, nil, err
	}
	name := f.Name()
	if _, err := f.Write(raw); err != nil {
		f.Close()
		os.Remove(name)
		return nil, nil, err
	}
	f.Close()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(name)+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		os.Remove(name)
		return nil, nil, err
	}
	db.SetMaxOpenConns(1)
	return db, func() { db.Close(); os.Remove(name) }, nil
}

// Package save reads and writes Dune: Awakening client saves.
//
// game.db, game_prepatch.db and autosave/*.bak share one container: a
// little-endian uint32 flag (1), a little-endian uint32 uncompressed size and a
// zlib stream whose payload is a plain SQLite 3 database.
package save

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var sqliteMagic = []byte("SQLite format 3\x00")

const flagZlib = 1

// Decode returns the SQLite bytes contained in a save container. A file that is
// already a plain SQLite database is returned unchanged.
func Decode(blob []byte) ([]byte, error) {
	if bytes.HasPrefix(blob, sqliteMagic) {
		return blob, nil
	}
	if len(blob) < 10 {
		return nil, errors.New("file too small to be a Dune save")
	}
	flag := binary.LittleEndian.Uint32(blob[0:4])
	size := binary.LittleEndian.Uint32(blob[4:8])
	if flag != flagZlib {
		return nil, fmt.Errorf("unknown container flag %d", flag)
	}
	zr, err := zlib.NewReader(bytes.NewReader(blob[8:]))
	if err != nil {
		return nil, fmt.Errorf("zlib: %w", err)
	}
	defer zr.Close()
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("zlib: %w", err)
	}
	if uint32(len(raw)) != size {
		return nil, fmt.Errorf("size mismatch: header says %d, got %d", size, len(raw))
	}
	if !bytes.HasPrefix(raw, sqliteMagic) {
		return nil, errors.New("payload is not a SQLite database")
	}
	return raw, nil
}

// Encode wraps SQLite bytes in the save container.
func Encode(raw []byte) ([]byte, error) {
	if !bytes.HasPrefix(raw, sqliteMagic) {
		return nil, errors.New("refusing to encode a non-SQLite payload")
	}
	var buf bytes.Buffer
	hdr := make([]byte, 8)
	binary.LittleEndian.PutUint32(hdr[0:4], flagZlib)
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(raw)))
	buf.Write(hdr)
	zw, _ := zlib.NewWriterLevel(&buf, 6)
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

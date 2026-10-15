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
	"os"
)

var sqliteMagic = []byte("SQLite format 3\x00")

const flagZlib = 1

// maxDecoded is the largest save (compressed or decoded) that Decode accepts.
// Real saves decode to ~2 MB; the cap only exists to stop hostile files.
var maxDecoded = int64(256 << 20)

// readCapped reads a file the user pointed the editor at, refusing one over maxDecoded from its size before any byte is read, and never
// reading more than maxDecoded+1 bytes even when the size changes under it. os.ReadFile would read a multi-gigabyte file in full and only
// then let Decode say it is too big (SEC-6 of the 2026-10-15 audit).
func readCapped(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.Size() > maxDecoded {
		return nil, fmt.Errorf("file is %d bytes, over the %d byte limit", st.Size(), maxDecoded)
	}
	b, err := io.ReadAll(io.LimitReader(f, maxDecoded+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maxDecoded {
		return nil, fmt.Errorf("file is over the %d byte limit", maxDecoded)
	}
	return b, nil
}

// Decode returns the SQLite bytes contained in a save container. A file that is
// already a plain SQLite database is returned unchanged.
//
// The input is untrusted (users share saves): the declared size is checked against
// maxDecoded before any decompression, and the stream is read through a limit of
// declared size + 1, so a small file can never expand into unbounded memory.
func Decode(blob []byte) ([]byte, error) {
	if int64(len(blob)) > maxDecoded {
		return nil, fmt.Errorf("file is %d bytes, over the %d byte limit", len(blob), maxDecoded)
	}
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
	if int64(size) > maxDecoded {
		return nil, fmt.Errorf("declared size %d exceeds the %d byte limit", size, maxDecoded)
	}
	zr, err := zlib.NewReader(bytes.NewReader(blob[8:]))
	if err != nil {
		return nil, fmt.Errorf("zlib: %w", err)
	}
	defer zr.Close()
	raw, err := io.ReadAll(io.LimitReader(zr, int64(size)+1))
	if err != nil {
		return nil, fmt.Errorf("zlib: %w", err)
	}
	if int64(len(raw)) != int64(size) {
		return nil, fmt.Errorf("size mismatch: header says %d, stream has %s", size, streamLen(len(raw), int64(size)))
	}
	if !bytes.HasPrefix(raw, sqliteMagic) {
		return nil, errors.New("payload is not a SQLite database")
	}
	return raw, nil
}

func streamLen(got int, declared int64) string {
	if int64(got) > declared {
		return "more"
	}
	return fmt.Sprint(got)
}

// Encode wraps SQLite bytes in the save container.
func Encode(raw []byte) ([]byte, error) {
	if !bytes.HasPrefix(raw, sqliteMagic) {
		return nil, errors.New("refusing to encode a non-SQLite payload")
	}
	if int64(len(raw)) > maxDecoded {
		return nil, fmt.Errorf("payload is %d bytes, over the %d byte limit", len(raw), maxDecoded)
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

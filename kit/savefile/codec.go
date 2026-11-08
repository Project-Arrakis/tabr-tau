// Package savefile reads and writes the container that Dune: Awakening client saves are stored in.
//
// game.db, game_prepatch.db and autosave/*.bak share one container: a
// little-endian uint32 flag (1), a little-endian uint32 uncompressed size and a
// zlib stream whose payload is a plain SQLite 3 database. Decode and Encode
// convert between that container and the SQLite bytes; ReadCapped reads a file
// without ever holding more than Limit bytes of it.
//
// Every input is treated as untrusted: a save is a file people share, and a
// small file can claim to hold a huge database.
package savefile

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

// Limit is the largest save, in bytes, that Decode, Encode and ReadCapped accept, compressed or decoded. Real saves decode to about
// 2 MB; the cap only exists to stop hostile files.
const Limit = 256 << 20

// limit is Limit as the functions use it. It is a variable only so that this package's tests can lower it.
var limit int64 = Limit

// ReadCapped reads a file a person pointed the program at, refusing one over Limit from its size before any byte is read, and never
// reading more than Limit+1 bytes even when the size changes under it. os.ReadFile would read a multi-gigabyte file in full and only
// then let Decode say it is too big (finding SEC-6 of the editor's 2026-10-15 audit).
func ReadCapped(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.Size() > limit {
		return nil, fmt.Errorf("file is %d bytes, over the %d byte limit", st.Size(), limit)
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("file is over the %d byte limit", limit)
	}
	return b, nil
}

// Decode returns the SQLite bytes contained in a save container. A file that is
// already a plain SQLite database is returned unchanged.
//
// The input is untrusted (users share saves): the declared size is checked against
// Limit before any decompression, and the stream is read through a limit of
// declared size + 1, so a small file can never expand into unbounded memory.
func Decode(blob []byte) ([]byte, error) {
	if int64(len(blob)) > limit {
		return nil, fmt.Errorf("file is %d bytes, over the %d byte limit", len(blob), limit)
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
	if int64(size) > limit {
		return nil, fmt.Errorf("declared size %d exceeds the %d byte limit", size, limit)
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
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("payload is %d bytes, over the %d byte limit", len(raw), limit)
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

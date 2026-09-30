package save

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"math/rand"
	"runtime"
	"strings"
	"testing"
)

// container builds a save container with an explicit header, independent of Encode, so tests can lie about the size.
func container(flag, declared uint32, payload []byte) []byte {
	var buf bytes.Buffer
	hdr := make([]byte, 8)
	binary.LittleEndian.PutUint32(hdr[0:4], flag)
	binary.LittleEndian.PutUint32(hdr[4:8], declared)
	buf.Write(hdr)
	zw := zlib.NewWriter(&buf)
	zw.Write(payload)
	zw.Close()
	return buf.Bytes()
}

func sqlitePayload(n int) []byte {
	p := make([]byte, n)
	copy(p, sqliteMagic)
	rand.New(rand.NewSource(1)).Read(p[len(sqliteMagic):])
	return p
}

func withLimit(t *testing.T, n int64) {
	t.Helper()
	old := maxDecoded
	maxDecoded = n
	t.Cleanup(func() { maxDecoded = old })
}

func TestDecodeRoundTripRealSize(t *testing.T) {
	raw := sqlitePayload(3 << 20) // real saves decode to ~1.7 MB
	blob, err := Encode(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(blob)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("round trip failed: err=%v equal=%v", err, bytes.Equal(got, raw))
	}
}

// A small file whose stream expands far beyond its header must be stopped early, with bounded memory (SEC-1).
func TestDecodeBombLyingHeader(t *testing.T) {
	bomb := append(append([]byte{}, sqliteMagic...), make([]byte, 64<<20)...) // 64 MiB of zeros compress ~1000:1
	blob := container(1, 100, bomb)
	if len(blob) > 1<<20 {
		t.Fatalf("test setup: bomb should be small on disk, got %d bytes", len(blob))
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, err := Decode(blob)
	runtime.ReadMemStats(&after)
	if err == nil || !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("want size mismatch error, got %v", err)
	}
	if grown := after.TotalAlloc - before.TotalAlloc; grown > 8<<20 {
		t.Fatalf("decoder allocated %d bytes for a bomb; must stay bounded by the header size", grown)
	}
}

func TestDecodeHeaderOverCapRejectedBeforeDecompress(t *testing.T) {
	withLimit(t, 1<<20)
	blob := container(1, 2<<20, sqlitePayload(64))
	_, err := Decode(blob)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("want 'exceeds' error, got %v", err)
	}
}

func TestDecodeOversizeFileRejected(t *testing.T) {
	withLimit(t, 1<<10)
	if _, err := Decode(sqlitePayload(4 << 10)); err == nil || !strings.Contains(err.Error(), "over the") {
		t.Fatalf("plain SQLite over the cap must be rejected, got %v", err)
	}
}

func TestDecodeTruncatedStream(t *testing.T) {
	blob, _ := Encode(sqlitePayload(1 << 20))
	if _, err := Decode(blob[:len(blob)/2]); err == nil {
		t.Fatal("truncated stream must fail")
	}
}

func TestDecodeSizeMismatchBothDirections(t *testing.T) {
	p := sqlitePayload(4096)
	if _, err := Decode(container(1, 4095, p)); err == nil || !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("header smaller than payload: %v", err)
	}
	if _, err := Decode(container(1, 4097, p)); err == nil || !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("header larger than payload: %v", err)
	}
}

func TestDecodeWrongFlag(t *testing.T) {
	if _, err := Decode(container(2, 4096, sqlitePayload(4096))); err == nil || !strings.Contains(err.Error(), "unknown container flag") {
		t.Fatalf("got %v", err)
	}
}

func TestDecodePayloadNotSQLite(t *testing.T) {
	p := bytes.Repeat([]byte("x"), 4096)
	if _, err := Decode(container(1, 4096, p)); err == nil || !strings.Contains(err.Error(), "not a SQLite") {
		t.Fatalf("got %v", err)
	}
}

func TestEncodeRefusesNonSQLite(t *testing.T) {
	if _, err := Encode([]byte("not sqlite")); err == nil {
		t.Fatal("must refuse")
	}
}

func FuzzDecode(f *testing.F) {
	good, _ := Encode(sqlitePayload(2048))
	f.Add(good)
	f.Add(good[:len(good)/2])
	f.Add(container(1, 100, sqlitePayload(4096)))
	f.Add([]byte{})
	f.Add(sqliteMagic)
	f.Fuzz(func(t *testing.T, b []byte) {
		out, err := Decode(b)
		if err != nil {
			return
		}
		if !bytes.HasPrefix(out, sqliteMagic) {
			t.Fatalf("Decode succeeded on a payload without the SQLite magic")
		}
		if int64(len(out)) > maxDecoded {
			t.Fatalf("Decode returned %d bytes, over the cap", len(out))
		}
	})
}

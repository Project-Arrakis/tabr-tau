package web

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"
)

// The map pictures of the Saved map (#758): every picture in static/maps/ is listed in the art manifest (mapArt) with the size, the picture size and the hash it has, every listed picture is
// embedded, the notice names the pictures and says which maps have none, and a map without a picture is one the manifest says has none. The route list that serves the pictures is held by
// TestEveryEmbeddedAssetIsServedWithItsOwnBytes (a picture embedded without a route fails there). The tie between the manifest and the map configurations (Image empty or not) is in
// internal/ops/livemap_hub_test.go.

type mapArtFile struct {
	Role   string `json:"role"`
	Map    string `json:"map"`
	File   string `json:"file"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Source string `json:"source"`
}

func readMapArt(t *testing.T) (files []mapArtFile, without []string) {
	t.Helper()
	b, err := os.ReadFile(lccManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		MapArt struct {
			RuntimeRoot    string       `json:"runtimeRoot"`
			Files          []mapArtFile `json:"files"`
			WithoutPicture []string     `json:"withoutPicture"`
		} `json:"mapArt"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("art-manifest.json: %v", err)
	}
	if m.MapArt.RuntimeRoot != "internal/web/static/maps/" {
		t.Errorf("mapArt runtimeRoot %q, want internal/web/static/maps/", m.MapArt.RuntimeRoot)
	}
	return m.MapArt.Files, m.MapArt.WithoutPicture
}

func TestMapPicturesMatchTheArtManifestAndTheNotice(t *testing.T) {
	files, without := readMapArt(t)
	want := map[string]mapArtFile{}
	for _, f := range files {
		if f.Role != "map" || f.Map == "" || f.Source == "" || f.Width <= 0 || f.Height <= 0 || strings.Contains(f.File, "/") {
			t.Errorf("mapArt entry %+v: needs role map, the map, a file name in the folder, a source and the picture size", f)
		}
		if _, dup := want[f.File]; dup {
			t.Errorf("mapArt lists %s twice", f.File)
		}
		want[f.File] = f
	}
	entries, err := fs.ReadDir(static, "static/maps")
	if err != nil {
		t.Fatal(err)
	}
	var embedded []string
	for _, e := range entries {
		if e.IsDir() { // icons/ is the marker icons, listed by the notice and held by TestEveryEmbeddedAssetIsServedWithItsOwnBytes
			continue
		}
		embedded = append(embedded, e.Name())
		if _, ok := want[e.Name()]; !ok {
			t.Errorf("static/maps/%s is embedded but not in the art manifest (mapArt)", e.Name())
		}
	}
	sort.Strings(embedded)
	notice, err := os.ReadFile("../../docs/art-notice.md")
	if err != nil {
		t.Fatal(err)
	}
	for file, f := range want {
		b, err := static.ReadFile("static/maps/" + file)
		if err != nil {
			t.Errorf("%s is in the manifest but not embedded: %v", file, err)
			continue
		}
		sum := sha256.Sum256(b)
		if len(b) != f.Bytes || hex.EncodeToString(sum[:]) != f.SHA256 {
			t.Errorf("%s: %d bytes / hash %s, manifest says %d / %s", file, len(b), hex.EncodeToString(sum[:])[:12], f.Bytes, f.SHA256[:12])
		}
		// a PNG header with the picture size the manifest records (the picture is scaled to the map space of the map's configuration, so the size is the picture's own, not the map space)
		if len(b) < 24 || string(b[1:4]) != "PNG" || int(binary.BigEndian.Uint32(b[16:20])) != f.Width || int(binary.BigEndian.Uint32(b[20:24])) != f.Height {
			t.Errorf("%s is not a PNG of the recorded %dx%d size", file, f.Width, f.Height)
		}
		if !strings.Contains(string(notice), file) {
			t.Errorf("docs/art-notice.md does not name %s", file)
		}
	}
	// a map without a picture is said so by the manifest, and the notice says that the hubs have none while the manifest says they have none
	sort.Strings(without)
	if strings.Join(without, ",") != "Arrakeen,HarkoVillage" {
		t.Errorf("withoutPicture is %v: Arrakeen and Harko Village have no picture yet; when one is added, move the map from withoutPicture to files, list the picture and extend the notice", without)
	}
	if len(without) > 0 && !strings.Contains(string(notice), "no picture of either hub") {
		t.Error("docs/art-notice.md does not say that the hubs have no picture")
	}
}

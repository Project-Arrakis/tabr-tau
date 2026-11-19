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
// embedded, the notice names the pictures and says where the hub pictures come from, and a map without a picture is one the manifest says has none. The route list that serves the pictures is held by
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
	// every map of the Saved map has a picture now (the two hubs came with #758's pictures): the manifest lists no map without one
	if len(without) > 0 {
		t.Errorf("withoutPicture is %v: every map has a picture; a map added without one is listed there and drawn as an outline", without)
	}
	// the notice says where the two hub pictures come from, plainly: the companion app's tiles, not Funcom's render, shipped on the owner's decision, not affiliated, removable on request
	for _, must := range []string{"map tiles of the gaming.tools companion app", "not** from the same source", "shipped on the decision of the project owner", "not affiliated with gaming.tools", "open an issue on this repository and they will be taken out"} {
		if !strings.Contains(string(notice), must) {
			t.Errorf("docs/art-notice.md does not say %q about the hub pictures", must)
		}
	}
	for _, f := range []string{"arrakeen.png", "harko-village.png"} {
		if !strings.Contains(want[f].Source, "gaming.tools companion") || strings.Contains(want[f].Source, "Dune Docker console map picture") {
			t.Errorf("%s: the manifest source must name the gaming.tools companion tiles, got %q", f, want[f].Source)
		}
	}
}

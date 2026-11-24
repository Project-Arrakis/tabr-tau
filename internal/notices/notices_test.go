package notices

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The datasets taken from the game's own item tables have a notice (audit GRC-6).
func TestGameTableDatasetsHaveANotice(t *testing.T) {
	for _, must := range []string{"## Stack sizes and cosmetic unlock ids", "internal/catalog/stacks.json", "internal/catalog/customization_unlocks.json", "does not record"} {
		if !strings.Contains(Text, must) {
			t.Errorf("expected %q in the notices", must)
		}
	}
}

// Every file of data taken from the game that is embedded in the executable is named in the notices (audit 2026-11-13, D11-3).
// The list is checked against the disk, so a game-derived file added without an entry has to be added to this list and to the notices.
func TestEveryEmbeddedGameDataFileHasANotice(t *testing.T) {
	for _, file := range []string{
		"internal/catalog/stacks.json",
		"internal/catalog/volumes.json",
		"internal/catalog/customization_unlocks.json",
		"internal/catalog/buildcosts.json",
		"internal/ops/decree-names.json",
		"internal/layoutroll/worldlayouts-line.txt",
		"kit/gamedata/vehicles/vehicles.json",
		"kit/gamedata/regions/hagga-regions.json",
		"kit/gamedata/fillables/item_table_fillables.json",
		"internal/ops/place-names.json",
		"internal/ops/place-arrivals.json",
		"internal/ops/deep-desert-taxis.json",
		"internal/ops/landsraad_fixed_places.go",
		"kit/schema/schema.sql",
		"kit/schema/schema_map_names.sql",
	} {
		if _, err := os.Stat("../../" + file); err != nil {
			t.Errorf("%s is listed here as embedded game data but is not on disk: %v", file, err)
		}
		if !strings.Contains(Text, file) {
			t.Errorf("%s holds data taken from the game but THIRD-PARTY-NOTICES.md does not name it", file)
		}
	}
	for _, must := range []string{"DT_StaticLocationData", "ST_Localization_Dialogue", "DeepDesert_1_Travel", "DT_ItemTableFillables", "select sql from sqlite_master", "DefaultGame.ini", "not affiliated with or endorsed by Funcom"} {
		if !strings.Contains(Text, must) {
			t.Errorf("expected %q in the notices", must)
		}
	}
}

// Every data file that a package embeds in the executable (a //go:embed line naming one JSON, SQL or text file) is named in the notices by its path.
// The explicit list above says which of them come from the game; this one walks the tree, so an embedded data file that is added to any package without
// an entry in the notices fails here (audit D11-3: the check must not depend on someone remembering to extend a list).
func TestEveryEmbeddedDataFileIsNamedInTheNotices(t *testing.T) {
	embed := regexp.MustCompile(`(?m)^//go:embed\s+(.+)$`)
	root := "../.."
	seen := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if n := d.Name(); n == ".git" || n == "node_modules" || n == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range embed.FindAllStringSubmatch(string(src), -1) {
			for _, pat := range strings.Fields(m[1]) {
				switch path.Ext(pat) {
				case ".json", ".sql", ".txt":
				default:
					continue // folders and picture or page sets are covered by their own notices
				}
				if strings.ContainsAny(pat, "*?[") {
					t.Errorf("%s embeds %q with a pattern; name the data files one by one so each one needs an entry", p, pat)
					continue
				}
				rel, err := filepath.Rel(root, filepath.Join(filepath.Dir(p), pat))
				if err != nil {
					return err
				}
				rel = filepath.ToSlash(rel)
				seen++
				if !strings.Contains(Text, rel) {
					t.Errorf("%s is embedded in the executable but THIRD-PARTY-NOTICES.md does not name it", rel)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen < 14 {
		t.Fatalf("found only %d embedded data files; the walk or the pattern is wrong", seen)
	}
}

// Every module the project depends on must have its licence reproduced under a heading with the version go.mod requires, so adding a dependency without
// its notice, or bumping one without refreshing the heading, fails here (audit D11-4).
func TestEveryModuleHasItsLicenceText(t *testing.T) {
	gomod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?m)^\s+([a-z0-9.\-]+\.[a-z]+/\S+)\s+(v\S+)`)
	text := strings.ReplaceAll(Text, "\r\n", "\n")
	found := 0
	for _, m := range re.FindAllStringSubmatch(string(gomod), -1) {
		// The nested modules of this repository (kit) are the project's own code under its own LICENSE, not a third party's.
		if strings.HasPrefix(m[1], "github.com/Project-Arrakis/tabr-tau/") {
			continue
		}
		found++
		heading := "\n## " + m[1] + " " + m[2] + "\n"
		if !strings.Contains(text, heading) {
			t.Errorf("go.mod requires %s %s but THIRD-PARTY-NOTICES.md has no licence section headed %q (a bumped module needs its heading and its licence text refreshed)", m[1], m[2], strings.TrimSpace(heading))
		}
	}
	if found < 5 {
		t.Fatalf("parsed only %d modules from go.mod; the pattern is wrong", found)
	}
	for _, must := range []string{"MIT License", "ISC License", "Redistribution and use in source and binary forms", "Microsoft WebView2 loader", "Copyright (c) 2026 RedBlink"} {
		if !strings.Contains(Text, must) {
			t.Errorf("expected licence text %q in the notices", must)
		}
	}
}

// The item catalog notice names the commits and blobs the file was copied from (#405).
func TestItemCatalogNoticeNamesTheRefresh(t *testing.T) {
	for _, must := range []string{"82548d72", "ecffc834", "v1.4.52", "MTX_IxianTotem_Patent", "1bde840c", "a9d50345", "739ecaef", "v1.4.54"} {
		if !strings.Contains(Text, must) {
			t.Errorf("expected %q in the item catalog notice", must)
		}
	}
}

package notices

import (
	"os"
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
		"kit/gamedata/vehicles/vehicles.json",
		"kit/gamedata/regions/hagga-regions.json",
		"kit/gamedata/fillables/item_table_fillables.json",
		"internal/ops/place-names.json",
		"internal/ops/place-arrivals.json",
		"internal/ops/deep-desert-taxis.json",
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
	for _, must := range []string{"DT_StaticLocationData", "ST_Localization_Dialogue", "DeepDesert_1_Travel", "DT_ItemTableFillables", "select sql from sqlite_master", "not affiliated with or endorsed by Funcom"} {
		if !strings.Contains(Text, must) {
			t.Errorf("expected %q in the notices", must)
		}
	}
}

// Every module the project depends on must have its licence reproduced, so adding a dependency without its notice
// fails here.
func TestEveryModuleHasItsLicenceText(t *testing.T) {
	gomod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?m)^\s+([a-z0-9.\-]+\.[a-z]+/\S+)\s+v`)
	found := 0
	for _, m := range re.FindAllStringSubmatch(string(gomod), -1) {
		// The nested modules of this repository (kit) are the project's own code under its own LICENSE, not a third party's.
		if strings.HasPrefix(m[1], "github.com/Project-Arrakis/tabr-tau/") {
			continue
		}
		found++
		if !strings.Contains(Text, "## "+m[1]+" ") {
			t.Errorf("go.mod requires %s but THIRD-PARTY-NOTICES.md has no licence section for it", m[1])
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

// The dye palettes of the design module name the public page their colours were read from (#683), and the notices say where the question of the wiki's licence stands: a
// permission line or "Not yet confirmed". The line cannot be dropped without this test failing, so a merge never hides an open question.
func TestDyePaletteNoticeNamesItsSourcePageAndTheStateOfTheLicenceQuestion(t *testing.T) {
	for _, must := range []string{"https://awakening.wiki/Module:Swatches/data", "https://awakening.wiki/Swatches", "House dyes", "Outfit dyes", "Creative Commons Attribution-NonCommercial-ShareAlike 4.0", "https://creativecommons.org/licenses/by-nc-sa/4.0/"} {
		if !strings.Contains(Text, must) {
			t.Errorf("expected %q in the notices", must)
		}
	}
	if !strings.Contains(Text, "Permission: ") && !strings.Contains(Text, "Not yet confirmed with the wiki's owners") {
		t.Error("the notices name neither the permission of the wiki's owners nor that it is not yet confirmed")
	}
	for _, mustNot := range []string{"No text, table layout, image or code of the wiki", "this module's own work", "which this module does not use"} {
		if strings.Contains(Text, mustNot) {
			t.Errorf("the notices still say %q", mustNot)
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

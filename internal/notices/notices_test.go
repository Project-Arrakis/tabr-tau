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

// The item catalog notice names the commits and blobs the file was copied from (#405).
func TestItemCatalogNoticeNamesTheRefresh(t *testing.T) {
	for _, must := range []string{"82548d72", "ecffc834", "v1.4.52", "MTX_IxianTotem_Patent", "1bde840c"} {
		if !strings.Contains(Text, must) {
			t.Errorf("expected %q in the item catalog notice", must)
		}
	}
}

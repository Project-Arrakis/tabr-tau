package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type obj = map[string]any

func write(t *testing.T, dir, name string, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const (
	pinnacleLoc = "09D3DED34D48F0AB2994DE853D7B2FCF"
	anvilLoc    = "2445CEFC45F16149BFD5D78C80841113"
	desertLoc   = "6F72F943466BF7C8FC1D3E88853764D5"
	caveLoc     = "FE08706A4E495CEB3A6E4D8C12FF1CBB"
)

func pkg(object string) string {
	return "/Script/DuneSandbox.DuneStaticMarkerActor /Game/Dune/Maps/X/" + object + "." + object + ":PersistentLevel.DuneStaticMarkerActor_0"
}

func inputs(t *testing.T, rows obj, text obj) Inputs {
	dir := t.TempDir()
	return Inputs{
		Locations: write(t, dir, "loc.json", []obj{{"Rows": rows}}),
		Dialogue:  write(t, dir, "dlg.json", []obj{{"StringTable": obj{"KeysToEntries": text}}}),
		Build:     "b", Read: "2026-10-30",
	}
}

func fullText() obj {
	return obj{
		"DIALOGUE/DIALOGUE_OPTION_D6TNQ": "Pinnacle Station Tradepost.",
		"DIALOGUE/DIALOGUE_OPTION_LFRWB": "Crossroads Tradepost.",
		"DIALOGUE/DIALOGUE_OPTION_X4D2Y": "Griffin's Reach Tradepost.",
		"DIALOGUE/DIALOGUE_OPTION_WI65H": "The Anvil Tradepost.",
	}
}

func TestGenerateNamesTheTradingPostRowsOnly(t *testing.T) {
	rows := obj{
		pinnacleLoc:                        obj{"m_PackageName": pkg("CB_Arrakis_Generic_M_05_PinnacleStation_MarkerName"), "m_StaticLocationTags": []string{tradePostTag}},
		anvilLoc:                           obj{"m_PackageName": pkg("CB_Trading_Post_MirzabahsShadow_Gameplay"), "m_StaticLocationTags": []string{tradePostTag}},
		desertLoc:                          obj{"m_PackageName": pkg("DeepDesert_1_Travel"), "m_StaticLocationTags": []string{tradePostTag}},
		caveLoc:                            obj{"m_PackageName": pkg("SH_Arrakeen_Gameplay"), "m_StaticLocationTags": []string{"Exploration.Cave"}},
		"5A8421CC4BB33ECB861EC98845A9C6E8": obj{"m_PackageName": pkg("CB_Trading_Post_MirzabahsShadow_Gameplay"), "m_StaticLocationTags": []string{}}, // same object, not a trading post
		"TradingPost":                      obj{"m_PackageName": "None", "m_StaticLocationTags": []string{tradePostTag}},                              // the generic tag row
	}
	f, err := Generate(inputs(t, rows, fullText()))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Places) != 3 {
		t.Fatalf("the cave, the untagged row and the generic row are not trading posts: %+v", f.Places)
	}
	if p := f.Places[pinnacleLoc]; p.Name != "Pinnacle Station Tradepost" || p.Kind != "tradepost" || p.Derived {
		t.Errorf("read from the game's text, without the full stop: %+v", p)
	}
	if p := f.Places[anvilLoc]; p.Name != "The Anvil Tradepost" || p.Alias != "Mirzabah's Shadow" || p.Derived {
		t.Errorf("the post of the Mirzabah's Shadow level is The Anvil: %+v", p)
	}
	if p := f.Places[desertLoc]; p.Name != "Deep Desert Tradepost" || !p.Derived {
		t.Errorf("a name that is not in the game's text is marked derived: %+v", p)
	}
	if f.Build != "b" || f.Read != "2026-10-30" || len(f.Tables) != 2 {
		t.Errorf("provenance: %+v", f)
	}
}

func TestGenerateRefusesWhatItCannotNameInsteadOfGuessing(t *testing.T) {
	post := func(object string) obj {
		return obj{pinnacleLoc: obj{"m_PackageName": pkg(object), "m_StaticLocationTags": []string{tradePostTag}}}
	}
	if _, err := Generate(inputs(t, post("CB_Some_New_Tradepost"), fullText())); err == nil || !strings.Contains(err.Error(), "CB_Some_New_Tradepost") {
		t.Errorf("a trading post of an object the rule has no line for must be an error that names it: %v", err)
	}
	text := fullText()
	delete(text, "DIALOGUE/DIALOGUE_OPTION_D6TNQ")
	if _, err := Generate(inputs(t, post("CB_Arrakis_Generic_M_05_PinnacleStation_MarkerName"), text)); err == nil || !strings.Contains(err.Error(), "DIALOGUE_OPTION_D6TNQ") {
		t.Errorf("a missing line of the game's text must be an error: %v", err)
	}
	if _, err := Generate(inputs(t, obj{caveLoc: obj{"m_PackageName": "x", "m_StaticLocationTags": []string{"Exploration.Cave"}}}, fullText())); err == nil {
		t.Error("a dump without a trading post is the wrong dump")
	}
}

func TestObjectOf(t *testing.T) {
	for in, want := range map[string]string{
		pkg("CB_A_B"):                  "CB_A_B",
		"None":                         "None",
		"/x/y/Z.Z:PersistentLevel.A_0": "Z",
	} {
		if got := objectOf(in); got != want {
			t.Errorf("objectOf(%q) = %q, want %q", in, got, want)
		}
	}
}

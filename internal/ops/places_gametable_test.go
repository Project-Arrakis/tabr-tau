package ops

import (
	"sort"
	"strings"
	"testing"
)

// gameTradePostKeys are the rows of the game's table DT_StaticLocationData that carry the trading-post tag (Exploration.POI.TradePost) and have a 32-digit row name, as read on
// 2026-11-16 (the game's own table, read-only; the generic row named "TradingPost" has no map object and is not a place). The list is kept here so that the test runs without the
// game's files: it is the checked-in record of which keys the game table had, and the keys the editor must name (#869). 11 keys are trading posts, 3 belong to the Deep Desert's
// travel level and are its taxi service points (#516).
//
// When the game adds a trading post, the file place-names.json is made again from the new table (internal/catalog/PROVENANCE.md, "place-names.json") and the key is added here;
// until then a key the file does not have still shows as "Unnamed trading post" (the fallback is kept for exactly that case).
var gameTradePostKeys = map[string]string{
	// Pinnacle Station
	"09D3DED34D48F0AB2994DE853D7B2FCF": "tradepost",
	"293E8B214EB4B7FAA31540911ABC0BAF": "tradepost",
	"84AAE0014040BCD05E16849A140E4CF5": "tradepost",
	// Crossroads
	"00271320416D180DAAF847BB767C31A6": "tradepost",
	"6FD413DB48DC86C9095C9FAF56A2BB0E": "tradepost",
	// Griffin's Reach
	"1546D0AC43DF1AADFA14E5A2C5C56D19": "tradepost",
	// The Anvil (the level of Mirzabah's Shadow)
	"04562129415514A156EB829659631E33": "tradepost",
	"2445CEFC45F16149BFD5D78C80841113": "tradepost",
	"C0090F88458324408E2EDB9F455F5111": "tradepost",
	"E3A969F649C10BBA658A759FF9140847": "tradepost",
	"F084ECF848AE48243928868FBB99E03E": "tradepost",
	// Deep Desert taxi service points
	"5B1E5FA645B571DF9714B2B2F8EDD06B": kindTaxi,
	"6F72F943466BF7C8FC1D3E88853764D5": kindTaxi,
	"747ED59B48C052C7577385A5C8DEC0C1": kindTaxi,
}

// Every trading-post key of the game's table has its real name in place-names.json, so no trading post of a save can show as "Unnamed trading post" while the game's table is the
// one recorded above (#869). The names are read from the game's own text, never made up: a key with no name fails here instead of being given a guess.
func TestEveryGameTradePostKeyHasAName(t *testing.T) {
	for key, kind := range gameTradePostKeys {
		p, ok := placeNames[key]
		if !ok {
			t.Errorf("%s: the game's table has this trading-post key and place-names.json does not name it", key)
			continue
		}
		if strings.TrimSpace(p.Name) == "" {
			t.Errorf("%s: the name is blank", key)
		}
		if p.Kind != kind {
			t.Errorf("%s: kind %q, want %q", key, p.Kind, kind)
		}
		display, _ := tradePostDisplay(p.Name, key, 1)
		if display == unnamedTradePost || display != p.Name {
			t.Errorf("%s: the page would show %q for a post named %q", key, display, p.Name)
		}
	}
}

// The file and the record of the game's table agree both ways: a key the file has that the record does not is a key of a newer table (or a mistake), and the record must be
// brought up to date with it so the test above keeps covering every key.
func TestPlaceNamesKeysAreExactlyTheGameTableKeys(t *testing.T) {
	var extra []string
	for key := range placeNames {
		if _, ok := gameTradePostKeys[key]; !ok {
			extra = append(extra, key)
		}
	}
	sort.Strings(extra)
	if len(extra) > 0 {
		t.Errorf("place-names.json has keys that gameTradePostKeys (the game's table) does not: %v", extra)
	}
	if len(placeNames) != len(gameTradePostKeys) {
		t.Errorf("place-names.json has %d keys, the game's table has %d", len(placeNames), len(gameTradePostKeys))
	}
}

// The fallback stays: a trading post whose key is neither in the file nor in the game's table the editor knows (a post a later game build adds) is still listed, as
// "Unnamed trading post", with its key in the tooltip.
func TestAKeyOfALaterGameBuildStillGetsTheFallback(t *testing.T) {
	const later = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"
	if _, ok := gameTradePostKeys[later]; ok {
		t.Fatal("test key is in the game table record")
	}
	if _, ok := placeNames[later]; ok {
		t.Fatal("test key is named")
	}
	display, detail := tradePostDisplay("TradingPost (key "+later+")", later, 7)
	if display != unnamedTradePost || detail != "Location key "+later {
		t.Errorf("display %q detail %q", display, detail)
	}
}

package ops

import (
	_ "embed"
	"encoding/json"
)

// The names players know for the Landsraad decrees (#608). The save holds only the game's internal id of a decree (landsraad_decrees.decree_name); the name and the effect line the
// game shows are in its own table DT_LandsraadDecrees, copied once into decree-names.json (internal/catalog/PROVENANCE.md, "decree-names.json"). The Landsraad read adds three
// fields to every decree row: display_name (the game's name, with the variant in brackets where two decrees share one name), effect (the game's one-line effect, when it has one)
// and name_known (false when the name is not the game's: an id that is not in the file, and the plain SpecialVendorActive row, which has no row in the game's table). A decree id
// the file does not have is still returned, with name_known false and no display_name, and the page then shows a readable form of the id.

//go:embed decree-names.json
var rawDecreeNames []byte

// decreeName is what decree-names.json says about one decree id.
type decreeName struct {
	Name    string `json:"name"`
	Variant string `json:"variant"` // told apart in brackets where two decrees share one name (In Honor of the Fallen: Ranged and Melee)
	Effect  string `json:"effect"`
	Derived bool   `json:"derived"` // the editor's own words: the game's table has no row for this id
}

// decreeNames is the decree id (the save's decree_name) -> its name and effect.
var decreeNames = func() map[string]decreeName {
	var f struct {
		Decrees map[string]decreeName `json:"decrees"`
	}
	if err := json.Unmarshal(rawDecreeNames, &f); err != nil {
		panic("ops: decree-names.json is not valid: " + err.Error())
	}
	return f.Decrees
}()

// decreeDisplay returns the player-facing name of a decree id, its effect line, and whether the name is the game's own.
func decreeDisplay(id string) (name, effect string, known bool) {
	d, ok := decreeNames[id]
	if !ok || d.Name == "" {
		return "", "", false
	}
	name = d.Name
	if d.Variant != "" {
		name += " (" + d.Variant + ")"
	}
	return name, d.Effect, !d.Derived
}

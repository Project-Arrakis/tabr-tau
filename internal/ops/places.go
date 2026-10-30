package ops

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// Named places (#500). Admin > Movement > Go to a place lists four kinds of place by the names a player sees: trading posts, the character's own bases, the
// character's own vehicles and the taxi hubs. Nothing else on the saved map (outposts, camps, resources, hazards, caves, vendors) is offered.
//
// A trading post's name is not in the save. The save's marker has only the generic type "TradingPost", and its payload starts with a LocationKey (32 hexadecimal digits),
// which is the name of a row in the game's own table DT_StaticLocationData. place-names.json says which key is which place; tools/placenamesgen makes it from the game's tables
// (internal/catalog/PROVENANCE.md, "place-names.json"). A trading post whose key the file does not have is listed with its type and key, not hidden.
//
// The personal bases are the character's own totems (a permission record that gives the character the owner rank), at the claim position, with the name the character gave
// them. The map's HomeBase markers are not used: the TotemId in their payload does not say which base a marker is (the same id sits on several markers, and most ids are not a
// base in the save), so a marker cannot be tied to the character. The vehicles are the character's own that stand in the world (a stored or lost vehicle has no place).
// A taxi hub is a TaxiService marker, which sits on the map of its hub (Arrakeen, Harko Village), so it is offered while the list is for that map.

//go:embed place-names.json
var rawPlaceNames []byte

// namedPlace is what place-names.json says about one LocationKey.
type namedPlace struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Alias string `json:"alias"`
}

// placeNames is LocationKey (upper case) -> the place.
var placeNames = func() map[string]namedPlace {
	var f struct {
		Places map[string]namedPlace `json:"places"`
	}
	if err := json.Unmarshal(rawPlaceNames, &f); err != nil {
		panic("ops: place-names.json is not valid: " + err.Error())
	}
	return f.Places
}()

// The groups of the places list, in the order the page shows them.
const (
	groupTradePost = "tradepost"
	groupBase      = "base"
	groupVehicle   = "vehicle"
	groupTaxi      = "taxi"
)

// DestinationGroup is one heading of the places list.
type DestinationGroup struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

var destinationGroups = []DestinationGroup{
	{groupTradePost, "Trading posts"},
	{groupBase, "Personal bases"},
	{groupVehicle, "Vehicles"},
	{groupTaxi, "Taxis"},
}

func groupIndex(key string) int {
	for i, g := range destinationGroups {
		if g.Key == key {
			return i
		}
	}
	return len(destinationGroups)
}

// payloadHead is how much of a marker's payload is read: the LocationKey is among the first bytes; the rest of a payload is not needed and is not read.
const payloadHead = 512

var locationKeyName = []byte("LocationKey")

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// markerLocationKey reads the LocationKey from the start of a marker's payload: the name "LocationKey", a short length prefix the game writes before the text, and 32
// hexadecimal digits. It returns the key in upper case, or "" when the payload has none.
func markerLocationKey(payload []byte) string {
	i := bytes.Index(payload, locationKeyName)
	if i < 0 {
		return ""
	}
	rest := payload[i+len(locationKeyName):]
	for s := 0; s <= 8 && s+32 <= len(rest); s++ {
		ok := true
		for _, c := range rest[s : s+32] {
			if !isHexDigit(c) {
				ok = false
				break
			}
		}
		if ok && (s == 0 || !isHexDigit(rest[s-1])) && (s+32 == len(rest) || !isHexDigit(rest[s+32])) {
			return strings.ToUpper(string(rest[s : s+32]))
		}
	}
	return ""
}

// payloadOf decodes the hex of the head of a payload (the page reads it with hex(substr(payload,1,payloadHead)), because a blob is not returned whole by the save reader).
func payloadOf(v any) []byte {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil
	}
	return b
}

// tradePostName is the label of a TradingPost marker and the other name the filter matches: the name of its key when the file has it, else its type and key (or, with no
// key in the payload, its type and the marker's number).
func tradePostName(markerType string, key string, markerID any) (label, alias string) {
	if p, ok := placeNames[key]; ok && key != "" && p.Name != "" {
		return p.Name, p.Alias
	}
	if key != "" {
		return fmt.Sprintf("%s (key %s)", markerType, key), ""
	}
	return fmt.Sprintf("%s (marker %v)", markerType, markerID), ""
}

// hubName is a hub's map name as words: "HarkoVillage" is "Harko Village".
func hubName(mapName string) string {
	var b strings.Builder
	for i, r := range mapName {
		if i > 0 && unicode.IsUpper(r) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// taxiName is the label of a TaxiService marker on the map named mapName; "Taxi Service" is the game's own text for that marker type.
func taxiName(mapName string) string { return hubName(mapName) + " Taxi Service" }

// baseLabel is the label of a base: the name the character gave it, or the game's name for its claim structure when it has none, and its number.
func baseLabel(id any, name, totemType string) string {
	if name == "" || name == "None" || strings.HasPrefix(name, "##") {
		name = totemTypes[strings.ToLower(totemType)]
	}
	if name == "" {
		name = "Base"
	}
	return fmt.Sprintf("%s #%v", name, id)
}

// vehicleLabel is the label of a vehicle: the name the character gave it, or its blueprint name, and its number.
func vehicleLabel(id, class any, name string) string {
	if name == "" || name == "None" || strings.HasPrefix(name, "##") {
		name = shortClass(class)
	}
	return fmt.Sprintf("%s #%v", name, id)
}

// distinguishNames tells apart places of one group that would be listed under the same name (a save can hold several keys of one trading post): each gets the start of its
// key after the name. A name that is alone is left as it is.
func distinguishNames(all []placeRow, group []int) {
	count := map[string]int{}
	for _, i := range group {
		count[all[i].label]++
	}
	for _, i := range group {
		if count[all[i].label] > 1 && len(all[i].key) >= 6 {
			all[i].label = fmt.Sprintf("%s (%s)", all[i].label, all[i].key[:6])
		}
	}
}

// placeRow is a place found in the save before it is filtered and given its distance.
type placeRow struct {
	group, label, alias, key string
	x, y, z                  any
}

// groupsOf are the headings that have a place in the list, in the order the page shows them.
func groupsOf(list []Destination) []DestinationGroup {
	out := []DestinationGroup{}
	for _, g := range destinationGroups {
		for _, d := range list {
			if d.Group == g.Key {
				out = append(out, g)
				break
			}
		}
	}
	return out
}

// namedPlaces finds the places of the four groups on the map mp for the character p. A trading post or taxi hub is a marker the character has discovered; a base or vehicle
// is the character's own. A place with no position in the save is left out by the caller. Places that share one name inside a group are told apart (distinguishNames).
func (o *Ops) namedPlaces(p player, mp string) ([]placeRow, error) {
	var out []placeRow

	marks, err := o.S.Query(`select m.marker_type t, m.marker_hash_id h, m.x x, m.y y, m.z z, hex(substr(m.payload,1,?)) p from markers m
		join player_markers pm on pm.marker_hash_id=m.marker_hash_id and pm.dimension_index=m.dimension_index and pm.map_name=m.map_name
		where pm.player_id=? and pm.discovery_level>0 and m.map_name=? and m.marker_type in ('TradingPost','TaxiService') order by m.marker_hash_id`, payloadHead, p.Controller, mp)
	if err != nil {
		return nil, err
	}
	for _, r := range marks {
		switch r["t"] {
		case "TradingPost":
			key := markerLocationKey(payloadOf(r["p"]))
			label, alias := tradePostName("TradingPost", key, r["h"])
			out = append(out, placeRow{group: groupTradePost, label: label, alias: alias, key: key, x: r["x"], y: r["y"], z: r["z"]})
		case "TaxiService":
			out = append(out, placeRow{group: groupTaxi, label: taxiName(mp), x: r["x"], y: r["y"], z: r["z"]})
		}
	}

	bases, err := o.S.Query(`select t.id id, t.landclaim_original_global_location_x x, t.landclaim_original_global_location_y y, t.landclaim_original_global_location_z z,
			coalesce(pa.actor_name,'') name, coalesce(tp.building_type,'') totem_type
		from totems t join actors a on a.id=t.id
		join permission_actor_rank r on r.permission_actor_id=t.id and r.player_id=? and r.rank=1
		left join permission_actor pa on pa.actor_id=t.id left join placeables tp on tp.id=t.id
		where a.map=? order by t.id`, p.Controller, mp)
	if err != nil {
		return nil, err
	}
	for _, r := range bases {
		name, _ := r["name"].(string)
		tt, _ := r["totem_type"].(string)
		out = append(out, placeRow{group: groupBase, label: baseLabel(r["id"], name, tt), x: r["x"], y: r["y"], z: r["z"]})
	}

	vehicles, err := o.S.Query(`select a.id id, a.class cls, a.location_x x, a.location_y y, a.location_z z, coalesce(pa.actor_name,'') name
		from vehicles v join actors a on a.id=v.id left join permission_actor pa on pa.actor_id=v.id
		where v.id in (`+ownedVehicleSQL+`) and v.id not in (`+storedVehicleSQL+`) and a.map=? order by a.id`, p.Controller, mp)
	if err != nil {
		return nil, err
	}
	for _, r := range vehicles {
		name, _ := r["name"].(string)
		out = append(out, placeRow{group: groupVehicle, label: vehicleLabel(r["id"], r["cls"], name), x: r["x"], y: r["y"], z: r["z"]})
	}

	for _, g := range destinationGroups {
		var rows []int
		for i, r := range out {
			if r.group == g.Key {
				rows = append(rows, i)
			}
		}
		distinguishNames(out, rows)
	}
	return out, nil
}

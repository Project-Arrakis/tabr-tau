package ops

import (
	"fmt"
	"strings"
)

// mapConfig is how a map's world coordinates become pixels on its picture. The numbers are the Dune Docker console's
// LIVE_MAP_CONFIGS (MIT): px = (x - MinX) / (MaxX - MinX) * Width, py = (y - MinY) / (MaxY - MinY) * Height, no flip.
// Width and Height are the map space the bounds refer to; the picture is scaled to it.
type mapConfig struct {
	Key    string  `json:"key"`
	Label  string  `json:"label"`
	Image  string  `json:"image"`
	Width  int     `json:"width"`
	Height int     `json:"height"`
	MinX   float64 `json:"minX"`
	MaxX   float64 `json:"maxX"`
	MinY   float64 `json:"minY"`
	MaxY   float64 `json:"maxY"`
}

var liveMaps = []mapConfig{
	{Key: "HaggaBasin", Label: "Hagga Basin", Image: "/maps/hagga-basin.png", Width: 4096, Height: 4096,
		MinX: -456752.21, MaxX: 354547.46, MinY: -450630.14, MaxY: 353821.95},
	// The Deep Desert rectangle is the in-game sector grid, squared up (console liveMapSector.js).
	{Key: "DeepDesert", Label: "The Deep Desert", Image: "/maps/deep-desert.png", Width: 4096, Height: 4096,
		MinX: -1268450, MaxX: 1158400, MinY: -1261434, MaxY: 1165416},
}

// storageTypes is the "Storage" group of containers: the plain storage containers the map shows as storage markers.
var storageTypes = containerGroups[0].Types

func findMap(key string) mapConfig {
	for _, m := range liveMaps {
		if m.Key == key {
			return m
		}
	}
	return liveMaps[0]
}

// LiveMap is the save's view of one map: the character, the player's own vehicles, bases, storage containers and the game's
// map markers (resources and points of interest, with whether the player has discovered them). It is the save as of its last
// write, not the running game.
func (o *Ops) LiveMap(key string) (any, error) {
	cfg := findMap(key)
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	maps := make([]map[string]any, 0, len(liveMaps))
	for _, m := range liveMaps {
		maps = append(maps, map[string]any{"key": m.Key, "label": m.Label})
	}
	out := map[string]any{"config": cfg, "maps": maps}

	out["character"] = nil // the character is only on the map it is on
	if ch, _ := o.S.One(`select ? name, location_x x, location_y y, location_z z from actors where id=? and map=?`, p.Name, p.Pawn, cfg.Key); len(ch) > 0 {
		out["character"] = ch
	}

	out["vehicles"], _ = o.S.Query(`select a.id, a.class, a.location_x x, a.location_y y from vehicles v join actors a on a.id=v.id
		where a.map=? and v.id in (`+ownedVehicleSQL+`) order by a.id`, cfg.Key, p.Controller)
	vehicles, _ := out["vehicles"].([]map[string]any)
	for _, v := range vehicles {
		v["name"] = shortClass(v["class"])
		delete(v, "class")
	}

	out["bases"], _ = o.S.Query(`select t.id, a.location_x x, a.location_y y from totems t join actors a on a.id=t.id where a.map=? order by t.id`, cfg.Key)

	types := make([]any, 0, len(storageTypes))
	marks := make([]string, 0, len(storageTypes))
	for t := range storageTypes {
		types = append(types, t)
		marks = append(marks, "?")
	}
	storage, err := o.S.Query(`select p.id, p.building_type, coalesce(pa.actor_name,'') custom_name, a.location_x x, a.location_y y
		from placeables p
		join actors a on a.id=p.id
		join actor_fgl_entities afe on afe.entity_id=p.owner_entity_id
		join totems t on t.id=afe.actor_id
		left join permission_actor pa on pa.actor_id=p.id
		where a.map=? and p.is_hologram=0 and lower(p.building_type) in (`+strings.Join(marks, ",")+`) order by p.id`, append([]any{cfg.Key}, types...)...)
	if err != nil {
		return nil, fmt.Errorf("storage on the map: %w", err)
	}
	for _, s := range storage {
		bt, _ := s["building_type"].(string)
		name, _ := s["custom_name"].(string)
		if name == "" || name == "None" || strings.HasPrefix(name, "##") {
			name = storageTypes[strings.ToLower(bt)]
		}
		s["name"] = name
		delete(s, "custom_name")
		delete(s, "building_type")
	}
	out["storage"] = storage

	// The game's own map markers; d is the discovery level for this character (0 when never discovered).
	out["markers"], _ = o.S.Query(`select m.marker_type t, m.x, m.y, coalesce(pm.discovery_level,0) d from markers m
		left join player_markers pm on pm.marker_hash_id=m.marker_hash_id and pm.dimension_index=m.dimension_index and pm.player_id=?
		where m.map_name=? and m.x is not null and m.y is not null order by m.marker_type, m.marker_hash_id`, p.Controller, cfg.Key)
	return out, nil
}

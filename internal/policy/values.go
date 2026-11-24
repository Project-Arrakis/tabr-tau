package policy

// The limits of the other values of a base file (#613, audit D3-1, G-021, G-045). The real figures seen are far below them; they are there to refuse the absurd, not to model the game.
// Each was run over the whole of a real save and refuses nothing the game wrote.
const (
	// MaxItemDurability is the most an item's durability figures (current, maximum, decayed maximum) may be (real gear: 100).
	MaxItemDurability = 1_000_000.0
	// MaxUnknownStack is the most of one item a file may put in a slot when the catalog does not know the item.
	MaxUnknownStack = 100_000
	// MaxSlot is the highest slot an item may be in when its inventory states no slot count, and the largest slot count an inventory may state (the largest the game's own saves hold
	// is 1,000).
	MaxSlot = 100_000
	// NoSlotCount is the value the game writes in max_item_count when no slot count is stated (as do 0 and NULL).
	NoSlotCount = -1
	// MaxVolume is the most an inventory's volume limit (max_item_volume) or an item's own volume (volume_override, per unit) may be. The largest figures the editor knows are a
	// developer storage container's 50,000 (docs/design/2026-10-28-cargo-limits.md) and a unit volume of 125 (the catalog's largest, a container vehicle); the ceiling is 20 times the
	// first. A negative volume is never one.
	MaxVolume = 1_000_000.0
)

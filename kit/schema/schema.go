// Package schema is the shape of a Dune: Awakening single-player save for a program that reads or writes one with SQL: the DDL of a real
// save, the names of the tables and columns that programs' SQL uses, and the pieces of SQL text that two programs write the same way.
//
// It is text and numbers only. It opens no database and imports no SQLite driver: a program loads DDL() into a database of its own
// choosing (a plain in-memory one in a test, say) and builds its queries from the names and fragments here. The DDL is the schema only
// (CREATE statements extracted from a save's sqlite_master); no row of any save is in the module.
//
// What is not here is a program's rule. Which vehicles a character owns and which vehicles are stored are questions that two programs
// answer with different SQL, because the rule is the program's; this package gives the names such a rule is written with.
package schema

import _ "embed"

//go:embed schema.sql
var ddl string

//go:embed schema_map_names.sql
var ddlMapNames string

// DDL is the schema of the game's single-player database as it was until the game's update of early November 2026: 94 tables, their indexes and
// the game's one trigger, as the CREATE statements the game wrote, in an order that runs as one script into an empty database. In this layout
// markers and player_markers carry the map as text (the column map_name). It is unchanged since v0.7.0; a save the current game writes has the
// layout of DDLMapNames. A program that reads saves of both tells them apart by the columns of markers (map_name or map_name_id), never by a
// game version.
func DDL() string { return ddl }

// DDLMapNames is the schema of a save written by the game after its update of early November 2026: 97 tables, their indexes and the game's one
// trigger, in the same shape as DDL. Against DDL: markers and player_markers have map_name_id (an integer) in place of map_name, and the names
// are in the new table map_names (map_name_id, map_name); landsraad_one_time_house_rewards and landsraad_player_one_time_house_rewards are new;
// and a few indexes were added or dropped. Every other table is defined as in DDL. Since v0.10.0.
func DDLMapNames() string { return ddlMapNames }

// Table and column names. Each variable is one table: Table is its name and the other fields are columns of it, so a query can be written
// as "select " + schema.Actors.State + " from " + schema.Actors.Table. A test checks every name against DDL(): a name that is not in the
// schema fails the build. Only the tables and columns that programs' SQL touches are listed; the DDL has the rest.
var (
	Actors = struct {
		Table, ID, Class, Map, LocationX, LocationY, LocationZ, DimensionIndex, GasAttributes, Properties, OwnerAccountID, State string
	}{"actors", "id", "class", "map", "location_x", "location_y", "location_z", "dimension_index", "gas_attributes", "properties", "owner_account_id", "state"}

	ActorFglEntities = struct{ Table, ActorID, EntityID, SlotName string }{"actor_fgl_entities", "actor_id", "entity_id", "slot_name"}

	BackupVehicles = struct{ Table, CharacterID, VehicleID string }{"backup_vehicles", "character_id", "vehicle_id"}

	FglEntities = struct{ Table, EntityID, Components string }{"fgl_entities", "entity_id", "components"}

	Inventories = struct {
		Table, ID, ActorID, ItemID, VehicleModuleID, InventoryType, MaxItemCount, MaxItemVolume string
	}{"inventories", "id", "actor_id", "item_id", "vehicle_module_id", "inventory_type", "max_item_count", "max_item_volume"}

	Items = struct{ Table, ID, InventoryID, StackSize, PositionIndex, TemplateID, Stats string }{"items", "id", "inventory_id", "stack_size", "position_index", "template_id", "stats"}

	PermissionActor = struct{ Table, ActorID, ActorType string }{"permission_actor", "actor_id", "actor_type"}

	PermissionActorRank = struct{ Table, PermissionActorID, PlayerID, Rank string }{"permission_actor_rank", "permission_actor_id", "player_id", "rank"}

	PlayerState = struct{ Table, ID, AccountID, CharacterName, PlayerPawnID string }{"player_state", "id", "account_id", "character_name", "player_pawn_id"}

	RecoveredVehicles = struct{ Table, CharacterID, VehicleID string }{"recovered_vehicles", "character_id", "vehicle_id"}

	VehicleModules = struct{ Table, ID, VehicleID, TemplateID, Stats string }{"vehicle_modules", "id", "vehicle_id", "template_id", "stats"}

	Vehicles = struct{ Table, ID string }{"vehicles", "id"}
)

// MapNames is the table that only the layout of DDLMapNames has (a test checks it against DDLMapNames()). There markers.map_name_id and
// player_markers.map_name_id are integers that refer to MapNames.MapNameID, and MapNames.MapName is the text that the tables of DDL carry
// directly as markers.map_name and player_markers.map_name.
var MapNames = struct{ Table, MapNameID, MapName string }{"map_names", "map_name_id", "map_name"}

// The JSON records of a save. An item's or a module's stats (and an entity's components) are JSON in which each record is a two-element
// array, [[], {fields}], and the fields are in element 1. A Key constant is the path of the whole record (null or missing when the record
// is absent); the constant without Key ends with "[1]." so that a field name can be appended in a query.
const (
	// VehicleModuleDurabilityKey and VehicleModuleDurability: the durability record of a vehicle module (vehicle_modules.stats).
	VehicleModuleDurabilityKey = "$.FVehicleModuleDurabilityStats"
	VehicleModuleDurability    = VehicleModuleDurabilityKey + "[1]."

	// ItemDurabilityKey and ItemDurability: the durability record of an item (items.stats): worn gear, tools, weapons.
	ItemDurabilityKey = "$.FItemStackAndDurabilityStats"
	ItemDurability    = ItemDurabilityKey + "[1]."

	// FillableKey and Fillable: the record of an item that holds a liquid (items.stats), such as a Literjon or a blood sack.
	FillableKey = "$.FFillableItemStats"
	Fillable    = FillableKey + "[1]."
)

// Field names inside those records. A save leaves CurrentDurability out when it is 0 and DecayedMaxDurability out when nothing has worn
// the maximum down.
const (
	CurrentDurability    = "CurrentDurability"
	DecayedMaxDurability = "DecayedMaxDurability"
	MaxDurability        = "MaxDurability"
	CurrentAmount        = "CurrentAmount"
	FillableType         = "FillableType"
)

// Whole paths, for a query that reads or writes one field.
const (
	VehicleModuleCurrentDurability    = VehicleModuleDurability + CurrentDurability
	VehicleModuleDecayedMaxDurability = VehicleModuleDurability + DecayedMaxDurability
	VehicleModuleMaxDurability        = VehicleModuleDurability + MaxDurability
	ItemCurrentDurability             = ItemDurability + CurrentDurability
	ItemDecayedMaxDurability          = ItemDurability + DecayedMaxDurability
	FillableCurrentAmount             = Fillable + CurrentAmount
	FillableLiquid                    = Fillable + FillableType
)

// The game's lifecycle state of a vehicle actor (actors.state, 0 for an ordinary actor). Seen in real saves: a vehicle in the character's
// vehicle backup has state 2 and a vehicle stored for recovery has state 4. The single-player file stores integers; no other state is
// interpreted here.
const (
	StateVehicleBackup   = 2
	StateVehicleRecovery = 4

	// StoredStates is the two states as the inside of an SQL "in (...)" list.
	StoredStates = "2, 4"
)

// GearInventoryTypes is the inside of an SQL "in (...)" list over inventories.inventory_type: the inventories that make up a character's
// own gear (the backpack, 0, and the worn and loadout inventories, 1 and 15).
const GearInventoryTypes = "0,1,15"

package ops

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Move the Reserve into named storage boxes (#1191). A recycle (#1176) puts everything into the character's Reserve (inventory type 33). The Reserve can only be filled by the game
// (a recycle), never by the player, so it is a one-way inbox: this edit empties it into two storage boxes the person has named, so that the next recycle starts with an empty Reserve.
//
//   - Which boxes: each configured name is looked up as the name the person gave the box in the game (permission_actor.actor_name of the box's placeable, the same name the Bases >
//     Inventory list shows). It must match exactly one Developer storage container of the save, and that one must stand in a base of the character's own (the owner rank, as a recycle
//     checks it), be in the normal state, not be a hologram, not be on the Deep Desert, and not be part of a base that is being recycled (MoveReserveOptions.ExcludeBases). A name that
//     matches none, or more than one, a box that is refused for one of those reasons, and the same box named twice all refuse the whole move, naming what was found. Nothing is guessed:
//     a box that has never been renamed ("##Developer_StorageContainer_Placeable") has no name to find it by.
//   - What moves: EVERY item row of the Reserve, each as it is. Only inventory_id and position_index of a row change (the same two columns a recycle changes when it moves what a base
//     stored); a row is never split, merged, topped up, recreated or deleted. The rows go in their order in the Reserve (position, then id), each into the first box that has both a
//     free slot and room by volume, into that box's lowest free position.
//   - Room, by the give's own rules (giveupto.go: freeSlotsInTx, inventoryVolumeLimitInTx, inventoryVolumeUsedInTx, unitVolume, stackFitsByVolume), applied to whole stacks. Every stack
//     takes one slot (cosmetics and uniques too). A box holds max_item_count stacks (the slots; a box with no count above 0 is refused although a give would call it unlimited: a Developer
//     storage container always holds 1,000 slots, so such a box is unknown, and a move cannot be undone in the game) and max_item_volume of volume (a limit
//     of 0 or none recorded means no limit, as a give reads it). Volume is each row's unit volume (its volume_override when it has one, else the game's item table, catalog.Volume)
//     times its stack. A row whose item has no known volume counts as none and the answer says how many units that was. When the Reserve does not fit, nothing is changed and the answer
//     says what was needed, what each box has free, and the shortfall.
//   - All or nothing: one edit, read back inside itself (every row is where it was planned, with every other column as it was; the Reserve is empty; no position is shared), then the
//     save layer's own checks. The write must be sent the fingerprint the preview gave. While a game process exists the edit is queued and put back onto the save the game wrote last;
//     it is dropped when the game changed anything the edit read.
//
// An empty Reserve is not an error and writes nothing.

// DefaultReserveBoxes are the names of the two boxes the operator uses: the editor and the command offer them when none are typed. A name is matched exactly, capitals included: the
// save keeps the name as it was typed in the game, and nothing is known of the game treating names as case-insensitive.
var DefaultReserveBoxes = []string{"DD-1", "DD-2"}

// MoveReserveOptions are the choices of one move.
type MoveReserveOptions struct {
	// ExcludeBases are the totems of the bases that are being recycled: a box in one of them is refused.
	ExcludeBases []int64
}

var errMoveReserveDry = errors.New("move reserve dry run")

// ErrMoveReserveNoRoom is the kind of the refusal of a Reserve that does not fit into the boxes. The refusal is also an apierr.ErrNoRoom. A caller that must tell "the boxes are
// full" from every other refusal (the multi-base run stops at it) asks IsMoveNoRoom, never reads the words.
var ErrMoveReserveNoRoom = errors.New("the Reserve does not fit into the boxes")

// IsMoveNoRoom says whether err is the refusal of a Reserve that does not fit (slots or volume).
func IsMoveNoRoom(err error) bool { return errors.Is(err, ErrMoveReserveNoRoom) }

// moveReserveBeforeReadBack is nil outside tests. A test sets it to change the save between the writes and the read back, to show that a difference fails the whole edit.
var moveReserveBeforeReadBack func(m *save.Mut)

// maxMoveReserveStacks bounds the rows one move writes. The Reserve of a few recycles holds a few thousand stacks.
const maxMoveReserveStacks = 20000

type reserveStack struct {
	ID       int64
	Template string
	Stack    int64
	Pos      int64
	Unit     float64 // the unit volume (unitVolume); 0 when the item has no known volume
	Volume   float64 // the stack's volume: the unit volume times the stack
	Known    bool    // false when the item has no known volume (counted as none, as a give counts it)
}

type moveBox struct {
	Name     string
	Actor    int64
	Inv      int64
	Base     int64
	BaseName string
	Map      string
	Cap      int64   // slots
	Free0    int64   // the free slots before the move (freeSlotsInTx, the give's count)
	VolLimit float64 // 0 = no limit recorded
	Used     map[int64]bool
	UsedVol  float64 // the volume already in the box (inventoryVolumeUsedInTx, the give's count)
	Unknown  int64   // units already there with no known volume
	Before   int64 // stacks in the box before the move
	// planned
	nextPos        int64
	Stacks, Units  int64
	Volume         float64
	FreeSlots      int64
	FreeVolume     float64 // -1 when the box has no volume limit
	slotsTaken     int64
	positionsTaken map[int64]bool
}

type reserveMove struct {
	Item int64
	Box  int // index into the plan's boxes
	Slot int64
}

type moveReservePlan struct {
	Reserve  int64
	Boxes    []*moveBox
	Stacks   []reserveStack
	Moves    []reserveMove
	Units    int64
	Volume   float64
	Unknown  int64 // units of the Reserve with no known volume
	Digest   string
	Warnings []string
	Dry      bool // a preview: the summary says what would be done
}

func (p *moveReservePlan) fingerprint() string {
	h := sha256.New()
	fmt.Fprintf(h, "movereserve|%d|%s|%d|%d|", p.Reserve, p.Digest, len(p.Stacks), p.Units)
	for i, b := range p.Boxes {
		fmt.Fprintf(h, "box%d:%d/%d/%d/%s/%d/%g/%d;", i, b.Actor, b.Inv, b.Base, b.Name, b.Cap, b.VolLimit, b.Before)
	}
	for _, mv := range p.Moves {
		fmt.Fprintf(h, "%d>%d@%d;", mv.Item, mv.Box, mv.Slot)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// negativeVolume says whether a row carries a volume of its own below 0, which no game row has: such a row would make the room of a box add up wrongly.
func negativeVolume(override any) bool {
	o, ok := override.(float64)
	return ok && o < 0
}

func fmtVol(v float64) string { return fmt.Sprintf("%.1f", v) }

// ownedByCharacter says whether the character holds the owner rank (1) over the totem and nobody else does: the check a recycle makes.
func ownedByCharacter(q baseQ, totem, controller int64) (bool, error) {
	ranks, err := q.Query(`select player_id, rank from permission_actor_rank where permission_actor_id=?`, totem)
	if err != nil {
		return false, err
	}
	mine, owners := false, 0
	for _, r := range ranks {
		pid, _ := toInt(r["player_id"])
		if rk, _ := toInt(r["rank"]); rk == 1 {
			owners++
			mine = mine || pid == controller
		}
	}
	return mine && owners == 1, nil
}

// resolveMoveBox finds the one Developer storage container the person named, or says why there is not exactly one usable.
func resolveMoveBox(q baseQ, pl player, name string, exclude map[int64]bool) (*moveBox, error) {
	if name == "" || name == "None" || strings.HasPrefix(name, "##") || len(name) > 200 {
		return nil, apierr.Newf(apierr.ErrBadArgument, "%q is not a name a box can be found by (a box that was never renamed has none). Name the two boxes in the game first. Nothing was changed.", name)
	}
	rows, err := q.Query(`select p.id actor_id, coalesce(a.map,'') map, a.state state, p.is_hologram holo,
		(select afe.actor_id from actor_fgl_entities afe join totems t on t.id=afe.actor_id where afe.entity_id=p.owner_entity_id and afe.slot_name='Actor' order by afe.actor_id limit 1) base_id,
		exists (select 1 from base_backup_linked_actors where actor_id=p.id) linked
		from placeables p join actors a on a.id=p.id join permission_actor pa on pa.actor_id=p.id
		where lower(p.building_type)=? and trim(pa.actor_name, ' '||char(9,10,13))=? order by p.id`, strings.ToLower(devStorageType), name)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, apierr.Newf(apierr.ErrNotFound, "No Developer storage container in this save is named %q. Names are matched exactly, capitals included. %s Name the box in the game and let the game save, or check the spelling. Nothing was changed.", name, namesRead(q))
	}
	if len(rows) > 1 {
		var where []string
		for _, r := range rows {
			id, _ := toInt(r["actor_id"])
			mp, _ := r["map"].(string)
			if mp == "" {
				mp = "no map"
			}
			where = append(where, fmt.Sprintf("number %d on %s", id, mp))
		}
		return nil, apierr.Newf(apierr.ErrConflict, "%d Developer storage containers are named %q (%s). The name must find exactly one box: rename the others in the game. Nothing was changed.", len(rows), name, strings.Join(where, "; "))
	}
	r := rows[0]
	id, _ := toInt(r["actor_id"])
	mp, _ := r["map"].(string)
	state, _ := toInt(r["state"])
	holo, _ := toInt(r["holo"])
	linked, _ := toInt(r["linked"])
	base, hasBase := toInt(r["base_id"])
	refuse := func(why string) (*moveBox, error) {
		return nil, apierr.Newf(apierr.ErrConflict, "The box %q (number %d) %s, so it cannot receive the Reserve. Nothing was changed.", name, id, why)
	}
	tt := map[string]any{}
	if hasBase {
		var err error
		if tt, err = q.One(`select coalesce(a.map,'') map, a.state state, exists (select 1 from base_backup_linked_actors where actor_id=a.id) linked from actors a where a.id=?`, base); err != nil {
			return nil, err
		}
		if tt == nil {
			tt = map[string]any{"map": "", "state": int64(0), "linked": int64(0)}
		}
	}
	tmap, _ := tt["map"].(string)
	tstate, _ := toInt(tt["state"])
	tlinked, _ := toInt(tt["linked"])
	switch {
	case isDeepDesertMap(mp):
		return refuse("is on the Deep Desert")
	case strings.TrimSpace(mp) == "":
		return refuse("has no map recorded, so it is not known whether it is on the Deep Desert")
	case holo != 0:
		return refuse("is only a hologram (placed but not built)")
	case state != 0 || linked != 0:
		return refuse("is not in the normal state (it is part of a base that was picked up, or of a base backup)")
	case !hasBase:
		return refuse("does not stand in a base of yours (its owner is not a base's totem)")
	case exclude[base]:
		return refuse(fmt.Sprintf("stands in base %d, which is being recycled", base))
	case isDeepDesertMap(tmap):
		return refuse(fmt.Sprintf("stands in base %d, which is on the Deep Desert", base))
	case strings.TrimSpace(tmap) == "":
		return refuse(fmt.Sprintf("stands in base %d, which has no map recorded, so it is not known whether it is on the Deep Desert", base))
	case tstate != 0 || tlinked != 0:
		return refuse(fmt.Sprintf("stands in base %d, which is not in the normal state (it was picked up, or it is part of a base backup)", base))
	}
	if mine, err := ownedByCharacter(q, base, pl.Controller); err != nil {
		return nil, err
	} else if !mine {
		return refuse(fmt.Sprintf("stands in base %d, which is not yours (the character does not hold its owner rank alone)", base))
	}
	invs, err := q.Query(`select id, max_item_count cap from inventories where actor_id=? and item_id is null order by id`, id)
	if err != nil {
		return nil, err
	}
	if len(invs) != 1 {
		return refuse(fmt.Sprintf("has %d inventories, not one", len(invs)))
	}
	b := &moveBox{Name: name, Actor: id, Base: base, Map: mp, Used: map[int64]bool{}, positionsTaken: map[int64]bool{}}
	b.Inv, _ = toInt(invs[0]["id"])
	b.Cap, _ = toInt(invs[0]["cap"])
	// The slots are counted by the give's own rule (freeSlotsInTx). One deliberate difference: a give treats a count of -1, 0 or none as "no limit", but this move refuses such a box,
	// because every Developer storage container in the saves seen holds 1,000 slots, so a box without a count is unknown, and a move cannot be undone in the game.
	if b.Free0, err = freeSlotsInTx(q, b.Inv); err != nil {
		return nil, err
	}
	if b.Free0 < 0 {
		return refuse("has no slot count above 0 recorded (this version does not know what such a box is)")
	}
	if b.VolLimit, err = inventoryVolumeLimitInTx(q, b.Inv); err != nil {
		return nil, err
	}
	if bn, err := q.One(`select coalesce(pa.actor_name,'') name, coalesce(tp.building_type,'') totem_type from totems t left join permission_actor pa on pa.actor_id=t.id left join placeables tp on tp.id=t.id where t.id=?`, base); err == nil && bn != nil {
		rn, _ := bn["name"].(string)
		tt, _ := bn["totem_type"].(string)
		b.BaseName = backupDisplayName(rn, tt)
	}
	// what is in the box now
	have, err := q.Query(`select position_index p, volume_override o from items where inventory_id=?`, b.Inv)
	if err != nil {
		return nil, err
	}
	var shared, beyond, negative int64
	for _, h := range have {
		if p, ok := toInt(h["p"]); ok {
			if b.Used[p] {
				shared++
			}
			if p < 0 || p >= b.Cap {
				beyond++
			}
			b.Used[p] = true
		}
		if negativeVolume(h["o"]) {
			negative++
		}
	}
	if shared > 0 || beyond > 0 {
		return refuse(fmt.Sprintf("has items in places the game would not use (%d stacks share a slot with another, %d lie beyond its %d slots), so it was not touched; put its contents in order in the game first", shared, beyond, b.Cap))
	}
	if negative > 0 {
		return refuse(fmt.Sprintf("holds %d stacks with a volume of their own below 0, so its room cannot be worked out", negative))
	}
	b.Before = int64(len(have))
	if b.UsedVol, b.Unknown, err = inventoryVolumeUsedInTx(q, b.Inv); err != nil {
		return nil, err
	}
	return b, nil
}

// namesRead lists the names the editor read from the save for every Developer storage container (the same list the Bases > Inventory tab shows), so that a refusal says what to look
// for: "Developer storage containers named in this save: "DD-1" (1), "Dump Box" (1); 9 were never renamed."
func namesRead(q baseQ) string {
	rows, err := q.Query(`select coalesce(pa.actor_name,'') name, count(*) n from placeables p left join permission_actor pa on pa.actor_id=p.id
		where lower(p.building_type)=? group by 1 order by 1`, strings.ToLower(devStorageType))
	if err != nil {
		return "(The names in the save could not be listed.)"
	}
	var named []string
	var unnamed int64
	for _, r := range rows {
		n, _ := r["name"].(string)
		c, _ := toInt(r["n"])
		if n == "" || n == "None" || strings.HasPrefix(n, "##") {
			unnamed += c
			continue
		}
		if len(named) < 30 {
			named = append(named, fmt.Sprintf("%q (%d)", importLabel(n), c))
		}
	}
	out := "The save has no renamed Developer storage container."
	if len(named) > 0 {
		out = "Developer storage containers named in this save: " + strings.Join(named, ", ") + "."
	}
	if unnamed > 0 {
		out += fmt.Sprintf(" %d were never renamed.", unnamed)
	}
	return out
}

// freePos is the lowest free position of the box that has not been given out yet, or false when there is none.
func (b *moveBox) freePos() (int64, bool) {
	if b.slotsTaken >= b.Free0 {
		return 0, false
	}
	for b.nextPos < b.Cap && (b.Used[b.nextPos] || b.positionsTaken[b.nextPos]) {
		b.nextPos++
	}
	if b.nextPos >= b.Cap {
		return 0, false
	}
	return b.nextPos, true
}

// volumeFits says whether a whole stack of n units of unit volume still fits by volume next to what is in the box and what the move has already put there: the give's own count,
// applied to the whole stack (stackFitsByVolume).
func (b *moveBox) volumeFits(unit float64, n int64) bool {
	return stackFitsByVolume(b.VolLimit, b.UsedVol+b.Volume, unit, n)
}

// digestColumns are every column of an item row except its place (inventory_id and position_index): what a move must leave exactly as it was. A test holds this list against the
// save's own items table, so a column the game adds later makes the test fail instead of going unchecked.
var digestColumns = []string{"id", "template_id", "stack_size", "is_new", "acquisition_time", "stats", "quality_level", "volume_override"}

func contentDigest(rows []save.Row) string {
	h := sha256.New()
	for _, r := range rows {
		for _, c := range digestColumns {
			fmt.Fprintf(h, "%v|", r[c])
		}
		fmt.Fprint(h, ";")
	}
	return hex.EncodeToString(h.Sum(nil))
}

const moveItemColumns = `id, template_id, stack_size, position_index, is_new, acquisition_time, stats, quality_level, volume_override`

func planMoveReserve(q baseQ, pl player, names []string, opt MoveReserveOptions) (*moveReservePlan, error) {
	if len(names) == 0 {
		return nil, apierr.New(apierr.ErrBadArgument, "Name the storage boxes first (one or more names). Nothing was changed.")
	}
	seen := map[string]bool{}
	clean := make([]string, len(names))
	for i, n := range names {
		clean[i] = strings.TrimSpace(n)
		k := strings.ToLower(clean[i])
		if seen[k] && k != "" {
			return nil, apierr.Newf(apierr.ErrBadArgument, "The box %q is named twice. Each name must be a different box. Nothing was changed.", clean[i])
		}
		seen[k] = true
	}
	exclude := map[int64]bool{}
	for _, b := range opt.ExcludeBases {
		exclude[b] = true
	}
	p := &moveReservePlan{}
	for _, n := range clean {
		b, err := resolveMoveBox(q, pl, n, exclude)
		if err != nil {
			return nil, err
		}
		for _, o := range p.Boxes {
			if o.Actor == b.Actor {
				return nil, apierr.Newf(apierr.ErrBadArgument, "The names %q and %q find the same box (number %d). Each name must be a different box. Nothing was changed.", o.Name, b.Name, b.Actor)
			}
		}
		p.Boxes = append(p.Boxes, b)
	}

	res, err := q.Query(`select id from inventories where actor_id=? and inventory_type=? and item_id is null`, pl.Pawn, reserveInventoryType)
	if err != nil {
		return nil, err
	}
	if len(res) != 1 {
		return nil, apierr.Newf(apierr.ErrConflict, "The character has %d Reserve inventories in this save, not one, so it is not certain what to move. Nothing was changed.", len(res))
	}
	p.Reserve, _ = toInt(res[0]["id"])
	for _, b := range p.Boxes {
		if b.Inv == p.Reserve {
			return nil, apierr.Newf(apierr.ErrConflict, "The box %q is the Reserve itself. Nothing was changed.", b.Name)
		}
	}
	rows, err := q.Query(`select `+moveItemColumns+` from items where inventory_id=? order by position_index, id`, p.Reserve)
	if err != nil {
		return nil, err
	}
	if len(rows) > maxMoveReserveStacks {
		return nil, apierr.Newf(apierr.ErrConflict, "The Reserve holds %s stacks, more than one move writes (%s). Nothing was changed.", commas(int64(len(rows))), commas(maxMoveReserveStacks))
	}
	p.Digest = contentDigest(rows)
	for _, r := range rows {
		s := reserveStack{}
		s.ID, _ = toInt(r["id"])
		s.Template, _ = r["template_id"].(string)
		s.Stack, _ = toInt(r["stack_size"])
		s.Pos, _ = toInt(r["position_index"])
		if negativeVolume(r["volume_override"]) {
			return nil, apierr.Newf(apierr.ErrConflict, "The Reserve holds a stack (item %d) with a volume of its own below 0, so the room in the boxes cannot be worked out. Nothing was changed.", s.ID)
		}
		if u, ok := unitVolume(s.Template, r["volume_override"]); ok {
			s.Unit, s.Volume, s.Known = u, u*float64(s.Stack), true
		} else {
			p.Unknown += s.Stack // counts as none, as in a give
		}
		p.Units += s.Stack
		p.Volume += s.Volume
		p.Stacks = append(p.Stacks, s)
	}
	if len(p.Stacks) == 0 {
		return p, nil
	}
	// first fit: each row, in the Reserve's order, goes into the first box with a free slot (the give's count) and room by volume (the give's count, for the whole stack). Units with
	// no known volume (cosmetics, uniques, placeables with no entry in the item table) count as none by volume and take one slot per stack, as in a give.
	var left []reserveStack
	for _, s := range p.Stacks {
		placed := false
		for i, b := range p.Boxes {
			if !b.volumeFits(s.Unit, s.Stack) {
				continue
			}
			pos, ok := b.freePos()
			if !ok {
				continue
			}
			b.positionsTaken[pos] = true
			b.slotsTaken++
			b.Stacks++
			b.Units += s.Stack
			b.Volume += s.Volume
			p.Moves = append(p.Moves, reserveMove{Item: s.ID, Box: i, Slot: pos})
			placed = true
			break
		}
		if !placed {
			left = append(left, s)
		}
	}
	if len(left) > 0 {
		return nil, moveShortfall(p, left)
	}
	for _, b := range p.Boxes {
		b.FreeSlots = b.Free0 - b.Stacks
		b.FreeVolume = -1
		if b.VolLimit > 0 {
			b.FreeVolume = b.VolLimit - b.UsedVol - b.Volume
		}
		if b.Unknown > 0 {
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s units already in the box %q have no known volume and were counted as none.", commas(b.Unknown), b.Name))
		}
	}
	if p.Unknown > 0 {
		p.Warnings = append(p.Warnings, fmt.Sprintf("%s units in the Reserve have no known volume and were counted as none (each stack still takes a slot), as Give item counts them.", commas(p.Unknown)))
	}
	return p, nil
}

// moveShortfall is the refusal of a Reserve that does not fit: what is needed, what each box has free, and what is short.
func moveShortfall(p *moveReservePlan, left []reserveStack) error {
	var lv float64
	for _, s := range left {
		lv += s.Volume
	}
	var parts []string
	var freeSlots int64
	for _, b := range p.Boxes {
		fs := b.Free0
		freeSlots += fs
		fv := "no volume limit"
		if b.VolLimit > 0 {
			fv = fmt.Sprintf("%s volume free", fmtVol(b.VolLimit-b.UsedVol))
		}
		parts = append(parts, fmt.Sprintf("%q has %s slots free and %s", b.Name, commas(fs), fv))
	}
	return apierr.Wrap(apierr.ErrNoRoom, fmt.Sprintf("The Reserve does not fit into the boxes, so nothing was changed. Needed: %s stacks (%s items, volume %s). Free: %s (%s slots in all). Short by %s stacks (volume %s) that fit in neither box. Free room in the boxes in the game, or name boxes with more room.",
		commas(int64(len(p.Stacks))), commas(p.Units), fmtVol(p.Volume), strings.Join(parts, "; "), commas(freeSlots), commas(int64(len(left))), fmtVol(lv)), ErrMoveReserveNoRoom)
}

// PreviewMoveReserve says what moving the Reserve into the named boxes would do and the fingerprint the write must be sent. It does the whole edit, reads it back, runs the save
// layer's two checks on it and rolls it back: nothing in the save changes.
func (o *Ops) PreviewMoveReserve(boxNames []string, opt MoveReserveOptions) (any, error) {
	return o.moveReserve(boxNames, "", true, opt)
}

// MoveReserve moves every stack of the character's Reserve into the named boxes. fingerprint is the one the preview gave.
func (o *Ops) MoveReserve(boxNames []string, fingerprint string, opt MoveReserveOptions) (any, error) {
	if fingerprint == "" {
		return nil, apierr.New(apierr.ErrBadArgument, "fingerprint is missing: the Reserve is moved as the question showed it")
	}
	return o.moveReserve(boxNames, fingerprint, false, opt)
}

func (o *Ops) moveReserve(names []string, want string, dry bool, opt MoveReserveOptions) (any, error) {
	pl, err := o.player()
	if err != nil || pl.Controller <= 0 || pl.Pawn <= 0 {
		return nil, apierr.New(apierr.ErrNotFound, "This save has no character with a Reserve, so nothing was changed.")
	}
	var res map[string]any
	_, err = o.S.Mutate("Move the Reserve into storage", func(m *save.Mut) error {
		plan, err := planMoveReserve(m, pl, names, opt)
		if err != nil {
			return err
		}
		if want != "" && want != plan.fingerprint() {
			return apierr.New(apierr.ErrConflict, "The Reserve or the boxes have changed since the question was shown. Nothing was changed. Ask again.")
		}
		plan.Dry = dry
		r, err := applyMoveReserve(m, plan)
		if err != nil {
			return err
		}
		res = r
		res["dryRun"] = dry
		if len(plan.Stacks) > 0 {
			m.Desc = fmt.Sprintf("Move the Reserve into storage: %d item stacks into %s", len(plan.Stacks), boxNamesWords(plan))
		}
		if dry {
			if err := m.CheckReferences(); err != nil {
				return apierr.Newf(apierr.ErrConflict, "The Reserve cannot be moved by this version: %v. Report this so the case can be covered.", err)
			}
			if err := m.CheckJSON(); err != nil {
				return apierr.Newf(apierr.ErrConflict, "The Reserve cannot be moved by this version: %v. Report this so the case can be covered.", err)
			}
			return errMoveReserveDry
		}
		return nil
	})
	if dry && errors.Is(err, errMoveReserveDry) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

func boxNamesWords(p *moveReservePlan) string {
	var n []string
	for _, b := range p.Boxes {
		n = append(n, `"`+importLabel(b.Name)+`"`)
	}
	return strings.Join(n, " and ")
}

func moveMismatch(what string, got, want int64) error {
	return apierr.Newf(apierr.ErrConflict, "The move was not written: %s read back as %d, and the plan was %d. Nothing was changed.", what, got, want)
}

func applyMoveReserve(m *save.Mut, p *moveReservePlan) (map[string]any, error) {
	boxOut := func() []map[string]any {
		out := []map[string]any{}
		for _, b := range p.Boxes {
			e := map[string]any{"name": b.Name, "actor": b.Actor, "inventory": b.Inv, "base": b.Base, "baseName": b.BaseName, "map": b.Map,
				"slots": b.Cap, "stacksBefore": b.Before, "stacks": b.Stacks, "items": b.Units, "volume": b.Volume, "volumeLimit": b.VolLimit}
			out = append(out, e)
		}
		return out
	}
	if len(p.Stacks) == 0 {
		return map[string]any{"ok": true, "changed": false, "nothingToDo": true, "stacks": 0, "items": 0, "boxes": boxOut(), "reserveInventory": p.Reserve,
			"summary": []string{"The Reserve is empty. Nothing was moved and nothing was written."}, "warnings": []string{}, "fingerprint": p.fingerprint()}, nil
	}

	// 1. the rows move: only inventory_id and position_index change
	for _, mv := range p.Moves {
		b := p.Boxes[mv.Box]
		r, err := m.Exec(`update items set inventory_id=?, position_index=? where id=? and inventory_id=?`, b.Inv, mv.Slot, mv.Item, p.Reserve)
		if err != nil {
			return nil, err
		}
		if n, _ := r.RowsAffected(); n != 1 {
			return nil, moveMismatch("an item stack that was moved", n, 1)
		}
	}
	if moveReserveBeforeReadBack != nil {
		moveReserveBeforeReadBack(m)
	}

	// 2. read back: the Reserve is empty, every row is in the box and slot that were planned, no other column of any row changed, no position is shared, no box is over its limits
	left, err := m.One(`select count(*) n from items where inventory_id=?`, p.Reserve)
	if err != nil {
		return nil, err
	}
	if n, _ := toInt(left["n"]); n != 0 {
		return nil, moveMismatch("the stacks left in the Reserve", n, 0)
	}
	ids := make([]int64, 0, len(p.Moves))
	where := map[int64]reserveMove{}
	for _, mv := range p.Moves {
		ids = append(ids, mv.Item)
		where[mv.Item] = mv
	}
	rows, err := m.Query(`select inventory_id, ` + moveItemColumns + ` from items where id in (` + idList(ids) + `)`)
	if err != nil {
		return nil, err
	}
	if int64(len(rows)) != int64(len(p.Moves)) {
		return nil, moveMismatch("the moved item stacks", int64(len(rows)), int64(len(p.Moves)))
	}
	// the digest was taken in the Reserve's order (position, then id), which the moves keep as their own order
	byID := map[int64]save.Row{}
	for _, r := range rows {
		id, _ := toInt(r["id"])
		byID[id] = r
		mv, ok := where[id]
		inv, _ := toInt(r["inventory_id"])
		pos, _ := toInt(r["position_index"])
		if !ok || inv != p.Boxes[mv.Box].Inv || pos != mv.Slot {
			return nil, apierr.Newf(apierr.ErrConflict, "The move was not written: item stack %d is not where it was planned. Nothing was changed.", id)
		}
	}
	ordered := make([]save.Row, 0, len(p.Stacks))
	for _, s := range p.Stacks {
		r, ok := byID[s.ID]
		if !ok {
			return nil, moveMismatch("a moved item stack", 0, 1)
		}
		cp := save.Row{}
		for k, v := range r {
			cp[k] = v
		}
		ordered = append(ordered, cp)
	}
	if got := contentDigest(ordered); got != p.Digest {
		return nil, apierr.New(apierr.ErrConflict, "The move was not written: a column of the moved item stacks other than their place changed. Nothing was changed.")
	}
	for i, b := range p.Boxes {
		c, err := m.One(`select count(*) n, count(distinct position_index) d, coalesce(min(position_index),0) lo, coalesce(max(position_index),0) hi from items where inventory_id=?`, b.Inv)
		if err != nil {
			return nil, err
		}
		n, _ := toInt(c["n"])
		d, _ := toInt(c["d"])
		hi, _ := toInt(c["hi"])
		lo, _ := toInt(c["lo"])
		if n != b.Before+b.Stacks {
			return nil, moveMismatch(fmt.Sprintf("the stacks in the box %q", b.Name), n, b.Before+b.Stacks)
		}
		if d != n {
			return nil, moveMismatch(fmt.Sprintf("the stacks of the box %q that share a position", b.Name), n-d, 0)
		}
		if n > b.Cap || hi >= b.Cap || lo < 0 {
			return nil, apierr.Newf(apierr.ErrConflict, "The move was not written: the box %q (box %d) would be over its %d slots. Nothing was changed.", b.Name, i+1, b.Cap)
		}
	}

	// 3. the answer, in words
	moved, in, was, afterwards := "Moved", "in", "were", "afterwards"
	if p.Dry {
		moved, in, was, afterwards = "Would move", "would go in", "are", "afterwards there would be"
	}
	summary := []string{fmt.Sprintf("%s: %s item stacks (%s items) from the Reserve into storage, whole and unchanged. Only the place of each stack changes.", moved, commas(int64(len(p.Stacks))), commas(p.Units))}
	for _, b := range p.Boxes {
		free := fmt.Sprintf("%s slots free", commas(b.FreeSlots))
		if b.FreeVolume >= 0 {
			free += fmt.Sprintf(", volume %s free", fmtVol(b.FreeVolume))
		}
		where := b.BaseName
		if where == "" {
			where = fmt.Sprintf("base %d", b.Base)
		}
		summary = append(summary, fmt.Sprintf("Box %q (in %q): %s stacks (%s items) %s, %s stacks %s already there; %s %s.", b.Name, importLabel(where), commas(b.Stacks), commas(b.Units), in, commas(b.Before), was, afterwards, free))
	}
	if p.Dry {
		summary = append(summary, "The Reserve would be empty afterwards.")
	} else {
		summary = append(summary, "The Reserve is empty afterwards.")
	}
	return map[string]any{"ok": true, "changed": true, "nothingToDo": false, "stacks": len(p.Stacks), "items": p.Units, "volume": p.Volume, "boxes": boxOut(), "reserveInventory": p.Reserve,
		"summary": summary, "warnings": p.Warnings, "fingerprint": p.fingerprint()}, nil
}

package layoutroll

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The opt-in step of the Deep Desert guard that recycles the player's Deep Desert bases (#1167, step 4), switched on with `layout-roll --recycle-bases`. Without the flag none of this
// runs and a standing base still refuses the roll, as before.
//
// What it does, in this order; every step must succeed or the run stops, and everything before the one write of the save leaves the save file as it was:
//
//	1. (the guard before it) the game is closed, a checkpoint of the save is taken and verified by its hash, the save is read;
//	2. stop, changing nothing, when the character is in the Deep Desert, when something could not be read, when ANY base that is not the player's own stands in the Deep Desert
//	   (owned by someone else, shared, owned by nobody), or when a base's map is not recorded;
//	3. every base is exported to a console base backup file, and each file is read back;
//	4. every base is asked about on the save as it is (all refusals other than lack of room are listed together and refuse the whole run), then the bases are recycled ONE AT A TIME
//	   in the order of their numbers, each as its own recycle with the Solido option, on the working copy of the save: each is asked about again on the copy as the bases before it left
//	   it, recycled, read back and verified (the base gone, the Solido filled and linked, the Reserve holding what it should) before the next one starts. The Reserve must be EMPTY
//	   before each base (a recycle sends everything to the Reserve, and the operator's rule is that no further recycle happens until it is empty): after each base the whole Reserve is
//	   moved into the storage boxes DD-1, DD-2 ... DD-6, as many as run in a row from DD-1 (recyclemove.go, #1191, #1214), by the editor's own move on the same working copy, with every base of the run left out of the boxes; a
//	   Reserve an earlier run left is moved before the first base. The Solido devices need places in the backpack or the loadout. The run recycles the largest number k of bases, in
//	   order, for which all of that holds: a box that is missing, misnamed or in a base of the run gives 0 of N before anything is changed (the boxes are asked about first, even with an empty Reserve); no room in the boxes stops it like a lack of room for a base. After the last base the Reserve is moved too, when
//	   the boxes can take it; when they cannot, the roll goes on (nothing is left to hold back) and the run says that the Reserve is still full;
//	5. k = 0 refuses and changes nothing; otherwise the intent is written to the state file (a state file that cannot be written refuses the write), the save is written once and read
//	   back from the file (the bases gone, the stacks in the boxes, the Reserve as the working copy ended), and when k is less than the number of bases N the run stops before the roll
//	   and says what to do; only k = N goes on to the roll (and the Deep Desert is looked at again first: it must hold no base).
//
// Nothing reaches the file before the write, so a failure at any base or any move before it leaves the save exactly as it was. A run that stopped at k < N is safe to run again: the
// bases done are gone, each run takes its own checkpoint, and the next run starts with the remaining bases.

// spaceMargin is how many slots must remain free in the backpack or the loadout after the new Solido devices. The recycle itself only refuses when there is no room at all; this keeps
// a few slots free so the character is not left with a full backpack.
const spaceMargin = 2

// enforceEmptyReserve is the operator's rule that the Reserve must be empty before each base is recycled. It is a variable only so that a test can show the backpack and loadout room logic
// on its own, with several bases in one run; nothing else changes it.
var enforceEmptyReserve = true

// The reads and writes of this step, as variables only so that a test can make one fail. Nothing else changes them.
var (
	// emptyReserveInto moves the Reserve of the working copy into the storage boxes DD-1 ... (#1191, recyclemove.go). exclude are the totems of every base of this run (a box in one of them
	// is refused). With previewOnly it only asks what the move would do; otherwise it asks and then moves, on the same working copy. It is called after each base is recycled and
	// verified and before a base when the Reserve is not empty (what an earlier run left), and must leave the Reserve empty (the run checks that). A refusal that leaves the copy
	// as it was (no such box, no room) comes back as a cannotMove error: the run then stops like a run that is out of room. It is nil only in a test that shows the loop without it.
	emptyReserveInto = moveReserveIntoBoxes

	otherBases     = func(s *save.Save) ([]ops.HoldBase, error) { return (&ops.Ops{S: s}).OtherDeepDesertBases() }
	baseActorMaps  = func(s *save.Save, id int64) ([]string, error) { return (&ops.Ops{S: s}).BaseActorsOffDeepDesert(id) }
	roomOf         = func(s *save.Save) (ops.RecycleRoom, error) { return (&ops.Ops{S: s}).RecycleRoom() }
	previewRecycle = func(s *save.Save, id int64) (map[string]any, error) {
		v, err := (&ops.Ops{S: s}).PreviewBaseRecycleWith(id, ops.RecycleOptions{Solido: true})
		return asMap(v, err)
	}
	recycleOne = func(s *save.Save, id int64, fingerprint string) (map[string]any, error) {
		v, err := (&ops.Ops{S: s}).RecycleBaseWith(id, fingerprint, ops.RecycleOptions{Solido: true})
		return asMap(v, err)
	}
	verifyBaseHere = func(s *save.Save, totem, item, blueprint int64) error {
		return (&ops.Ops{S: s}).VerifyRecycledBase(totem, item, blueprint)
	}
	// verifyBasesWritten reads the file that was written, opened afresh: every recycled base gone, every Solido in place, exactly the bases that were not recycled still standing, the
	// Reserve holding what the working copy ended with (empty when the last move into the boxes was made), and every stack that was moved in a box and in no other place.
	verifyBasesWritten = func(path string, done []recycledRecord, remaining []ops.HoldBase, want writtenExpect) error {
		fresh, err := openSave(path)
		if err != nil {
			return err
		}
		defer fresh.Close()
		for _, d := range done {
			if err := verifyBaseHere(fresh, d.ID, d.SolidoItem, d.SolidoBlueprint); err != nil {
				return fmt.Errorf("base %d: %v", d.ID, err)
			}
		}
		h, err := inspectSave(fresh)
		if err != nil {
			return err
		}
		if len(h.Bases) != len(remaining) {
			return fmt.Errorf("%d base(s) of yours stand in the Deep Desert, and %d were expected", len(h.Bases), len(remaining))
		}
		for i, b := range h.Bases {
			if b.ID != remaining[i].ID {
				return fmt.Errorf("base %d stands in the Deep Desert, and base %d was expected", b.ID, remaining[i].ID)
			}
		}
		others, err := otherBases(fresh)
		if err != nil {
			return err
		}
		if len(others) > 0 {
			return fmt.Errorf("%d other base(s) stand in the Deep Desert", len(others))
		}
		room, err := roomOf(fresh)
		if err != nil {
			return err
		}
		if room.Reserve.Used != want.ReserveLeft {
			return fmt.Errorf("the Reserve holds %d stacks, and %d were planned", room.Reserve.Used, want.ReserveLeft)
		}
		return verifyMovedWritten(fresh, want)
	}
)

func asMap(v any, err error) (map[string]any, error) {
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("the answer of the recycle is not in the form expected (a bug)")
	}
	return m, nil
}

// refundItem is one item of a base's refund.
type refundItem struct {
	Item  string `json:"item"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// recycledRecord is what one base's recycle did, as kept in the state file.
type recycledRecord struct {
	ID              int64        `json:"id"`
	Name            string       `json:"name"`
	Map             string       `json:"map"`
	Pieces          int64        `json:"pieces"`
	Placeables      int64        `json:"placeables"`
	StoredStacks    int64        `json:"storedStacks"` // item stacks that were stored in the base and moved to the Reserve
	RefundStacks    int64        `json:"refundStacks"` // new stacks in the Reserve
	Refund          []refundItem `json:"refund"`
	SolidoItem      int64        `json:"solidoItem"`      // the device that holds the Solido
	SolidoBlueprint int64        `json:"solidoBlueprint"` // its blueprint number
	SolidoNew       bool         `json:"solidoNew"`       // a new device was made (false: the character's own blank one was used)
	SolidoWhere     string       `json:"solidoWhere,omitempty"`

	// what the recycle itself said about the base (its summary and warnings), for the run's output; not kept in the state file
	summary, warnings []string
}

func intOf(m map[string]any, k string) int64 {
	switch v := m[k].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	}
	return 0
}

// isDeepDesert says whether a map is exactly the Deep Desert (the save writes DeepDesert). The recycle deletes, so it asks for more than the roll's check does (which also counts a
// travel map and a map that is not recorded as the Deep Desert, so as to block): a base on any other name is not recycled.
func isDeepDesert(m string) bool { return strings.EqualFold(strings.TrimSpace(m), "DeepDesert") }

// recordOf takes what a recycle (or its question) answered and checks that it holds what this step relies on: the base, its Deep Desert map and a made Solido.
func recordOf(b ops.HoldBase, res map[string]any) (recycledRecord, error) {
	var d recycledRecord
	d.ID = intOf(res, "base")
	d.Map, _ = res["map"].(string)
	d.Name, _ = res["name"].(string)
	d.summary, _ = res["summary"].([]string)
	d.warnings, _ = res["warnings"].([]string)
	if d.ID != b.ID {
		return d, fmt.Errorf("the answer is for base %d, not %d", d.ID, b.ID)
	}
	if !isDeepDesert(d.Map) {
		return d, fmt.Errorf("the base is on the map %q, not the Deep Desert", d.Map)
	}
	d.Pieces, d.Placeables = intOf(res, "pieces"), intOf(res, "placeables")
	d.StoredStacks, d.RefundStacks = intOf(res, "itemStacksMoved"), intOf(res, "refundStacks")
	if lines, ok := res["refund"].([]ops.RefundLine); ok {
		for _, l := range lines {
			d.Refund = append(d.Refund, refundItem{Item: l.Item, Name: l.Name, Count: l.Count})
		}
	}
	sol, ok := res["solido"].(map[string]any)
	if !ok {
		return d, errors.New("no Solido was made for it")
	}
	d.SolidoItem, d.SolidoBlueprint = intOf(sol, "item"), intOf(sol, "blueprint")
	d.SolidoNew, _ = sol["itemCreated"].(bool)
	d.SolidoWhere, _ = sol["itemWhere"].(string)
	if d.SolidoItem <= 0 || d.SolidoBlueprint <= 0 {
		return d, errors.New("the Solido has no device or blueprint number")
	}
	return d, nil
}

func baseLabel(b ops.HoldBase) string { return fmt.Sprintf("base %d %q", b.ID, b.Name) }

// recycleOutcome is what the step did, for the guard around it.
type recycleOutcome struct {
	done    bool // the run must end now
	planned bool // a dry run planned the recycle of every base: they count as gone for the rest of the dry run
}

const reserveRule = "The Reserve must be empty before each base is recycled: a recycle sends everything (what the base stored and its refund) to the Reserve, and no further recycle can happen until the Reserve is empty. After each base (and before the first, when an earlier run left something in it) the run moves the whole Reserve into your storage boxes " + boxesWords + " (#1191); when a box is missing, misnamed or otherwise cannot be used, nothing is recycled, and when there is no room in them the run recycles the bases that fit and stops before the roll."

// stateWriter is how the step keeps the state file (the guard's own record of the run) up to date. The guard owns the state; these write it at once.
type stateWriter struct {
	// intent is called BEFORE the save is written: the checkpoint and the bases about to be recycled are on record first. A write of it that fails refuses the write of the save.
	intent func(outcome string) error
	// withdraw takes the intent back when the save was not written.
	withdraw func()
	// persist is called as soon as the save has been written, before anything is read back: a process that dies after that point still leaves the checkpoint and the bases that were
	// recycled on record. An error is returned so that the step can say it loudly.
	persist func(outcome string) error
}

// reserveCannotWords is the sentence for a Reserve that could not be moved into the boxes.
func reserveCannotWords(err error) string {
	if ops.IsMoveNoRoom(err) {
		return "it does not fit into the boxes " + boxesWords + ": " + err.Error()
	}
	return "it cannot be moved into the boxes " + boxesWords + ": " + err.Error()
}

// recycleStep is steps 2 to 5 of the list above.
func recycleStep(o Options, r *Result, rec *checkRecord, s *save.Save, h ops.DeepDesertHolds, stop func(string, error) (bool, error), sw stateWriter, history []string) (recycleOutcome, error) {
	halt := func(outcome string, format string, a ...any) (recycleOutcome, error) {
		msg := fmt.Sprintf(format, a...)
		rec.Blockers = append(rec.Blockers, msg)
		r.say("Not changed: %s", msg)
		_, e := stop(outcome, errors.New("not changed: "+msg))
		return recycleOutcome{done: true}, e
	}
	others, err := otherBases(s)
	if err != nil {
		return halt("not changed: the other bases could not be checked", "the save could not be checked for bases that are not yours (%v)", err)
	}
	if len(h.Bases) == 0 && len(others) == 0 {
		r.say("--recycle-bases: no base stands in the Deep Desert, so nothing is recycled.")
		return recycleOutcome{}, nil
	}
	// 2. a base that is not the player's own: the run stops, nothing is recycled
	if len(others) > 0 {
		var l []string
		for _, b := range others {
			l = append(l, fmt.Sprintf("%s (%s)", baseLabel(b), placeName(b.Map)))
		}
		return halt("not changed: a base that is not yours stands in the Deep Desert", "%d base(s) that are not yours stand in the Deep Desert: %s. --recycle-bases only recycles bases your own character owns, and the run does not go on with a base left there; nothing was recycled", len(others), strings.Join(l, "; "))
	}
	if h.Character != nil || len(h.Problems) > 0 {
		r.say("--recycle-bases: no base is recycled in this run, because the character is in the Deep Desert or something could not be read (below). Settle that first; nothing is changed.")
		return recycleOutcome{}, nil
	}
	bases := append([]ops.HoldBase(nil), h.Bases...)
	sort.Slice(bases, func(i, j int) bool { return bases[i].ID < bases[j].ID })
	for _, b := range bases {
		if !isDeepDesert(b.Map) {
			return halt("not changed: a base is not exactly on the Deep Desert", "%s is on the map %q, which is not exactly the Deep Desert (or the save does not name it), so it is not recycled; nothing was recycled", baseLabel(b), b.Map)
		}
		// the map of the totem alone is not enough: every actor of the base must be on the Deep Desert too
		off, err := baseActorMaps(s, b.ID)
		if err != nil {
			return halt("not changed: the actors of a base could not be checked", "the actors of %s could not be checked (%v); nothing was recycled", baseLabel(b), err)
		}
		if len(off) > 0 {
			return halt("not changed: a base has actors off the Deep Desert", "%s has actors that are not on the Deep Desert (%s), so it is not recycled; nothing was recycled", baseLabel(b), strings.Join(off, "; "))
		}
	}
	r.say("--recycle-bases: %d base(s) of yours stand in the Deep Desert, to be recycled one at a time in the order of their numbers: %s.", len(bases), labels(bases))
	if len(history) > 0 {
		r.say("Earlier runs that recycled bases kept their checkpoints (the earliest one is the way back to before the first of them): %s.", strings.Join(history, "; "))
	}

	// the vehicles are planned now (they are moved after the bases): a plan that cannot be made stops the run before a base is touched
	if len(h.Vehicles) > 0 {
		if _, err := planVehicles(s); err != nil {
			return halt("not changed: the vehicles could not be placed", "the vehicles of yours in the Deep Desert cannot be taken out (%v), so no base was recycled", err)
		}
	}

	room0, err := roomOf(s)
	if err != nil {
		return halt("not changed: the room could not be read", "the room in the Reserve, the backpack and the loadout could not be read (%v)", err)
	}
	// a character with no Reserve cannot receive a recycle: refused here, before anything is exported
	if !room0.Reserve.Present {
		return halt("not changed: the character has no Reserve", "the character has no Reserve to receive what a base returns, so no base is recycled and nothing was exported")
	}
	r.say("Rule: %s", reserveRule)
	r.say("Room now: %s", strings.Join(roomLines(room0), "; "))

	// 4a. every base is asked about on the save as it is (nothing is changed by asking); a refusal for any reason other than room refuses the whole run, and all of them are listed
	// together. The first base must also fit: when not even it does, the run ends here, before anything is exported, so that a refused run leaves no export folder behind.
	var refused []string
	var first recycledRecord
	firstWhy := ""
	var firstRoom error // the first base was refused for lack of room on the save as it is (asked again below when a Reserve an earlier run left can be moved first, #1201)
	for i, b := range bases {
		pv, err := previewRecycle(s, b.ID)
		var d recycledRecord
		if err == nil {
			d, err = recordOf(b, pv)
		}
		switch {
		case err == nil:
			if i == 0 {
				first = d
			}
		case ops.IsRoomRefusal(err):
			if i == 0 {
				if emptyReserveInto != nil && room0.Reserve.Used > 0 {
					firstRoom = err
				} else {
					firstWhy = "there is no room for it: " + err.Error()
				}
			}
		default:
			refused = append(refused, fmt.Sprintf("%s: %v", baseLabel(b), err))
		}
	}
	if len(refused) > 0 {
		return halt("not changed: a base cannot be recycled", "%d of %d base(s) cannot be recycled, so none was (all or nothing): %s", len(refused), len(bases), strings.Join(refused, " | "))
	}
	// The boxes are asked about now, on the untouched save, even when the Reserve is empty (the move looks for the boxes first): a box that is missing, misnamed, in a base of this run or
	// on the Deep Desert gives 0 of N with nothing changed and nothing exported, instead of a base recycled and the rest left. What an earlier run left in the Reserve is moved first, so it
	// is asked about the same way.
	if firstWhy == "" && emptyReserveInto != nil {
		pre, err := emptyReserveInto(s, holdIDs(bases), true)
		if err != nil {
			if !isCannotMove(err) {
				return halt("not changed: the move of the Reserve could not be asked about", "the move of the Reserve into the boxes could not be asked about (%v); nothing was recycled", err)
			}
			if room0.Reserve.Used > 0 {
				firstWhy = reserveWhy(0, room0.Reserve.Used) + ", and " + reserveCannotWords(err)
			} else {
				firstWhy = "the boxes " + boxesWords + " cannot be used for the Reserve (" + err.Error() + ")"
				if len(bases) > 1 {
					firstWhy += "; the Reserve is moved into them after every base, so no base is recycled"
				}
			}
		} else {
			// said once, up front, while nothing is changed: which boxes were found, which were ignored behind a gap, the hint about a typo and the warning that only DD-1 was found (#1218)
			for _, l := range pre.notes {
				r.say("Boxes for the Reserve, checked before any base is recycled: %s", l)
			}
		}
	} else if firstWhy == "" && enforceEmptyReserve && room0.Reserve.Used > 0 {
		firstWhy = reserveWhy(0, room0.Reserve.Used)
	}
	// The order of the run is: the Reserve an earlier run left is moved into the boxes first, and only then is the room for the first base asked about (applyEach does it in that order).
	// So the question for the first base is put the same way here: when it was refused for room on the save as it is, and a Reserve is there that the run moves first, the move is
	// played on the working copy, the first base is asked about again on it, and the copy is thrown away (the checks above ask nothing of the file; nothing reaches it before the one
	// write). A Reserve with a slot limit that the leftover fills no longer gives 0 of N when moving the leftover makes room.
	if firstWhy == "" && firstRoom != nil {
		d, err := firstAfterLeftoverMove(s, bases)
		// The question threw the working copy away, and that read the file again: the file must still be the one the checkpoint of this run holds, or the run is on another save than the one
		// it was told about (a dry run takes no checkpoint and asks nothing of the file).
		if !o.DryRun && rec.CheckpointSHA256 != "" {
			cur, ferr := fileSHA(s.Path)
			if ferr != nil || cur != rec.CheckpointSHA256 {
				return halt("not changed: the save file changed during the run", "the save file is not the one the checkpoint %s was taken of (it was read again after the first base was asked about: %v); 0 of %d base(s) can be recycled now, and nothing was recycled or exported. Close whatever writes to the save and run this again", rec.Checkpoint, fileChangedWhy(cur, ferr), len(bases))
			}
		}
		switch {
		case err == nil:
			first = d
		case ops.IsRoomRefusal(err):
			firstWhy = "there is no room for it, even after the Reserve an earlier run left is moved into the boxes " + boxesWords + ": " + err.Error()
		case isCannotMove(err):
			firstWhy = reserveWhy(0, room0.Reserve.Used) + ", and " + reserveCannotWords(err)
		default:
			return halt("not changed: the first base could not be asked about", "the first base could not be asked about after the Reserve was moved (%v); nothing was recycled", err)
		}
	}
	if firstWhy == "" {
		if p := spaceProblems(room0, []recycledRecord{first}); len(p) > 0 {
			firstWhy = "there is not enough room: " + strings.Join(p, "; ")
		}
	}
	if firstWhy != "" {
		return halt("not changed: no base fits", "0 of %d base(s) can be recycled now: %s cannot be, because %s. Nothing was recycled and nothing was exported. %s", len(bases), baseLabel(bases[0]), firstWhy, makeRoom())
	}

	// 3. the export, before anything is recycled
	if o.DryRun {
		r.say("Dry run: would first write a console base backup file of each of the %d base(s) and read each back.", len(bases))
	} else {
		folder := exportFolderOf(o, rec)
		files, err := exportBases(s, bases, folder, rec.At, o.Editor)
		rec.ExportFolder, rec.ExportFiles = folder, files
		for _, f := range files {
			r.say("Exported and read back: %s", filepath.Join(folder, f))
		}
		if err != nil {
			rec.Blockers = append(rec.Blockers, "a base could not be exported: "+err.Error())
			r.say("Stopped: %v. Nothing else was changed.", err)
			_, e := stop("stopped: a base could not be exported", fmt.Errorf("stopped: %v", err))
			return recycleOutcome{done: true}, e
		}
	}

	// the save is locked against a second editor, and the state file against a second layout roll, by the lock Roll holds for the whole real run (#1201); a dry run writes nothing

	// 4b. one at a time on the working copy; nothing reaches the file
	res, failed, err := applyEach(s, bases, room0, o.DryRun)
	if err != nil {
		s.Discard()
		// a failure of the MOVE of the Reserve is not a failure of the base it followed: that base was recycled and verified on the working copy
		var nt *notTried
		notTriedErr := errors.As(err, &nt)
		var mf *moveFailure
		moveFailed := errors.As(err, &mf)
		for i, b := range bases {
			switch {
			case i < failed:
				r.say("  - %s: recycled and verified in the working copy only (discarded)", baseLabel(b))
			case i == failed && moveFailed && mf.baseDone:
				r.say("  - %s: recycled and verified in the working copy only (discarded); then moving its Reserve into the boxes FAILED", baseLabel(b))
			case i == failed && moveFailed:
				r.say("  - %s: not tried; moving the Reserve an earlier run left into the boxes FAILED first", baseLabel(b))
			case i == failed && notTriedErr:
				r.say("  - %s: not tried (%v)", baseLabel(b), nt.err)
			case i == failed:
				r.say("  - %s: FAILED", baseLabel(b))
			default:
				r.say("  - %s: not tried", baseLabel(b))
			}
		}
		if moveFailed {
			return halt("not changed: the move of the Reserve failed", "%v. The bases were only tried on a working copy and the save file was not written, so NOTHING is changed (the checkpoint %s is there anyway: Database > Save backups)", err, checkpointWords(o, rec))
		}
		if notTriedErr {
			return halt("not changed: a base could not be tried", "%v. No base was recycled in the save: the bases before it were only tried on a working copy and the save file was not written, so NOTHING is changed (the checkpoint %s is there anyway: Database > Save backups)", err, checkpointWords(o, rec))
		}
		return halt("not changed: a base failed to recycle", "%v. The bases before it were only tried on a working copy and the save file was not written, so NOTHING is changed (the checkpoint %s is there anyway: Database > Save backups)", err, checkpointWords(o, rec))
	}
	done := res.done
	k, n := len(done), len(bases)
	table := spaceTable(room0, done)
	for _, l := range table {
		r.say("  %s", l)
	}
	rec.RecycleSpace = table
	moveAt := func(base int64) {
		for _, m := range res.moves {
			if m.AfterBase != base {
				continue
			}
			what := "after it"
			if base == 0 {
				what = "before the first base (what an earlier run left)"
			}
			verb := "moved"
			if o.DryRun {
				verb = "would be moved"
			}
			r.say("  The Reserve %s into the boxes %s (%s): %d stack(s), %d item(s)", verb, boxNamesWords(m.Boxes), what, m.Stacks, m.Items)
			for _, l := range m.lines {
				r.say("      %s", l)
			}
		}
	}
	moveAt(0)
	for _, d := range done {
		r.say("Base %d %q: %s", d.ID, d.Name, describeRecycled(d))
		for _, l := range d.summary {
			r.say("    %s", l)
		}
		for _, l := range d.warnings {
			r.say("    Warning: %s", l)
		}
		moveAt(d.ID)
	}
	if res.reserveNote != "" {
		r.say("Warning: %s", res.reserveNote)
	}
	if k > 0 {
		r.say("%d of %d base(s) can be recycled in this run: %d Solido device(s), %d stored stacks and %d refund stacks to the Reserve; refund together: %s.", k, n, k, storedTotal(done), refundStacksTotal(done), refundTotal(done))
		r.say("What the recycle does with what the bases held: stored items and the refund go to the Reserve, a Solido device goes to the backpack (the loadout when the backpack has no room). The recycle itself touches no vehicle; the ones of yours in the Deep Desert are taken out afterwards by the vehicle step.")
	}
	if k == 0 {
		s.Discard()
		return halt("not changed: no base fits", "0 of %d base(s) can be recycled now: base %d %q cannot be, because %s. Nothing was recycled. %s", n, bases[0].ID, bases[0].Name, res.why, makeRoom())
	}
	rest := bases[k:]
	partial := ""
	if k < n {
		recycledWords, notWords, continues := "recycled", "was not", "The run continues"
		if o.DryRun {
			recycledWords, notWords, continues = "would be recycled", "would not be", "A real run would continue"
		}
		partial = fmt.Sprintf("%d of %d base(s) %s; base %d %q %s, because %s. Not rolled: the roll needs a Deep Desert with no base in it. %s %s with the remaining base(s) (%s) when run again; the checkpoint of this run is %s", k, n, recycledWords, rest[0].ID, rest[0].Name, notWords, res.why, makeRoom(), continues, labels(rest), checkpointWords(o, rec))
	}
	if o.DryRun {
		s.Discard()
		if k < n {
			r.say("Dry run: %s. Nothing was written.", partial)
			_, e := stop("", errors.New(partial))
			return recycleOutcome{done: true}, e
		}
		r.say("Dry run: that is what a real run does with the bases, after the checkpoint and the exported files; nothing was written.")
		return recycleOutcome{planned: true}, nil
	}

	// 5. the one write, then a read of the file. A write that is not reported as saved may still have replaced the file (it could not be read back in time): the file is compared with what it
	// was before the write, and a file that differs is a changed save whatever the answer said.
	// The file as it is now must be known: without that a write could not be told from no write afterwards, so a save whose file cannot be read is not written.
	before, berr := fileSHA(s.Path)
	if berr != nil || before == "" {
		s.Discard()
		return halt("not changed: the save file could not be read before the write", "the save file cannot be read before the write (%v), so a write could not be told from no write afterwards; the save was not written", berr)
	}
	expect := res.expect()
	// The intent is on record BEFORE the save is written: the state file then holds the checkpoint and the bases about to be recycled, so a run that dies, or a state file that cannot be
	// written afterwards, still leaves the way back on record. A state file that cannot be written now refuses the write.
	rec.SaveFile = s.Path
	rec.Intent = &writeIntent{Bases: recordIDs(done), SaveFile: s.Path, BeforeSHA256: before}
	for _, d := range done {
		rec.Intent.SolidoItems = append(rec.Intent.SolidoItems, d.SolidoItem)
		rec.Intent.SolidoBlueprints = append(rec.Intent.SolidoBlueprints, d.SolidoBlueprint)
	}
	if err := sw.intent(fmt.Sprintf("in progress: about to write the save with %d base(s) recycled (not written yet; if this is the last word, the save may or may not have been written: see the checkpoint)", len(done))); err != nil {
		rec.Intent = nil
		sw.withdraw()
		s.Discard()
		return halt("not changed: the state file could not be written", "the state file cannot be written (%v), and it must hold the checkpoint and the plan before the save is changed; the save was not written", err)
	}
	f := flushSave(s)
	// what is recorded as written (the bases, the moves of the Reserve, the warning about a Reserve left full) is set only once the file is known to hold it
	mark := func() {
		rec.Intent = nil
		rec.Recycled, rec.written, rec.SaveFile = done, true, s.Path
		rec.ReserveMoves, rec.ReserveNote = res.moves, res.reserveNote
	}
	warn := func(err error) { warnStateAfterWrite(o, r, rec, err) }
	changed := func(outcome, why string) (recycleOutcome, error) {
		msg := fmt.Sprintf("the save WAS changed (%d base(s) recycled) but %s. Look at it before the game starts", len(done), why)
		rec.Blockers = append(rec.Blockers, msg)
		r.say("Stopped: %s.", msg)
		_, e := stop(outcome, errors.New("stopped: "+msg))
		return recycleOutcome{done: true}, e
	}
	if !f.Saved {
		same, why := fileLeftAlone(s.Path, before)
		if same {
			s.Discard()
			rec.Intent = nil
			sw.withdraw()
			return halt("not changed: the bases could not be written", "the bases were recycled on the working copy but the save could not be written (%s); the save was not changed", f.Why)
		}
		// the file is different, but what made it different is not known until it is read: it is opened afresh and checked against the plan before anything is said about bases
		if verr := verifyBasesWritten(s.Path, done, rest, expect); verr != nil {
			msg := fmt.Sprintf("the write was not reported as saved (%s), and it is NOT known what happened to the save file: %s; checked against the plan, it does not read back as the recycle that was planned (%v). It may have been changed in part, or by another program. %s. Look at it before the game starts", f.Why, why, verr, rec.wayBack())
			rec.Blockers = append(rec.Blockers, msg)
			r.say("Stopped: %s.", msg)
			// the note stays, marked: the next run must not accept this file by looking at it again
			if rec.Intent != nil {
				rec.Intent.ReadBackFailed = true
				warn(sw.persist("stopped: the save file changed and does not read back as planned"))
			}
			_, e := stop("stopped: the save file changed and does not read back as planned", errors.New("stopped: "+msg))
			return recycleOutcome{done: true}, e
		}
		mark()
		warn(sw.persist(fmt.Sprintf("stopped: the save file changed with %d base(s) recycled, the write was not confirmed", len(done))))
		return changed("stopped: the write was not confirmed and the save file changed", "the write was not reported as saved ("+f.Why+") and "+why)
	}
	// from here the save file HAS been changed
	mark()
	warn(sw.persist(fmt.Sprintf("in progress: the save was written with %d base(s) recycled, not yet read back", len(done))))
	if f.Warning != "" {
		return changed("stopped: the written save needs a look", "the write reported: "+f.Warning)
	}
	if err := verifyBasesWritten(s.Path, done, rest, expect); err != nil {
		return changed("stopped: the written save does not read back", "the file does not read back as written ("+err.Error()+")")
	}
	if len(res.moves) > 0 {
		r.say("The Reserve was moved into the boxes %s: every stack that was moved is in a box and the Reserve holds %d stack(s), as read back from the file.", movedBoxNamesWords(res.moves), expect.ReserveLeft)
	}
	r.say("Recycled %d base(s) in the Deep Desert, one at a time, written in one save and read back from the file.", k)
	if k < n {
		rec.Blockers = append(rec.Blockers, partial)
		r.say("Stopped: %s.", partial)
		_, e := stop(fmt.Sprintf("stopped: %d of %d bases recycled, not rolled", k, n), errors.New("stopped: "+partial))
		return recycleOutcome{done: true}, e
	}
	r.say("No base of yours stands in the Deep Desert any more.")
	return recycleOutcome{}, nil
}

// checkpointWords names this run's checkpoint; a dry run takes none, and says so.
func checkpointWords(o Options, rec *checkRecord) string {
	if o.DryRun || rec.Checkpoint == "" {
		return "a new one, taken by the run before anything is changed (a dry run takes none)"
	}
	return rec.Checkpoint
}

// firstAfterLeftoverMove asks about the first base the way the run will meet it (#1201): the Reserve an earlier run left is moved into the boxes on the working copy first, the base is
// asked about on the copy as that left it, and the copy is thrown away (nothing reaches the file before the one write, and applyEach plays the same move again, with its records). The
// error is the answer as it is: a refusal for room, a refusal of the boxes (cannotMove) or a failure.
func firstAfterLeftoverMove(s *save.Save, bases []ops.HoldBase) (recycledRecord, error) {
	var d recycledRecord
	_, err := emptyReserveInto(s, holdIDs(bases), false)
	if err == nil {
		var pv map[string]any
		if pv, err = previewRecycle(s, bases[0].ID); err == nil {
			d, err = recordOf(bases[0], pv)
		}
	}
	if derr := s.Discard(); derr != nil {
		return d, fmt.Errorf("the working copy could not be reset after the question (%v)", derr)
	}
	return d, err
}

func fileChangedWhy(cur string, err error) string {
	if err != nil {
		return "it cannot be read now (" + err.Error() + ")"
	}
	return "its SHA-256 is now " + cur
}

func reserveWhy(i int, used int64) string {
	if i == 0 {
		return fmt.Sprintf("the Reserve is not empty (it holds %d stack(s)) and no recycle can happen until it is empty", used)
	}
	return fmt.Sprintf("the Reserve now holds %d stack(s) from the base(s) before it and no further recycle can happen until it is empty", used)
}

func makeRoom() string {
	return "Make room: free slots and volume in the storage boxes " + boxesWords + ", or add the next box (name another Developer storage container DD-n, up to DD-6), or empty the Reserve in the game (the run moves it into those boxes itself, #1191); and free the backpack slots the Solido devices need, then run this again."
}

// eachResult is what applyEach did: the bases recycled, and, when it stopped before one only for lack of room, why.
type eachResult struct {
	done []recycledRecord
	why  string

	// the Reserve and the boxes (#1191): the moves made, in order; the stacks the Reserve of the working copy ends with; the ids of every stack moved into a box and the stacks each box
	// holds; and, when the Reserve could not be emptied after the LAST base (so no base is left to hold back), what to tell the person
	moves       []reserveMoveRecord
	reserveLeft int64
	movedIDs    []int64
	boxStacks   map[int64]int64
	reserveNote string
}

// expect is what the written file must hold, as the working copy ended.
func (r eachResult) expect() writtenExpect {
	return writtenExpect{ReserveLeft: r.reserveLeft, MovedItems: r.movedIDs, BoxStacks: r.boxStacks}
}

func holdIDs(bs []ops.HoldBase) []int64 {
	var out []int64
	for _, b := range bs {
		out = append(out, b.ID)
	}
	return out
}

func recordIDs(ds []recycledRecord) []int64 {
	var out []int64
	for _, d := range ds {
		out = append(out, d.ID)
	}
	return out
}

// applyEach recycles the bases one at a time on the working copy, in the order given. Before each base the Reserve must be empty and the question about it (asked on the copy as the
// bases before it left it) must be answered with room for its Solido device with the margin; a base for which that is not so ends the loop (why says it), and so does every base after
// it. After each base the Reserve is moved into the boxes (emptyReserveInto) and the Reserve of a base that finds something in it (what an earlier run left) is moved first; a move
// that the boxes refuse (a box is missing, no room) ends the loop like a base that has no room, and a move that fails otherwise is a failure. dry says that this is a dry run: the
// move is also asked about first, so that the output holds the words of the question. Any other failure is returned as err, with failed the index of the base: the caller throws the
// working copy away.
func applyEach(s *save.Save, bases []ops.HoldBase, room0 ops.RecycleRoom, dry bool) (res eachResult, failed int, err error) {
	exclude := holdIDs(bases)
	res.boxStacks = map[int64]int64{}
	// moveReserve moves the whole Reserve (used stacks) into the boxes, after the base afterBase (0: before the first). It returns the refusal of the boxes (cannotMove) as it is.
	moveReserve := func(afterBase, used int64) error {
		var preview []string
		if dry {
			pv, err := emptyReserveInto(s, exclude, true)
			if err != nil {
				return err
			}
			preview = pv.lines
		}
		mv, err := emptyReserveInto(s, exclude, false)
		if err != nil {
			return err
		}
		if mv.Stacks != used {
			return fmt.Errorf("%d stack(s) were in the Reserve and %d were moved", used, mv.Stacks)
		}
		emptied, err := roomOf(s)
		if err != nil {
			return fmt.Errorf("the room could not be read after the Reserve was moved (%v)", err)
		}
		if emptied.Reserve.Used != 0 {
			return fmt.Errorf("the Reserve still holds %d stack(s) after it was moved into the boxes", emptied.Reserve.Used)
		}
		if dry {
			mv.lines = preview
		}
		mv.AfterBase = afterBase
		if mv.Stacks > 0 {
			res.moves = append(res.moves, mv)
			res.movedIDs = append(res.movedIDs, mv.itemIDs...)
			for _, b := range mv.Boxes {
				res.boxStacks[b.Inventory] = b.StacksAfter
			}
		}
		res.reserveLeft = 0
		return nil
	}
	for i, b := range bases {
		now, err := roomOf(s)
		if err != nil {
			return res, i, &notTried{err: fmt.Errorf("%s: the room could not be read (%v)", baseLabel(b), err)}
		}
		if !now.Reserve.Present {
			return res, i, &notTried{err: fmt.Errorf("%s: the character has no Reserve to receive it", baseLabel(b))}
		}
		res.reserveLeft = now.Reserve.Used
		if enforceEmptyReserve && now.Reserve.Used > 0 {
			if emptyReserveInto == nil {
				res.why = reserveWhy(i, now.Reserve.Used)
				return res, 0, nil
			}
			// something an earlier run left in the Reserve: moved into the boxes before this base
			prev := int64(0)
			if i > 0 {
				prev = bases[i-1].ID
			}
			if err := moveReserve(prev, now.Reserve.Used); err != nil {
				if isCannotMove(err) {
					res.why = reserveWhy(i, now.Reserve.Used) + ", and " + reserveCannotWords(err)
					return res, 0, nil
				}
				return res, i, &moveFailure{err: fmt.Errorf("the Reserve that an earlier run left could not be moved into the boxes before %s (%v)", baseLabel(b), err)}
			}
			if now, err = roomOf(s); err != nil {
				return res, i, &notTried{err: fmt.Errorf("%s: the room could not be read after the Reserve was moved (%v)", baseLabel(b), err)}
			}
		}
		pv, err := previewRecycle(s, b.ID)
		if err != nil {
			if ops.IsRoomRefusal(err) {
				res.why = "there is no room for it: " + err.Error()
				return res, 0, nil
			}
			return res, i, fmt.Errorf("%s was refused after the bases before it were recycled (%v)", baseLabel(b), err)
		}
		prec, err := recordOf(b, pv)
		if err != nil {
			return res, i, fmt.Errorf("%s: %v", baseLabel(b), err)
		}
		if problems := spaceProblems(room0, append(append([]recycledRecord(nil), res.done...), prec)); len(problems) > 0 {
			res.why = "there is not enough room: " + strings.Join(problems, "; ")
			return res, 0, nil
		}
		fp, _ := pv["fingerprint"].(string)
		if fp == "" {
			return res, i, fmt.Errorf("%s: the question gave no fingerprint", baseLabel(b))
		}
		out, err := recycleOne(s, b.ID, fp)
		if err != nil {
			return res, i, fmt.Errorf("%s could not be recycled (%v)", baseLabel(b), err)
		}
		d, err := recordOf(b, out)
		if err != nil {
			return res, i, fmt.Errorf("%s: %v", baseLabel(b), err)
		}
		if err := verifyBaseHere(s, d.ID, d.SolidoItem, d.SolidoBlueprint); err != nil {
			return res, i, fmt.Errorf("%s did not verify after its recycle (%v)", baseLabel(b), err)
		}
		after, err := roomOf(s)
		if err != nil {
			return res, i, fmt.Errorf("%s: the room could not be read after the recycle (%v)", baseLabel(b), err)
		}
		if want := now.Reserve.Used + d.StoredStacks + d.RefundStacks; after.Reserve.Used != want {
			return res, i, fmt.Errorf("%s: the Reserve holds %d stacks after the recycle, and %d were planned", baseLabel(b), after.Reserve.Used, want)
		}
		res.done = append(res.done, d)
		res.reserveLeft = after.Reserve.Used
		// the Reserve into the boxes (#1191), so that the next base finds it empty. With no move (a test that shows the loop without it) the Reserve stays full and the next base is
		// held back by the rule above.
		if emptyReserveInto != nil && after.Reserve.Used > 0 {
			if err := moveReserve(d.ID, after.Reserve.Used); err != nil {
				if !isCannotMove(err) {
					return res, i, &moveFailure{err: fmt.Errorf("the Reserve could not be moved into the boxes after %s was recycled (%v)", baseLabel(b), err), baseDone: true}
				}
				// the base is recycled and verified; what is left is the Reserve, which the boxes refused
				if i == len(bases)-1 {
					res.reserveNote = fmt.Sprintf("after the last base the Reserve still holds %d stack(s): %s. It stays in the Reserve; make room in the boxes %s (or empty it in the game) before the next recycle", after.Reserve.Used, reserveCannotWords(err), boxesWords)
					break
				}
				res.why = reserveWhy(i+1, after.Reserve.Used) + ", and " + reserveCannotWords(err)
				return res, 0, nil
			}
		}
	}
	return res, len(bases), nil
}

func labels(bs []ops.HoldBase) string {
	var l []string
	for _, b := range bs {
		l = append(l, baseLabel(b))
	}
	return strings.Join(l, ", ")
}

func slotWords(free int64) string {
	if free < 0 {
		return "no slot limit recorded"
	}
	return fmt.Sprintf("%d free slot(s)", free)
}

func roomLines(rm ops.RecycleRoom) []string {
	out := []string{}
	if rm.Reserve.Present {
		out = append(out, fmt.Sprintf("Reserve holds %d stack(s) (%s)", rm.Reserve.Used, slotWords(rm.Reserve.FreeSlots)))
	} else {
		out = append(out, "Reserve: none")
	}
	for _, in := range []ops.InventoryRoom{rm.Backpack, rm.Loadout} {
		if !in.Present {
			out = append(out, in.Name+": none")
			continue
		}
		out = append(out, fmt.Sprintf("%s %s", in.Name, slotWords(in.FreeSlots)))
	}
	return out
}

// devicesIn counts the new Solido devices that go to the backpack and to the loadout.
func devicesIn(done []recycledRecord) (pack, load int64) {
	for _, d := range done {
		switch {
		case !d.SolidoNew:
		case d.SolidoWhere == "loadout":
			load++
		default:
			pack++
		}
	}
	return pack, load
}

// spaceProblems says what is short for these bases together. The Reserve is not in it (its rule is "empty before each base", checked in applyEach); the backpack and the loadout must have
// a slot for each new Solido device and keep spaceMargin slots free after them when they have a slot limit, and the volume limit must leave room for the devices.
func spaceProblems(room ops.RecycleRoom, done []recycledRecord) []string {
	pack, load := devicesIn(done)
	var out []string
	for _, c := range []struct {
		in   ops.InventoryRoom
		need int64
	}{{room.Backpack, pack}, {room.Loadout, load}} {
		if c.need == 0 {
			continue
		}
		if !c.in.Present {
			out = append(out, fmt.Sprintf("the character has no %s, and %d slot(s) are needed there", c.in.Name, c.need))
			continue
		}
		if c.in.FreeSlots >= 0 && c.need+spaceMargin > c.in.FreeSlots {
			out = append(out, fmt.Sprintf("%s: %d slot(s) needed for Solido devices plus a margin of %d, %d free (short by %d)", c.in.Name, c.need, spaceMargin, c.in.FreeSlots, c.need+spaceMargin-c.in.FreeSlots))
		}
		if c.in.DeviceFit >= 0 && c.in.DeviceFit < c.need {
			out = append(out, fmt.Sprintf("%s: %d device(s) needed, the volume limit leaves room for %d", c.in.Name, c.need, c.in.DeviceFit))
		}
	}
	return out
}

// spaceTable is one line per destination: what the bases that will be recycled bring there and what is free.
func spaceTable(room ops.RecycleRoom, done []recycledRecord) []string {
	pack, load := devicesIn(done)
	out := []string{fmt.Sprintf("Reserve: empty before each base (holds %d stack(s) now); the %d base(s) bring %d stored stack(s) and %d refund stack(s) in all", room.Reserve.Used, len(done), storedTotal(done), refundStacksTotal(done))}
	for _, c := range []struct {
		in   ops.InventoryRoom
		need int64
	}{{room.Backpack, pack}, {room.Loadout, load}} {
		line := fmt.Sprintf("%s: needs %d slot(s) for new Solido devices; has %s", c.in.Name, c.need, slotWords(c.in.FreeSlots))
		if c.in.Present && c.need > 0 {
			if c.in.FreeSlots >= 0 {
				line += fmt.Sprintf("; with a margin of %d it must have %d", spaceMargin, c.need+spaceMargin)
			}
			if c.in.DeviceFit >= 0 {
				line += fmt.Sprintf("; the volume limit leaves room for %d device(s)", c.in.DeviceFit)
			} else if c.in.VolumeNote != "" {
				line += "; " + strings.TrimSuffix(strings.ToLower(c.in.VolumeNote[:1])+c.in.VolumeNote[1:], ".")
			}
		}
		out = append(out, line)
	}
	return out
}

func storedTotal(done []recycledRecord) (n int64) {
	for _, d := range done {
		n += d.StoredStacks
	}
	return n
}

func refundStacksTotal(done []recycledRecord) (n int64) {
	for _, d := range done {
		n += d.RefundStacks
	}
	return n
}

// refundTotal is the refund of all bases together, item by item, in the order of the item names.
func refundTotal(done []recycledRecord) string {
	sum := map[string]int64{}
	var names []string
	for _, d := range done {
		for _, l := range d.Refund {
			if _, ok := sum[l.Name]; !ok {
				names = append(names, l.Name)
			}
			sum[l.Name] += l.Count
		}
	}
	sort.Strings(names)
	var out []string
	for _, n := range names {
		out = append(out, fmt.Sprintf("%d %s", sum[n], n))
	}
	if len(out) == 0 {
		return "nothing"
	}
	return strings.Join(out, ", ")
}

func describeRecycled(d recycledRecord) string {
	var l []string
	for _, x := range d.Refund {
		l = append(l, fmt.Sprintf("%d %s", x.Count, x.Name))
	}
	refund := "nothing"
	if len(l) > 0 {
		refund = strings.Join(l, ", ")
	}
	dev := fmt.Sprintf("the character's own blank Solido device (item %d) is filled", d.SolidoItem)
	if d.SolidoNew {
		dev = fmt.Sprintf("a new Solido device (item %d) in the %s", d.SolidoItem, d.SolidoWhere)
	}
	return fmt.Sprintf("%d pieces and %d placeables deleted; %d stored stack(s) move to the Reserve; refund %d stack(s) in the Reserve (%s); Solido: %s, blueprint %d", d.Pieces, d.Placeables, d.StoredStacks, d.RefundStacks, refund, dev, d.SolidoBlueprint)
}

// notTried is a failure before a base was attempted at all (the room or the Reserve could not be read for it): that base is reported as not tried, not as a base that failed to recycle.
type notTried struct{ err error }

func (e *notTried) Error() string { return e.err.Error() }
func (e *notTried) Unwrap() error { return e.err }

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
//	   before each base (a recycle sends everything to the Reserve, and the operator's rule is that no further recycle happens until it is empty), and the Solido devices need places
//	   in the backpack or the loadout. The run recycles the largest number k of bases, in order, for which that holds;
//	5. k = 0 refuses and changes nothing; otherwise the save is written once and read back from the file, and when k is less than the number of bases N the run stops before the roll
//	   and says what to do; only k = N goes on to the roll (and the Deep Desert is looked at again first: it must hold no base).
//
// Nothing reaches the file before the write, so a failure at any base before it leaves the save exactly as it was. A run that stopped at k < N is safe to run again: the bases done are
// gone, each run takes its own checkpoint, and the next run starts with the remaining bases.

// spaceMargin is how many slots must remain free in the backpack or the loadout after the new Solido devices. The recycle itself only refuses when there is no room at all; this keeps
// a few slots free so the character is not left with a full backpack.
const spaceMargin = 2

// enforceEmptyReserve is the operator's rule that the Reserve must be empty before each base is recycled. It is a variable only so that a test can show the backpack and loadout room logic
// on its own, with several bases in one run; nothing else changes it.
var enforceEmptyReserve = true

// The reads and writes of this step, as variables only so that a test can make one fail. Nothing else changes them.
var (
	// emptyReserveInto is the call point of the move of the Reserve into the character's storage boxes (#1191, built apart from this change). It is called on the working copy after each
	// base is recycled and verified, and must leave the Reserve empty (the run checks that, and the final read of the written file expects it). It is nil today, which is the behaviour
	// described above: the Reserve stays full, the next base is held back, and the run stops and asks the person to empty it. Wiring #1191 in is setting this one variable.
	emptyReserveInto func(s *save.Save) error

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
	// verifyBasesWritten reads the file that was written, opened afresh: every recycled base gone, every Solido in place, exactly the bases that were not recycled still standing, and the
	// Reserve as planned.
	verifyBasesWritten = func(path string, done []recycledRecord, remaining []ops.HoldBase, start ops.RecycleRoom) error {
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
		want := start.Reserve.Used + reserveNeed(done)
		if emptyReserveInto != nil {
			want = 0 // each base's Reserve was moved into storage right after it
		}
		if room.Reserve.Used != want {
			return fmt.Errorf("the Reserve holds %d stacks, and %d were planned", room.Reserve.Used, want)
		}
		return nil
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

func reserveNeed(done []recycledRecord) int64 { return storedTotal(done) + refundStacksTotal(done) }

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

const reserveRule = "The Reserve must be empty before each base is recycled: a recycle sends everything (what the base stored and its refund) to the Reserve, and no further recycle can happen until the Reserve is empty. Moving the Reserve to storage is not part of this run (it is #1191), so with the Reserve in this state a run recycles only the bases that fit one after the other."

// recycleStep is steps 2 to 5 of the list above. persist writes the state file at once (the guard's own record of the run), which the step calls as soon as the save has been written, before
// anything is read back: a process that dies after that point still leaves the checkpoint and the bases that were recycled on record.
func recycleStep(o Options, r *Result, rec *checkRecord, s *save.Save, h ops.DeepDesertHolds, stop func(string, error) (bool, error), persist func(string), history []string) (recycleOutcome, error) {
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
	r.say("Rule: %s", reserveRule)
	r.say("Room now: %s", strings.Join(roomLines(room0), "; "))

	// 4a. every base is asked about on the save as it is (nothing is changed by asking); a refusal for any reason other than room refuses the whole run, and all of them are listed
	// together. The first base must also fit: when not even it does, the run ends here, before anything is exported, so that a refused run leaves no export folder behind.
	var refused []string
	var first recycledRecord
	firstWhy := ""
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
				firstWhy = "there is no room for it: " + err.Error()
			}
		default:
			refused = append(refused, fmt.Sprintf("%s: %v", baseLabel(b), err))
		}
	}
	if len(refused) > 0 {
		return halt("not changed: a base cannot be recycled", "%d of %d base(s) cannot be recycled, so none was (all or nothing): %s", len(refused), len(bases), strings.Join(refused, " | "))
	}
	if firstWhy == "" && enforceEmptyReserve && room0.Reserve.Used > 0 {
		firstWhy = reserveWhy(0, room0.Reserve.Used)
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

	// the save is locked against a second editor for the writes (a dry run writes nothing)
	if !o.DryRun {
		release, err := lockSave(s.Path)
		if err != nil {
			return halt("not changed: the save is in use", "the save is open in another TABR TAU window or program, so it is not written (%v). Close that and run this again", err)
		}
		defer release()
	}

	// 4b. one at a time on the working copy; nothing reaches the file
	res, failed, err := applyEach(s, bases, room0)
	if err != nil {
		s.Discard()
		for i, b := range bases {
			switch {
			case i < failed:
				r.say("  - %s: recycled and verified in the working copy only (discarded)", baseLabel(b))
			case i == failed:
				r.say("  - %s: FAILED", baseLabel(b))
			default:
				r.say("  - %s: not tried", baseLabel(b))
			}
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
	for _, d := range done {
		r.say("Base %d %q: %s", d.ID, d.Name, describeRecycled(d))
		for _, l := range d.summary {
			r.say("    %s", l)
		}
		for _, l := range d.warnings {
			r.say("    Warning: %s", l)
		}
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
		partial = fmt.Sprintf("%d of %d base(s) recycled; base %d %q was not, because %s. Not rolled: the roll needs a Deep Desert with no base in it. %s The run continues with the remaining base(s) (%s) when run again; the checkpoint of this run is %s", k, n, rest[0].ID, rest[0].Name, res.why, makeRoom(), labels(rest), checkpointWords(o, rec))
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
	before, _ := fileSHA(s.Path)
	f := flushSave(s)
	mark := func() { rec.Recycled, rec.written, rec.SaveFile = done, true, s.Path }
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
			return halt("not changed: the bases could not be written", "the bases were recycled on the working copy but the save could not be written (%s); the save was not changed", f.Why)
		}
		mark()
		persist(fmt.Sprintf("stopped: the save file changed with %d base(s) recycled, the write was not confirmed", len(done)))
		return changed("stopped: the write was not confirmed and the save file changed", "the write was not reported as saved ("+f.Why+") and "+why)
	}
	// from here the save file HAS been changed
	mark()
	persist(fmt.Sprintf("in progress: the save was written with %d base(s) recycled, not yet read back", len(done)))
	if f.Warning != "" {
		return changed("stopped: the written save needs a look", "the write reported: "+f.Warning)
	}
	if err := verifyBasesWritten(s.Path, done, rest, room0); err != nil {
		return changed("stopped: the written save does not read back", "the file does not read back as written ("+err.Error()+")")
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

func reserveWhy(i int, used int64) string {
	if i == 0 {
		return fmt.Sprintf("the Reserve is not empty (it holds %d stack(s)) and no recycle can happen until it is empty", used)
	}
	return fmt.Sprintf("the Reserve now holds %d stack(s) from the base(s) before it and no further recycle can happen until it is empty", used)
}

func makeRoom() string {
	return "Make room: empty the Reserve in the game (move its contents to storage; doing that here is #1191) and free the backpack slots the Solido devices need, then run this again."
}

// eachResult is what applyEach did: the bases recycled, and, when it stopped before one only for lack of room, why.
type eachResult struct {
	done []recycledRecord
	why  string
}

// applyEach recycles the bases one at a time on the working copy, in the order given. Before each base the Reserve must be empty and the question about it (asked on the copy as the
// bases before it left it) must be answered with room for its Solido device with the margin; a base for which that is not so ends the loop (why says it), and so does every base after
// it. Any other failure is returned as err, with failed the index of the base: the caller throws the working copy away.
func applyEach(s *save.Save, bases []ops.HoldBase, room0 ops.RecycleRoom) (res eachResult, failed int, err error) {
	for i, b := range bases {
		now, err := roomOf(s)
		if err != nil {
			return res, i, fmt.Errorf("%s: the room could not be read (%v)", baseLabel(b), err)
		}
		if !now.Reserve.Present {
			return res, i, fmt.Errorf("%s: the character has no Reserve to receive it", baseLabel(b))
		}
		if enforceEmptyReserve && now.Reserve.Used > 0 {
			res.why = reserveWhy(i, now.Reserve.Used)
			return res, 0, nil
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
		// the call point of the move into storage (#1191): nil today, so the Reserve stays full and the next base is held back by the rule above
		if emptyReserveInto != nil {
			if err := emptyReserveInto(s); err != nil {
				return res, i, fmt.Errorf("%s: the Reserve could not be moved into storage after it was recycled (%v)", baseLabel(b), err)
			}
			emptied, err := roomOf(s)
			if err != nil {
				return res, i, fmt.Errorf("%s: the room could not be read after the Reserve was moved (%v)", baseLabel(b), err)
			}
			if emptied.Reserve.Used != 0 {
				return res, i, fmt.Errorf("%s: the Reserve still holds %d stack(s) after it was moved into storage", baseLabel(b), emptied.Reserve.Used)
			}
		}
		res.done = append(res.done, d)
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

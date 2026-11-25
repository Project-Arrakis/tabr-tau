package layoutroll

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// A run that recycles bases writes a note of what it is about to do into the state file BEFORE it writes the save (an "unconfirmed" entry of recycleHistory, with the SHA-256 of the
// save file as it was). A run that ends without recording how the write went (killed, the state file unwritable afterwards, a write that was not confirmed and did not read back) leaves
// that entry. The next run settles it first, before anything else, and never rolls over an entry it could not settle:
//
//   - the save file is exactly as it was: the write never happened; the entry is dropped;
//   - the save file is different and every planned base is gone with its Solido in place: the write took effect and was never read back; the entry becomes a confirmed one, loudly, and the
//     run goes on (the Deep Desert is looked at again anyway);
//   - the save file is different and none of the planned bases is gone: the write did not take effect (the file changed for another reason, for example the game saved since); the entry is
//     dropped, loudly, and the run goes on: the bases that still stand are handled as in any run;
//   - anything else (some of the bases gone and some not, a Solido that is not in place, a file that cannot be read): the run STOPS and says what to look at. It changes nothing.

// settleIntents settles the unconfirmed entries of earlier runs. stopMsg is not empty when the run must stop now. Nothing is written in a dry run.
func settleIntents(o Options, r *Result, st *state, now time.Time) (stopMsg string) {
	if !o.DryRun && !o.lockHeld {
		// never reached by Roll (a run with no lock is refused before this); settling writes the state file, so it is not done without the lock
		r.say("Notes of earlier runs are not settled by a run that does not hold the lock of the save.")
		return ""
	}
	changed := false
	// a dry run settles nothing: it says what a real run would do
	dropped, recorded := "the note is dropped", "It is recorded as written"
	if o.DryRun {
		dropped, recorded = "the note would be dropped (a dry run changes nothing)", "It would be recorded as written (a dry run changes nothing)"
	}
	for _, e := range append([]recycleHistoryEntry(nil), st.RecycleHistory...) {
		if !e.Unconfirmed {
			continue
		}
		who := fmt.Sprintf("the run of %s (checkpoint %s)", e.At.Local().Format("2006-01-02 15:04"), e.Checkpoint)
		back := fmt.Sprintf("The checkpoint %s holds the save as it was before that run (Database > Save backups, or Copy-Item -Force \"%s\" \"%s\" with the game closed). If you have looked at the save and want this note gone, remove the entry marked \"unconfirmed\" from recycleHistory in %s (the run stops on this note every time until you do; --force does not bypass it)", e.Checkpoint, e.CheckpointPath, e.SaveFile, o.StatePath)
		if e.ReadBackFailed {
			return fmt.Sprintf("%s wrote the save and opened it again to check it, and the read-back FAILED: the file did not read back as the recycle that was planned. The save file cannot be accepted as written, so the note is not settled by looking at the same file again and this run changes nothing. Look at the save. %s", who, back)
		}
		if e.SaveFile == "" || e.BeforeSHA256 == "" {
			return fmt.Sprintf("%s noted that it was about to write the save and did not record how that went, and the note does not say which save file it was or how it looked, so it cannot be settled. Look at the save. %s", who, back)
		}
		cur, err := fileSHA(e.SaveFile)
		if err != nil {
			return fmt.Sprintf("%s noted that it was about to write the save and did not record how that went, and the save file cannot be read now (%v), so it cannot be settled. Look at the save. %s", who, err, back)
		}
		switch {
		case cur == e.BeforeSHA256:
			r.say("Earlier run settled: %s noted that it was about to write the save and did not record how that went. The save file is exactly as it was before that run, so the write did not happen; %s.", who, dropped)
			st.dropRecycleRun(e.At)
		default:
			var mism *plannedMismatch
			gone, standing, verr := plannedBasesIn(e)
			switch {
			case errors.As(verr, &mism):
				return fmt.Sprintf("%s noted that it was about to write the save and did not record how that went, and the save file is different from before that run. The file could be read, but it does not hold what was planned: %v. Check that base in the save (is its Solido in your backpack or loadout, linked to the base's blueprint?). Nothing is changed by this run. %s", who, verr, back)
			case verr != nil:
				return fmt.Sprintf("%s noted that it was about to write the save and did not record how that went, and the save file is different from before that run; reading it to settle the note failed (%v). Look at the save. %s", who, verr, back)
			case standing == len(e.Bases):
				r.say("WARNING: %s noted that it was about to write the save and did not record how that went. The save file is different from before that run (it was changed since, for example by the game), but none of the %d planned base(s) is gone, so that write did not take effect; %s. Look at the save if that is not what you expect.", who, len(e.Bases), dropped)
				st.dropRecycleRun(e.At)
			case gone == len(e.Bases):
				r.say("WARNING: %s noted that it was about to write the save and did not record how that went. The save file is different from before that run, and every one of the %d planned base(s) is gone with its Solido in place: the write took effect and was never read back. %s. The way back is the checkpoint %s.", who, len(e.Bases), recorded, e.Checkpoint)
				confirmEntry(st, e)
			default:
				return fmt.Sprintf("%s noted that it was about to write the save and did not record how that went, and the save file now holds a part of it: %d of the %d planned base(s) are gone and %d still stand. Nothing is changed by this run. Look at the save. %s", who, gone, len(e.Bases), standing, back)
			}
		}
		if c := st.LastCheck; c != nil && c.Intent != nil && c.Checkpoint == e.Checkpoint {
			c.Intent = nil
		}
		changed = true
	}
	// the note of an earlier vehicle step (#1201) is settled the same way, after the notes of the bases
	msg, vchanged := settleVehicleNote(o, r, st, now)
	if msg != "" {
		return msg
	}
	changed = changed || vchanged
	if changed && !o.DryRun {
		if err := saveRunErr(o.StatePath, *st, now, "an earlier run's unconfirmed write was settled"); err != nil {
			r.say("WARNING: the state file %s could not be updated after the earlier run's note was settled (%v); the note will be settled again by the next run.", baseOf(o.StatePath), err)
		}
	}
	return ""
}

// vehicleStoredState is the state the game gives a vehicle that is stored in its own vehicle backup (the same number the edit writes; ops/deepdesert_vehicles.go).
const vehicleStoredState = 2

// settleVehicleNote settles the unconfirmed note of an earlier vehicle step (#1201) with the same rule as the notes of the bases: the save file is exactly as it was, so the write never
// happened and the note is dropped; the file is different and every planned vehicle is where the plan put it, so the write took effect and was never recorded, and it is recorded as
// written, loudly; the file is different and no planned vehicle has left, so the write did not take effect (the file changed for another reason) and the note is dropped, loudly; anything
// else (some taken out and some not, a vehicle that is neither where it was nor where the plan put it, a file that cannot be read, a note marked readBackFailed) stops the run, which
// changes nothing. changed says that the state was altered and must be saved (not in a dry run).
func settleVehicleNote(o Options, r *Result, st *state, now time.Time) (stopMsg string, changed bool) {
	n := st.VehicleNote
	if n == nil {
		return "", false
	}
	dropped, recorded := "the note is dropped", "It is recorded as written"
	if o.DryRun {
		dropped, recorded = "the note would be dropped (a dry run changes nothing)", "It would be recorded as written (a dry run changes nothing)"
	}
	who := fmt.Sprintf("the run of %s (checkpoint %s), which was about to take your vehicles out of the Deep Desert,", n.At.Local().Format("2006-01-02 15:04"), n.Checkpoint)
	back := fmt.Sprintf("The checkpoint %s holds the save as it was before that run (Database > Save backups, or Copy-Item -Force \"%s\" \"%s\" with the game closed). If you have looked at the save and want this note gone, remove the whole \"vehicleNote\" key from %s (the run stops on this note every time until you do; --force does not bypass it)", n.Checkpoint, n.CheckpointPath, n.SaveFile, o.StatePath)
	if n.ReadBackFailed {
		return fmt.Sprintf("%s wrote the save and opened it again to check it, and the read-back FAILED: the file did not read back as the vehicles taken out. The save file cannot be accepted as written, so the note is not settled by looking at the same file again and this run changes nothing. Look at the save. %s", who, back), false
	}
	if n.SaveFile == "" || n.BeforeSHA256 == "" || len(n.Vehicles) == 0 {
		return fmt.Sprintf("%s noted that it was about to write the save and did not record how that went, and the note does not say which save file it was, how it looked or which vehicles, so it cannot be settled. Look at the save. %s", who, back), false
	}
	cur, err := fileSHA(n.SaveFile)
	if err != nil {
		return fmt.Sprintf("%s noted that it was about to write the save and did not record how that went, and the save file cannot be read now (%v), so it cannot be settled. Look at the save. %s", who, err, back), false
	}
	if cur == n.BeforeSHA256 {
		r.say("Earlier run settled: %s noted that it was about to write the save and did not record how that went. The save file is exactly as it was before that run, so the write did not happen; %s.", who, dropped)
		st.VehicleNote = nil
	} else {
		var mism *plannedMismatch
		taken, standing, verr := plannedVehiclesIn(*n)
		switch {
		case errors.As(verr, &mism):
			return fmt.Sprintf("%s noted that it was about to write the save and did not record how that went, and the save file is different from before that run. The file could be read, but it does not hold what was planned: %v. Nothing is changed by this run. %s", who, verr, back), false
		case verr != nil:
			return fmt.Sprintf("%s noted that it was about to write the save and did not record how that went, and the save file is different from before that run; reading it to settle the note failed (%v). Look at the save. %s", who, verr, back), false
		case standing == len(n.Vehicles):
			r.say("WARNING: %s noted that it was about to write the save and did not record how that went. The save file is different from before that run (it was changed since, for example by the game), but none of the %d planned vehicle(s) has left, so that write did not take effect; %s. Look at the save if that is not what you expect.", who, len(n.Vehicles), dropped)
			st.VehicleNote = nil
		case taken == len(n.Vehicles):
			r.say("WARNING: %s noted that it was about to write the save and did not record how that went. The save file is different from before that run, and every one of the %d planned vehicle(s) is out of the Deep Desert as planned: the write took effect and was never read back. %s. The way back is the checkpoint %s.", who, len(n.Vehicles), recorded, n.Checkpoint)
			st.recordSettledVehicleNote(*n, now)
			st.VehicleNote = nil
		default:
			return fmt.Sprintf("%s noted that it was about to write the save and did not record how that went, and the save file now holds a part of it: %d of the %d planned vehicle(s) are out and %d still stand where they were. Nothing is changed by this run. Look at the save. %s", who, taken, len(n.Vehicles), standing, back), false
		}
	}
	if c := st.LastCheck; c != nil && c.Intent != nil && len(c.Intent.Vehicles) > 0 && c.Checkpoint == n.Checkpoint {
		c.Intent = nil
	}
	return "", true
}

// plannedVehiclesIn reads the save file of a vehicle note, opened afresh: how many of its planned vehicles are out of the Deep Desert as planned (stored: the actor is at the stored state
// and has a row in the vehicle backup; moved: the actor is on the map it was put on), and how many still stand where they were. A vehicle that is neither is an error (the file is not
// what was planned, and not what it was before either).
func plannedVehiclesIn(n vehicleNote) (taken, standing int, err error) {
	fresh, err := openSave(n.SaveFile)
	if err != nil {
		return 0, 0, err
	}
	defer fresh.Close()
	for _, v := range n.Vehicles {
		a, err := fresh.One(`select map, state st from actors where id=?`, v.ID)
		if err != nil {
			return 0, 0, err
		}
		if a == nil {
			return 0, 0, &plannedMismatch{fmt.Sprintf("vehicle %d is not in the save at all", v.ID)}
		}
		bak, err := fresh.One(`select count(*) n from backup_vehicles where vehicle_id=?`, v.ID)
		if err != nil {
			return 0, 0, err
		}
		mp := ""
		if a["map"] != nil {
			mp = fmt.Sprint(a["map"])
		}
		st := intOf(a, "st")
		stored := intOf(bak, "n") > 0
		switch v.Action {
		case "store":
			switch {
			case st == vehicleStoredState && stored:
				taken++
			case st == v.State && !stored:
				standing++
			default:
				return 0, 0, &plannedMismatch{fmt.Sprintf("vehicle %d was to be stored, and it is neither stored nor as it was (state %d, %d row(s) in the vehicle backup)", v.ID, st, intOf(bak, "n"))}
			}
		case "move":
			switch {
			case v.ToMap == "" || v.ToMap == v.FromMap:
				return 0, 0, &plannedMismatch{fmt.Sprintf("the note does not say where vehicle %d was moved to", v.ID)}
			case mp == v.ToMap && !stored:
				taken++
			case mp == v.FromMap && !stored:
				standing++
			default:
				return 0, 0, &plannedMismatch{fmt.Sprintf("vehicle %d was to be moved to the map %q, and it is neither there nor on the map %q it stood on (it is on %q)", v.ID, v.ToMap, v.FromMap, mp)}
			}
		default:
			return 0, 0, &plannedMismatch{fmt.Sprintf("the note does not say what was to be done with vehicle %d", v.ID)}
		}
	}
	return taken, standing, nil
}

func baseOf(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// confirmEntry turns the unconfirmed entry into a confirmed one.
func confirmEntry(st *state, e recycleHistoryEntry) {
	for i := range st.RecycleHistory {
		if st.RecycleHistory[i].At.Equal(e.At) {
			st.RecycleHistory[i].Unconfirmed = false
			st.RecycleHistory[i].BeforeSHA256, st.RecycleHistory[i].SolidoBlueprints = "", nil
		}
	}
	st.trimHistory()
}

// plannedBasesIn reads the save file of an unconfirmed entry, opened afresh: how many of its planned bases are gone with their Solido in place, and how many still stand. A base that is
// gone without its Solido in place is an error (the write is not what was planned).
func plannedBasesIn(e recycleHistoryEntry) (gone, standing int, err error) {
	fresh, err := openSave(e.SaveFile)
	if err != nil {
		return 0, 0, err
	}
	defer fresh.Close()
	for i, id := range e.Bases {
		r, err := fresh.One(`select count(*) n from actors where id=?`, id)
		if err != nil {
			return 0, 0, err
		}
		if intOf(r, "n") > 0 {
			standing++
			continue
		}
		if i >= len(e.SolidoItems) || i >= len(e.SolidoBlueprints) {
			return 0, 0, &plannedMismatch{fmt.Sprintf("the note does not say which Solido base %d was to get", id)}
		}
		if err := verifyBaseHere(fresh, id, e.SolidoItems[i], e.SolidoBlueprints[i]); err != nil {
			return 0, 0, &plannedMismatch{fmt.Sprintf("a planned base is gone without its Solido rows as planned: base %d is gone but %v", id, err)}
		}
		gone++
	}
	return gone, standing, nil
}

// plannedMismatch is the file being read fine and not holding what the note planned (as opposed to a file that cannot be read).
type plannedMismatch struct{ msg string }

func (e *plannedMismatch) Error() string { return e.msg }

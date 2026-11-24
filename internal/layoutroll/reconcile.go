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
	changed := false
	for _, e := range append([]recycleHistoryEntry(nil), st.RecycleHistory...) {
		if !e.Unconfirmed {
			continue
		}
		who := fmt.Sprintf("the run of %s (checkpoint %s)", e.At.Local().Format("2006-01-02 15:04"), e.Checkpoint)
		back := fmt.Sprintf("The checkpoint %s holds the save as it was before that run (Database > Save backups, or Copy-Item -Force \"%s\" \"%s\" with the game closed). If you have looked at the save and want this note gone, remove the entry marked \"unconfirmed\" from recycleHistory in %s", e.Checkpoint, e.CheckpointPath, e.SaveFile, o.StatePath)
		if e.SaveFile == "" || e.BeforeSHA256 == "" {
			return fmt.Sprintf("%s noted that it was about to write the save and did not record how that went, and the note does not say which save file it was or how it looked, so it cannot be settled. Look at the save. %s", who, back)
		}
		cur, err := fileSHA(e.SaveFile)
		if err != nil {
			return fmt.Sprintf("%s noted that it was about to write the save and did not record how that went, and the save file cannot be read now (%v), so it cannot be settled. Look at the save. %s", who, err, back)
		}
		switch {
		case cur == e.BeforeSHA256:
			r.say("Earlier run settled: %s noted that it was about to write the save and did not record how that went. The save file is exactly as it was before that run, so the write did not happen; the note is dropped.", who)
			st.dropRecycleRun(e.At)
		default:
			gone, standing, verr := plannedBasesIn(e)
			switch {
			case verr != nil:
				return fmt.Sprintf("%s noted that it was about to write the save and did not record how that went, and the save file is different from before that run; reading it to settle the note failed (%v). Look at the save. %s", who, verr, back)
			case standing == len(e.Bases):
				r.say("WARNING: %s noted that it was about to write the save and did not record how that went. The save file is different from before that run (it was changed since, for example by the game), but none of the %d planned base(s) is gone, so that write did not take effect; the note is dropped. Look at the save if that is not what you expect.", who, len(e.Bases))
				st.dropRecycleRun(e.At)
			case gone == len(e.Bases):
				r.say("WARNING: %s noted that it was about to write the save and did not record how that went. The save file is different from before that run, and every one of the %d planned base(s) is gone with its Solido in place: the write took effect and was never read back. It is recorded as written. The way back is the checkpoint %s.", who, len(e.Bases), e.Checkpoint)
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
	if changed && !o.DryRun {
		if err := saveRunErr(o.StatePath, *st, now, "an earlier run's unconfirmed write was settled"); err != nil {
			r.say("WARNING: the state file %s could not be updated after the earlier run's note was settled (%v); the note will be settled again by the next run.", baseOf(o.StatePath), err)
		}
	}
	return ""
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
			return 0, 0, errors.New("the note does not say which Solido each base was to get")
		}
		if err := verifyBaseHere(fresh, id, e.SolidoItems[i], e.SolidoBlueprints[i]); err != nil {
			return 0, 0, fmt.Errorf("base %d is gone but %v", id, err)
		}
		gone++
	}
	return gone, standing, nil
}

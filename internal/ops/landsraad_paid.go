package ops

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/kit/durable"
)

// ---------------------------------------------------------------- the ground the editor already paid (#777, refs #655, #776)
//
// A board raise pays the missions it stands for (Solari, House Scrip, specialization XP), the reward rows of the tiers it crosses and, while the house is open, the same ground
// again on the faction's progress. Since #776 the editor writes no progress event, so the game's own record no longer says what the editor paid, and a tier removal followed by the
// same raise paid everything again. The editor therefore remembers the ground it paid itself, outside the game's save and outside the game's tables: a small file of its own next to
// its other state files (the journal, the write log), written only after the edit that paid it is in the game's file.
//
//	file          landsraad-paid-<hash of the save's path>.json in the editor's own folder; the hash is the one the journal's name uses
//	identity      the save's path (lower case) is inside the file and must be this save's, else the file is ignored; each entry carries the character (player controller) too
//	key           term + house + faction (+ character); the value is the highest contribution whose ground the editor paid there (a tier's threshold)
//	written       through the repo's durable replace (temp file, flush, rename, folder flush), by the sink of the save's notes, after the file of the game was written and never before
//	read          a file that is missing, not readable, not this format or of another save is read as "nothing paid": the guards of #776 (the character's own events, the ground
//	              under another faction) still apply, and a raise then pays as it did before this file existed. It is never repaired, moved or deleted by a read.
//	a removal     lowers the contribution and nothing here: the record is what was paid, not what stands
//	trusted only  while the save still shows a reward row of this character for the house written in this term (the editor writes one for every tier it pays, and the game keeps it
//	              for the term): a save restored from a backup made before the raise has no such row, and the record is then read as "nothing paid" instead of withholding a mission
//	              that the restored save never received
//
// What is recorded is the ground the editor paid, not the ground that stands, so the record only ever grows within a term, and a record of an earlier term is dropped the next time
// the file is written.

const (
	paidNoteKind    = "landsraad.paid" // the kind of the note a raise attaches to its edit (save.Mut.Note)
	paidFileVersion = 1
	paidMaxBytes    = 1 << 20
	paidMaxEntries  = 4096
)

// paidNote is what a raise asks to be remembered, and one entry of the file.
type paidNote struct {
	Player  int64   `json:"player"`
	Term    int64   `json:"term"`
	House   string  `json:"house"`
	Faction int64   `json:"faction"`
	Paid    float64 `json:"paid"`
}

type paidFile struct {
	Version int        `json:"version"`
	Save    string     `json:"save"` // the save's path, lower case: the file is ignored for any other save
	Entries []paidNote `json:"entries"`
}

// PaidLedger is the editor's own record of the Landsraad ground it paid, in one file.
type PaidLedger struct {
	path string
	save string
	mu   sync.Mutex
	opts durable.Options
}

// NewPaidLedger keeps the record in path for the save at savePath ("" for path gives a ledger that records and reads nothing).
func NewPaidLedger(path, savePath string) *PaidLedger {
	return &PaidLedger{path: path, save: strings.ToLower(savePath), opts: durable.Defaults()}
}

// PaidLedgerName is the name of the file for a save, from the hash the journal's name uses (h is the first six bytes of the hash, as hex).
func PaidLedgerName(hash string) string { return "landsraad-paid-" + hash + ".json" }

var errPaidFile = errors.New("the Landsraad paid-ground file is not usable")

// read returns the entries of the file. Anything that is not a whole file of this format for this save is an error: the caller reads that as "nothing paid".
func (l *PaidLedger) read() ([]paidNote, error) {
	b, err := os.ReadFile(l.path)
	if err != nil {
		return nil, err
	}
	if len(b) > paidMaxBytes {
		return nil, fmt.Errorf("%w: it is larger than %d bytes", errPaidFile, paidMaxBytes)
	}
	var f paidFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%w: %v", errPaidFile, err)
	}
	if f.Version != paidFileVersion || f.Save != l.save {
		return nil, fmt.Errorf("%w: another version or another save", errPaidFile)
	}
	return f.Entries, nil
}

// recorded is the highest ground recorded for the character on the house in the term, under any faction (the ground was paid to the character, whatever faction it was in), or 0.
// A ledger that is nil, has no path, or whose file cannot be used gives 0.
func (l *PaidLedger) recorded(player, term int64, house string) float64 {
	if l == nil || l.path == "" {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	es, err := l.read()
	if err != nil {
		return 0
	}
	return highestPaid(es, player, term, house)
}

func highestPaid(es []paidNote, player, term int64, house string) float64 {
	best := 0.0
	for _, e := range es {
		if e.Player == player && e.Term == term && e.House == house && e.Paid > best {
			best = e.Paid
		}
	}
	return best
}

// Record is the sink of the save's notes: it merges the paid-ground notes of the edits that were just written into the file. Notes of other kinds are not its concern. A file that
// cannot be used is replaced by a new one holding only these notes (the old content was read as "nothing paid" anyway); a file that cannot be written is an error, which the
// save logs: the write it follows has happened and cannot be undone.
func (l *PaidLedger) Record(notes []save.Note) error {
	if l == nil || l.path == "" {
		return nil
	}
	var add []paidNote
	for _, n := range notes {
		if n.Kind != paidNoteKind {
			continue
		}
		var p paidNote
		if err := json.Unmarshal(n.Data, &p); err != nil || p.House == "" || !(p.Paid > 0) {
			continue
		}
		add = append(add, p)
	}
	if len(add) == 0 {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	old, err := l.read()
	if err != nil {
		old = nil
	}
	// what is kept: the entries of the newest term seen, the highest value per key. A term older than the newest one in the file or in the notes is over, and a raise can only reach
	// the current term (currentTerm), so dropping it loses nothing a raise could still ask about.
	newest := int64(0)
	for _, e := range append(append([]paidNote{}, old...), add...) {
		if e.Term > newest {
			newest = e.Term
		}
	}
	type key struct {
		player, term, faction int64
		house                 string
	}
	merged := map[key]float64{}
	var order []key
	put := func(e paidNote) {
		if e.Term != newest {
			return
		}
		k := key{e.Player, e.Term, e.Faction, e.House}
		cur, seen := merged[k]
		if !seen {
			order = append(order, k)
		}
		merged[k] = max(cur, e.Paid)
	}
	for _, e := range old {
		put(e)
	}
	for _, e := range add {
		put(e)
	}
	out := paidFile{Version: paidFileVersion, Save: l.save, Entries: make([]paidNote, 0, len(order))}
	for _, k := range order {
		out.Entries = append(out.Entries, paidNote{Player: k.player, Term: k.term, House: k.house, Faction: k.faction, Paid: merged[k]})
	}
	if len(out.Entries) > paidMaxEntries {
		out.Entries = out.Entries[len(out.Entries)-paidMaxEntries:]
	}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return err
	}
	return durable.WriteFile(l.path, b, 0o600, l.opts)
}

// paidOf is the ground the editor paid for the character on the house in the term that still counts: what the file records, when the save still shows a reward row of the
// character for the house written in this term (hasRow), and what edits queued but not yet written carry (they are in the working copy, so no check is needed for them).
func paidOf(l *PaidLedger, pending []json.RawMessage, player, term int64, house string, hasRow bool) float64 {
	best := 0.0
	if hasRow {
		best = l.recorded(player, term, house)
	}
	var es []paidNote
	for _, raw := range pending {
		var p paidNote
		if json.Unmarshal(raw, &p) == nil {
			es = append(es, p)
		}
	}
	return max(best, highestPaid(es, player, term, house))
}

// paidGround is what the editor paid before for the character on the house, inside a raise's edit: the record in the file when the save still shows a reward row of the character for
// the house written in this term, and what the edits queued before this one carry. The row is looked for only when the file records something, so a save with no record is read as
// before.
func paidGround(m *save.Mut, ledger *PaidLedger, me myLandsraad, termID int64, house string, clock termClock) (float64, error) {
	hasRow := false
	if ledger.recorded(me.player, termID, house) > 0 {
		r, err := m.One(`select 1 x from landsraad_house_rewards where player_id=? and house_name=? and last_updated>=? limit 1`, me.player, house, clock.start)
		if err != nil {
			return 0, err
		}
		hasRow = r != nil
	}
	return paidOf(ledger, m.Pending(paidNoteKind), me.player, termID, house, hasRow), nil
}

// notePaidBefore says what part of a raise was not paid because the editor paid it before: lo0 is the ground the game's own events and the other factions cover, lo the same with
// the editor's record, after the contribution the raise goes to. The ground between lo0 and the smaller of after and lo, and the missions it stands for, are the part.
func (r *rewardResult) notePaidBefore(after, lo0, lo float64, g GrantSettings) {
	top := min(after, lo)
	if lo <= lo0 || top <= lo0 {
		return
	}
	r.paidGround = top - lo0
	if step := missionContribution * g.ContributionMultiplier; step > 0 {
		if n := ceilSteps(top, step) - ceilSteps(lo0, step); n > 0 {
			r.paidMissions = int64(min(n, 1e9))
		}
	}
}

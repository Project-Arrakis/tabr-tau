package layoutroll

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The move of the Reserve into the storage boxes between the bases of a run (#1191 wired into #1167). A recycle sends everything to the Reserve and no further recycle can happen
// until it is empty, so after each base (and before a base, when an earlier run left the Reserve full) the whole Reserve is moved, whole and unchanged, into the storage boxes the
// operator named DD-1, DD-2 ... DD-6 (as many as run in a row from DD-1, #1214), by the editor's own move (ops.PreviewMoveReserve, then ops.MoveReserve with the fingerprint the preview gave) on the SAME working copy as the recycles.
// Nothing reaches the file before the one write of the run.

// The storage boxes are named DD-1, DD-2 ... DD-6, matched exactly, capitals included, as the move does. The run never takes other names: reserveBoxNames is nil, which asks the move to
// find the boxes itself (ops.ReserveBoxScan: one detection for the command, the editor and this roll), as many as run in a row from DD-1 and at least DD-1.
func reserveBoxNames() []string { return nil }

// boxesWords names the boxes in a sentence.
const boxesWords = "DD-1, DD-2 ... DD-6 (as many as run in a row from DD-1)"

// cannotMove is the refusal of a move that left the working copy as it was: no box of that name, a box that is refused, or no room in them. The run does not fail on it: it stops
// like a run that is out of room (the bases done stay done, the rest is left for the next run). Any other error of the move fails the whole run, with the file untouched.
type cannotMove struct{ err error }

func (c *cannotMove) Error() string { return c.err.Error() }
func (c *cannotMove) Unwrap() error { return c.err }

func isCannotMove(err error) bool {
	var c *cannotMove
	return errors.As(err, &c)
}

// moveRefusal says whether the preview of the move refused for a reason of the boxes or the Reserve (the kinds a person can put right) and not because something failed.
func moveRefusal(err error) bool {
	if ops.IsMoveNoRoom(err) {
		return true
	}
	for _, k := range []error{apierr.ErrNotFound, apierr.ErrNoRoom, apierr.ErrConflict, apierr.ErrBadArgument} {
		if errors.Is(err, k) {
			return true
		}
	}
	return false
}

// boxState is where one box stands after a move: its inventory and how many stacks it holds.
type boxState struct {
	Name        string `json:"name"`
	Inventory   int64  `json:"inventory"`
	StacksAfter int64  `json:"stacksAfter"`
}

// reserveMoveRecord is one move of the Reserve into the boxes, as kept in the state file. AfterBase is the base that was recycled just before it (0 for the move of what an earlier run
// left, made before the first base of this run).
type reserveMoveRecord struct {
	AfterBase int64      `json:"afterBase"`
	Stacks    int64      `json:"stacks"`
	Items     int64      `json:"items"`
	Boxes     []boxState `json:"boxes,omitempty"`

	// what the move said, for the run's output (the preview's words in a dry run); the ids of the stacks, to check the written file; not kept in the state file
	lines   []string
	notes   []string // the words of the scan of the boxes (which were found, which were ignored, the typo hint, the DD-1 warning), for the preflight
	itemIDs []int64
}

// movedReserve is what one call of emptyReserveInto did (or, with previewOnly, would do).
type movedReserve = reserveMoveRecord

// moveReserveIntoBoxes is the real emptyReserveInto.
func moveReserveIntoBoxes(s *save.Save, exclude []int64, previewOnly bool) (movedReserve, error) {
	op := &ops.Ops{S: s}
	opt := ops.MoveReserveOptions{ExcludeBases: exclude}
	pv, err := asMap(op.PreviewMoveReserve(reserveBoxNames(), opt))
	if err != nil {
		if moveRefusal(err) {
			return movedReserve{}, &cannotMove{err}
		}
		return movedReserve{}, err
	}
	mr, err := movedFrom(pv)
	if err != nil {
		return mr, err
	}
	if previewOnly || pv["nothingToDo"] == true {
		return mr, nil
	}
	fp, _ := pv["fingerprint"].(string)
	if fp == "" {
		return mr, errors.New("the question about the move of the Reserve gave no fingerprint")
	}
	inv := intOf(pv, "reserveInventory")
	rows, err := s.Query(`select id from items where inventory_id=? order by position_index, id`, inv)
	if err != nil {
		return mr, err
	}
	var ids []int64
	for _, r := range rows {
		switch v := r["id"].(type) {
		case int64:
			ids = append(ids, v)
		}
	}
	if int64(len(ids)) != mr.Stacks {
		return mr, fmt.Errorf("the Reserve holds %d stack(s) and the move was asked about %d", len(ids), mr.Stacks)
	}
	out, err := asMap(op.MoveReserve(reserveBoxNames(), fp, opt))
	if err != nil {
		return mr, err
	}
	done, err := movedFrom(out)
	if err != nil {
		return done, err
	}
	if done.Stacks != mr.Stacks || done.Items != mr.Items {
		return done, fmt.Errorf("the move was asked about %d stack(s) and moved %d", mr.Stacks, done.Stacks)
	}
	done.itemIDs = ids
	return done, nil
}

// boxNamesWords names the boxes a move used ("DD-1, DD-2 and DD-3"); the general words when it names none.
func boxNamesWords(bs []boxState) string {
	var n []string
	for _, b := range bs {
		n = append(n, b.Name)
	}
	switch len(n) {
	case 0:
		return boxesWords
	case 1:
		return n[0]
	}
	return strings.Join(n[:len(n)-1], ", ") + " and " + n[len(n)-1]
}

// movedBoxNamesWords names the boxes the moves of a run used, each once, in the order they first appear.
func movedBoxNamesWords(moves []reserveMoveRecord) string {
	var all []boxState
	seen := map[string]bool{}
	for _, m := range moves {
		for _, b := range m.Boxes {
			if !seen[b.Name] {
				seen[b.Name] = true
				all = append(all, b)
			}
		}
	}
	return boxNamesWords(all)
}

// movedFrom reads the answer of the preview or of the move.
func movedFrom(m map[string]any) (movedReserve, error) {
	var mr movedReserve
	mr.Stacks, mr.Items = intOf(m, "stacks"), intOf(m, "items")
	switch v := m["summary"].(type) {
	case []string:
		mr.lines = v
	}
	if v, ok := m["scanNotes"].([]string); ok {
		mr.notes = v
	}
	boxes, ok := m["boxes"].([]map[string]any)
	if !ok || (mr.Stacks > 0 && len(boxes) == 0) {
		return mr, errors.New("the answer of the move of the Reserve names no boxes (a bug)")
	}
	for _, b := range boxes {
		name, _ := b["name"].(string)
		mr.Boxes = append(mr.Boxes, boxState{Name: name, Inventory: intOf(b, "inventory"), StacksAfter: intOf(b, "stacksBefore") + intOf(b, "stacks")})
	}
	return mr, nil
}

// writtenExpect is what the file that was written must hold besides the bases: the stacks the Reserve of the working copy ended with, the ids of the stacks that were moved into the
// boxes, and the number of stacks each box holds.
type writtenExpect struct {
	ReserveLeft int64
	MovedItems  []int64
	BoxStacks   map[int64]int64 // box inventory -> stacks
}

// verifyMovedWritten checks the boxes of the file that was written, opened afresh: each box holds the stacks it was planned to hold and every stack that was moved is in a box.
func verifyMovedWritten(fresh *save.Save, want writtenExpect) error {
	for inv, n := range want.BoxStacks {
		r, err := fresh.One(`select count(*) n from items where inventory_id=?`, inv)
		if err != nil {
			return err
		}
		if got := intOf(r, "n"); got != n {
			return fmt.Errorf("the storage box (inventory %d) holds %d stack(s), and %d were planned", inv, got, n)
		}
	}
	if len(want.MovedItems) == 0 {
		return nil
	}
	var in []string
	for inv := range want.BoxStacks {
		in = append(in, fmt.Sprint(inv))
	}
	if len(in) == 0 {
		return errors.New("stacks were moved but no box is recorded (a bug)")
	}
	var ids []string
	for _, id := range want.MovedItems {
		ids = append(ids, fmt.Sprint(id))
	}
	r, err := fresh.One(`select count(*) n from items where id in (` + strings.Join(ids, ",") + `) and inventory_id in (` + strings.Join(in, ",") + `)`)
	if err != nil {
		return err
	}
	if got := intOf(r, "n"); got != int64(len(want.MovedItems)) {
		return fmt.Errorf("%d of the %d stack(s) that were moved into the boxes are in them", got, len(want.MovedItems))
	}
	return nil
}

// moveFailure is a failure of the move of the Reserve into the boxes that is not a refusal of the boxes: the run's working copy is thrown away. baseDone says that the base the move
// followed had been recycled and verified on the working copy (false for the move of what an earlier run left, made before the first base).
type moveFailure struct {
	err      error
	baseDone bool
}

func (m *moveFailure) Error() string { return m.err.Error() }
func (m *moveFailure) Unwrap() error { return m.err }

package ops

import (
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// ---------------------------------------------------------------- the clock of landsraad_tasks.completion_time (#465)
//
// The column is not Unix time. In every save read (many files, every completed row the game wrote) it holds whole seconds of the game's play clock, the same clock as
// the term's start_time / end_time: every one of those values lies inside its own term's window. The play clock counts the seconds the game has run: the save keeps it as
//
//	(universe_lastactive_timestamp - universe_time_timestamp - down_time_accumulation) / 1,000,000
//
// of the farm_variables row (all three in microseconds). Read at the moment of the save, that value lies inside the latest term in every save read and is never below the
// largest completion_time the game wrote in it (a value a little above the largest game-written one, inside the term's window). A task that is open holds NULL
// in completion_time (and NULL in winning_faction_id); the game never wrote 0. See docs/design/landsraad-completion-clock.md for the evidence.

// gameClockNow is the play clock at the time of the save in whole seconds. ok is false when the save has no farm_variables row or the numbers do not make a clock.
func gameClockNow(m *save.Mut) (secs int64, ok bool) {
	r, err := m.One(`select universe_time_timestamp ut, universe_lastactive_timestamp la, down_time_accumulation dt from farm_variables limit 1`)
	if err != nil || r == nil {
		return 0, false
	}
	ut, ok1 := r["ut"].(int64)
	la, ok2 := r["la"].(int64)
	dt, ok3 := r["dt"].(int64)
	if !ok1 || !ok2 || !ok3 {
		return 0, false
	}
	if s := (la - ut - dt) / 1_000_000; s >= 0 {
		return s, true
	}
	return 0, false
}

// termClock is what a completed task of a term is given as completion_time and the window every value the game wrote lies in.
type termClock struct {
	term, value, start, end int64
	ok                      bool // false when the term is not in the save: nothing can be said about the clock, and a time is not written
}

// clockOfTerm gives the value for a completion now: the play clock at the time of the save, held inside the term's window (the term has started when the editor can reach it,
// and a term already over takes its end). A save that has the term but no usable clock gets the start of the term, which is inside the window.
func clockOfTerm(m *save.Mut, termID int64) termClock {
	t, err := m.One(`select start_time s, end_time e from landsraad_decree_term where term_id=?`, termID)
	if err != nil || t == nil {
		return termClock{}
	}
	s, ok1 := t["s"].(int64)
	e, ok2 := t["e"].(int64)
	if !ok1 || !ok2 || e < s {
		return termClock{}
	}
	v := s
	if now, ok := gameClockNow(m); ok {
		v = min(max(now, s), e)
	}
	return termClock{term: termID, value: v, start: s, end: e, ok: true}
}

// arg is the value to bind: the clock value, or NULL when the clock is not known.
func (c termClock) arg() any {
	if !c.ok {
		return nil
	}
	return c.value
}

// repairCompletionTimes brings the completed tasks of a term whose completion_time is outside the term's window (the Unix time the editor wrote before #465) onto the play
// clock, inside the edit that completes or reopens houses of that term. A time the game wrote is always inside the window, so a time inside it is never touched; an open task
// is never touched here either (Reopen clears its time). It returns how many tasks were repaired.
func repairCompletionTimes(m *save.Mut, termID int64) (int64, error) {
	c := clockOfTerm(m, termID)
	if !c.ok {
		return 0, nil
	}
	res, err := m.Exec(`update landsraad_tasks set completion_time=? where term_id=? and completed=1 and completion_time is not null and (completion_time < ? or completion_time > ?)`,
		c.value, termID, c.start, c.end)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// ---------------------------------------------------------------- the position of the save on the play clock, for the page's Term and decrees section (#922)
//
// The term's start_time and end_time are play-clock seconds. The save also holds where the play clock stood when it was written, and the real (Unix) time of that write
// (farm_variables.universe_lastactive_timestamp). That is all the save knows about the two clocks: the play clock stops while the game is closed, and the save does not say when
// those stops were, so the real date of any other play-clock second (the start or the end of a term) cannot be worked out from it. The page may show a distance on the play clock
// from the save's own position, and the real time of the save itself, and nothing more.

// saveClock reads the position of the save on the play clock: play_seconds (whole seconds, the clock of gameClockNow) and saved_at (Unix seconds of the last write). Nil when the save
// has no farm_variables row or its numbers do not make a clock. Read only.
func saveClock(o *Ops) map[string]any {
	rows, err := o.S.Query(`select universe_time_timestamp ut, universe_lastactive_timestamp la, down_time_accumulation dt from farm_variables limit 1`)
	if err != nil || len(rows) == 0 {
		return nil
	}
	ut, ok1 := rows[0]["ut"].(int64)
	la, ok2 := rows[0]["la"].(int64)
	dt, ok3 := rows[0]["dt"].(int64)
	if !ok1 || !ok2 || !ok3 {
		return nil
	}
	s := (la - ut - dt) / 1_000_000
	if s < 0 || la <= 0 {
		return nil
	}
	return map[string]any{"play_seconds": s, "saved_at": la / 1_000_000}
}

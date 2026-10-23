package ops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The automatic pass: when the editor opens (and the master switch autoRefillOnOpen is on) it runs the jobs below that are switched on, all
// in ONE edit: one transaction, one backup, one write-log line. A job that fails rolls the whole pass back, so the others are never half
// applied, and a pass in which nothing needed doing changes no rows (no backup, no log line, no save). Everything acts on what the
// character owns or carries: their own vehicles, inventories, worn gear, loadout, and the bases of the save they play (single player).

// AutoJob is one thing the automatic pass can do. The page lists them with a switch each (the settings key says where the switch is kept).
type AutoJob struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Desc    string `json:"desc"`    // one line for the page
	Key     string `json:"key"`     // the settings.json key of its switch (settings.JobKeys)
	Default bool   `json:"default"` // its switch when the file does not say
	Ready   bool   `json:"ready"`
	Reason  string `json:"reason,omitempty"` // why it is not ready (it is then listed, greyed, and never runs)

	noun    string // what the job looks after, for "were already full"
	repairs bool   // it repairs rather than fills, for "already full or in good repair"
	// run does the job inside the shared edit m for the character c. did says what it changed in a few words ("" when nothing needed doing); note is a
	// fact worth showing that is not a change ("this save has no hydration record").
	run func(m *save.Mut, c player) (did, note string, err error)
	// noChar is true for a job that needs no character (base water and power belong to the save, not to the person).
	noChar bool
}

func many(n int, one, more string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, more)
}

// AutoJobs is the registry, in the order the page lists them. Every switch defaults to on: the operator asked for all of them, repairs
// included. The master switch (autoRefillOnOpen) still defaults to off.
func AutoJobs() []AutoJob {
	return []AutoJob{
		{ID: "water", Label: "Base water", Key: "autoRefillWater", Default: true, Ready: true, noChar: true, noun: "water storage",
			Desc: "Fills Water Cisterns, Windtraps and Blood Purifiers to capacity.",
			run: func(m *save.Mut, _ player) (string, string, error) {
				r, err := refillBaseWaterTx(m)
				if err != nil || r.filled == 0 {
					return "", "", err
				}
				return r.summary, "", nil
			}},
		{ID: "power", Label: "Base power", Key: "autoRefillPower", Default: true, Ready: true, noChar: true, noun: "generators, wind turbines and windtraps",
			Desc: "Tops up generators, wind turbines and windtraps (fuel, lubricant, filters).",
			run: func(m *save.Mut, _ player) (string, string, error) {
				r, err := refillGeneratorsTx(m)
				if err != nil || r.filled == 0 {
					return "", "", err
				}
				return r.summary, "", nil
			}},
		{ID: "repair-vehicles", Label: "Repair vehicles", Key: "autoRepairVehicles", Default: true, Ready: true, noun: "vehicles", repairs: true,
			Desc: "Repairs every part of your own vehicles to full durability.",
			run: func(m *save.Mut, c player) (string, string, error) {
				n, err := repairVehiclesTx(m, c.Controller, 100, 0)
				if err != nil || n == 0 {
					return "", "", err
				}
				return many(int(n), "vehicle part repaired", "vehicle parts repaired"), "", nil
			}},
		{ID: "repair-gear", Label: "Repair worn equipment and loadout", Key: "autoRepairGear", Default: true, Ready: true, noun: "worn equipment and loadout", repairs: true,
			Desc: "Repairs what you wear and your loadout (not the items in your backpack).",
			run: func(m *save.Mut, c player) (string, string, error) {
				_, n, err := repairGearTx(m, c.Pawn)
				if err != nil || n == 0 {
					return "", "", err
				}
				return many(n, "worn or loadout item repaired", "worn or loadout items repaired"), "", nil
			}},
		{ID: "fill-containers", Label: "Fill water containers", Key: "autoFillContainers", Default: true, Ready: true, noun: "water containers",
			Desc: "Fills the Literjons and other water containers you carry.",
			run: func(m *save.Mut, c player) (string, string, error) {
				r, err := refillContainersTx(m, c.Pawn)
				if err != nil || r.filled == 0 {
					return "", "", err
				}
				return many(r.filled, "water container filled", "water containers filled"), "", nil
			}},
		{ID: "fill-hydration", Label: "Fill your water", Key: "autoFillHydration", Default: true, Ready: true, noun: "your water",
			Desc: "Fills the character's own water to the maximum.",
			run: func(m *save.Mut, c player) (string, string, error) {
				before, err := fillHydrationTx(m, c.Pawn, true)
				if errors.Is(err, errNoHydration) {
					return "", "your water was not filled: this save has no hydration record for the character", nil
				}
				if err != nil || before >= maxHydration {
					return "", "", err
				}
				return "your water filled", "", nil
			}},
		{ID: "fill-blood", Label: "Fill blood sacks", Key: "autoFillBloodSacks", Default: true, noun: "blood sacks",
			Desc:   "Fills the blood sacks you carry.",
			Reason: "arrives with the blood sack feature"},
		{ID: "refuel", Label: "Refuel vehicles", Key: "autoRefuelVehicles", Default: true, noun: "vehicle tanks",
			Desc:   "Fills the fuel tank of your own vehicles.",
			Reason: "arrives with the refuel feature"},
	}
}

// AutoJobError is a job of the pass that failed. The whole pass was rolled back: no job changed anything.
type AutoJobError struct {
	ID, Label string
	Err       error
}

func (e *AutoJobError) Error() string { return fmt.Sprintf("%s: %v", e.Label, e.Err) }
func (e *AutoJobError) Unwrap() error { return e.Err }

// AutoDone is what one job of a pass did.
type AutoDone struct {
	ID, Label, Text string
}

// AutoPass is the outcome of RunAutoPass.
type AutoPass struct {
	Ran     []string   // the jobs that were run, in order
	Done    []AutoDone // the ones that changed something
	Notes   []string   // facts worth showing that are not changes (a job that was skipped and why)
	Changed bool       // the pass changed rows (and so is waiting to be written)
	nouns   []string
	repairs bool
}

// Summary lists what the pass did, job after job ("1 Water Cistern; 11 vehicle parts repaired"). It is empty when nothing was done.
func (p AutoPass) Summary() string {
	parts := make([]string, 0, len(p.Done))
	for _, d := range p.Done {
		parts = append(parts, d.Text)
	}
	return strings.Join(parts, "; ")
}

// NothingNeeded is the sentence for a pass that found nothing to do, naming what it looked at.
func (p AutoPass) NothingNeeded() string {
	state := "already full"
	if p.repairs {
		state = "already full or in good repair"
	}
	return "Automatic refill: " + strings.Join(p.nouns, ", ") + " were " + state + "."
}

// RunAutoPass runs every job that is ready and that on says yes to, as one edit named "automatic pass: ...". A job that cannot run (the
// save has no character for one that needs the character) is skipped with a note. A job that fails stops the pass, rolls everything back and
// is returned as an *AutoJobError.
func (o *Ops) RunAutoPass(on func(AutoJob) bool) (AutoPass, error) {
	return o.runAutoJobs(AutoJobs(), on)
}

func (o *Ops) runAutoJobs(jobs []AutoJob, on func(AutoJob) bool) (AutoPass, error) {
	var todo []AutoJob
	needChar := false
	for _, j := range jobs {
		if j.Ready && j.run != nil && on(j) {
			todo = append(todo, j)
			needChar = needChar || !j.noChar
		}
	}
	if len(todo) == 0 {
		return AutoPass{}, nil
	}
	var c player
	var charErr error
	if needChar {
		c, charErr = o.player()
	}
	var out AutoPass
	n, err := o.S.Mutate("automatic pass", func(m *save.Mut) error {
		out = AutoPass{} // the function may be run again by a caller that retries: start from nothing
		for _, j := range todo {
			if !j.noChar && charErr != nil {
				out.Notes = append(out.Notes, strings.ToLower(j.Label[:1])+j.Label[1:]+" skipped: "+charErr.Error())
				continue
			}
			did, note, err := j.run(m, c)
			if err != nil {
				return &AutoJobError{ID: j.ID, Label: j.Label, Err: err}
			}
			out.Ran = append(out.Ran, j.ID)
			out.nouns = append(out.nouns, j.noun)
			out.repairs = out.repairs || j.repairs
			if note != "" {
				out.Notes = append(out.Notes, note)
			}
			if did != "" {
				out.Done = append(out.Done, AutoDone{j.ID, j.Label, did})
			}
		}
		m.Desc = "automatic pass: " + out.Summary()
		return nil
	})
	if err != nil {
		return AutoPass{}, err
	}
	out.Changed = n > 0
	if !out.Changed { // no row changed: whatever a job said, nothing was done
		out.Done = nil
	}
	return out, nil
}

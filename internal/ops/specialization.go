package ops

import (
	"fmt"
	"sort"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Specialization as the Dune Docker console's tab has it (console/web/src/features/players/SpecializationTab.tsx and the specialization
// functions of console/api/src/duneDb.js, MIT, RedBlink): the five tracks with their XP, level and keystones, Add XP, Grant Max and Reset
// per track, and Grant All / Reset All Keystones.
//
// One difference in the data: the console's database names the tracks (an enum), the save stores a bare number in
// specialization_tracks.track_type and nothing in the save says which number is which track. Keystones are named by track
// ("Combat_CombatKeystone_..."), so they need no numbers. For XP and level the editor needs the number of each track: the ones the game uses were
// measured (measuredTrackNumbers, #609) and are used where nothing was assigned; a number assigned on the Specialization tab (kept in the editor's
// settings) takes the place of the measured one. EffectiveTrackNumbers puts the two together.

// SpecTracks are the five tracks in the console's order.
var SpecTracks = []string{"Combat", "Crafting", "Exploration", "Gathering", "Sabotage"}

// specCumulativeXP is the XP at which each specialization level starts (console/api/src/duneDb/presentation.js).
var specCumulativeXP = []int64{
	0, 100, 205, 315, 431, 553, 681, 816, 958, 1107, 1264,
	1428, 1599, 1777, 1963, 2157, 2359, 2570, 2790, 3019, 3258,
	3504, 3757, 4017, 4285, 4561, 4845, 5137, 5438, 5748, 6067,
	6393, 6727, 7069, 7419, 7777, 8143, 8518, 8902, 9295, 9697,
	10107, 10525, 10951, 11385, 11827, 12277, 12736, 13204, 13681, 14167,
	14661, 15163, 15673, 16191, 16717, 17251, 17794, 18346, 18907, 19477,
	20052, 20632, 21217, 21807, 22402, 23002, 23608, 24220, 24838, 25462,
	26086, 26710, 27334, 27958, 28582, 29206, 29830, 30454, 31078, 31702,
	32326, 32950, 33574, 34198, 34822, 35446, 36070, 36694, 37318, 37942,
	38566, 39190, 39814, 40438, 41062, 41686, 42310, 42934, 43558, 44182,
}

// maxSpecXP is the XP of the last level; a track never holds more.
var maxSpecXP = specCumulativeXP[len(specCumulativeXP)-1]

// specLevel is the level for an XP amount, with the fraction of the way to the next one (the console's specializationXpToLevel).
func specLevel(xp int64) float64 {
	if xp <= 0 {
		return 0
	}
	if xp >= maxSpecXP {
		return float64(len(specCumulativeXP) - 1)
	}
	lo := sort.Search(len(specCumulativeXP), func(i int) bool { return specCumulativeXP[i] > xp }) - 1
	cur, next := specCumulativeXP[lo], specCumulativeXP[lo+1]
	return float64(lo) + float64(xp-cur)/float64(next-cur)
}

func validTrack(t string) bool {
	for _, x := range SpecTracks {
		if x == t {
			return true
		}
	}
	return false
}

// SpecRow is one track of the Specialization table.
type SpecRow struct {
	Track         string  `json:"track"`
	Number        *int64  `json:"number"` // the track_type number the editor works with for this track (an assigned one, else the measured one); null when none can be told
	XP            int64   `json:"xp"`
	Level         float64 `json:"level"`
	KeystoneOwned int64   `json:"keystoneOwned"`
	KeystoneTotal int64   `json:"keystoneTotal"`
	Granted       bool    `json:"granted"` // every keystone of the track is owned
}

// SpecRaw is a row of specialization_tracks whose number no track has been assigned to yet.
type SpecRaw struct {
	Number int64   `json:"number"`
	XP     int64   `json:"xp"`
	Level  float64 `json:"level"`
}

// Specs is the Specialization tab: every track (even one with no XP yet, as in the console), its keystones, and the rows of the save
// whose track number is not known yet. nums is the track numbers in use (EffectiveTrackNumbers of the editor's settings).
func (o *Ops) Specs(nums map[string]int64) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	raw, err := o.S.Query(`select track_type n, xp_amount xp, level l from specialization_tracks where player_id=? order by track_type`, p.Controller)
	if err != nil {
		return nil, err
	}
	byNum := map[int64]SpecRaw{}
	for _, r := range raw {
		n, _ := toInt(r["n"])
		xp, _ := toInt(r["xp"])
		l, _ := r["l"].(float64)
		byNum[n] = SpecRaw{Number: n, XP: xp, Level: l}
	}
	var resetCount int64
	for _, n := range knownTrackNumbers(nums) {
		if _, has := byNum[n]; has {
			resetCount++
		}
	}
	rows := make([]SpecRow, 0, len(SpecTracks))
	used := map[int64]bool{}
	for _, t := range SpecTracks {
		row := SpecRow{Track: t}
		if n, ok := nums[t]; ok {
			nn := n
			row.Number = &nn
			used[n] = true
			if r, has := byNum[n]; has {
				row.XP, row.Level = r.XP, r.Level
			}
		}
		tot, _ := o.S.One(`select count(*) c from specialization_keystones_map where substr(name,1,?)=?`, len(t)+1, t+"_")
		have, _ := o.S.One(`select count(*) c from purchased_specialization_keystones p join specialization_keystones_map m on m.id=p.keystone_id
			where p.player_id=? and substr(m.name,1,?)=?`, p.Controller, len(t)+1, t+"_")
		row.KeystoneTotal, _ = toInt(tot["c"])
		row.KeystoneOwned, _ = toInt(have["c"])
		row.Granted = row.KeystoneTotal > 0 && row.KeystoneOwned >= row.KeystoneTotal
		rows = append(rows, row)
	}
	unassigned := []SpecRaw{}
	for n, r := range byNum {
		if !used[n] {
			unassigned = append(unassigned, r)
		}
	}
	sort.Slice(unassigned, func(i, j int) bool { return unassigned[i].Number < unassigned[j].Number })
	return map[string]any{"rows": rows, "unassigned": unassigned, "maxXp": maxSpecXP, "tracks": SpecTracks, "resetCount": resetCount}, nil
}

// EffectiveTrackNumbers are the track numbers the Specialization tab and the Landsraad write work with: the ones the editor learned, and for every track it learned none the number
// measured in the game (measuredTrackNumbers: 1 Crafting, 2 Gathering, 3 Exploration, 4 Combat, 5 Sabotage). Two tracks never share a row: a learned number that is not one of
// the five tracks is ignored, a learned number that an earlier track (in SpecTracks order) already holds is left out, and a measured number that a learned number uses for another
// track is left out. A track left out stays unknown and the tab then offers the assignment. The learned map is not changed.
func EffectiveTrackNumbers(learned map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(SpecTracks))
	used := map[int64]bool{}
	for _, t := range SpecTracks {
		if n, ok := learned[t]; ok && !used[n] {
			out[t] = n
			used[n] = true
		}
	}
	for _, t := range SpecTracks {
		if _, has := out[t]; has {
			continue
		}
		if n, ok := measuredTrackNumbers[t]; ok && !used[n] {
			out[t] = n
			used[n] = true
		}
	}
	return out
}

// trackNumber is the number the editor works with for a track (nums is EffectiveTrackNumbers), or an error saying how to set it right.
func trackNumber(nums map[string]int64, track string) (int64, error) {
	if !validTrack(track) {
		return 0, apierr.New(apierr.ErrBadArgument, "unknown specialization track")
	}
	n, ok := nums[track]
	if !ok {
		return 0, fmt.Errorf("the number of %s is not known yet, because another track holds it: set that other track again to its own number in the Track numbers card below the table, then try again", track)
	}
	return n, nil
}

func (o *Ops) writeSpec(desc string, controller, number, xp int64, level float64) error {
	_, err := o.S.Mutate(desc, func(m *save.Mut) error { return upsertSpecInTx(m, controller, number, xp, level) })
	return err
}

func upsertSpecInTx(m *save.Mut, controller, number, xp int64, level float64) error {
	_, e := m.Exec(`insert into specialization_tracks(player_id, track_type, xp_amount, level) values(?,?,?,?)
		on conflict(player_id, track_type) do update set xp_amount=excluded.xp_amount, level=excluded.level`, controller, number, xp, level)
	return e
}

// AddSpecXP adds XP to a track (args track, amount a whole number from 1 to 44182) and sets the level to match; a track never goes above the XP of
// its last level. It never removes XP: zero, a negative number and anything that is not a whole number are refused and nothing is written (#1032);
// the XP of a track goes down only through ResetSpec.
func (o *Ops) AddSpecXP(nums map[string]int64, a Args) (any, error) {
	track := a.Str("track")
	n, err := trackNumber(nums, track)
	if err != nil {
		return nil, err
	}
	amount, err := a.IntRange("amount", 1, maxSpecXP)
	if err != nil {
		return nil, apierr.Newf(apierr.ErrBadArgument, "amount must be a whole number from 1 to %d; XP is added here, never removed (use Reset to take it back)", maxSpecXP)
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	// The current XP is read and the new total written in one transaction (audit L2 G09), so two overlapping calls add up instead of both
	// starting from the same old value.
	var old, next int64
	if _, err := o.S.Mutate(fmt.Sprintf("specialization %s xp %+d", track, amount), func(m *save.Mut) error {
		old = 0
		cur, err := m.One(`select xp_amount xp from specialization_tracks where player_id=? and track_type=?`, p.Controller, n)
		if err != nil {
			return err
		}
		if len(cur) > 0 {
			old, _ = toInt(cur["xp"])
		}
		next = min(max(old+amount, 0), maxSpecXP)
		return upsertSpecInTx(m, p.Controller, n, next, specLevel(next))
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "before": old, "after": next, "level": specLevel(next)}, nil
}

// GrantMaxSpec sets a track to the XP of its last level and level 100, as the console does.
func (o *Ops) GrantMaxSpec(nums map[string]int64, a Args) (any, error) {
	track := a.Str("track")
	n, err := trackNumber(nums, track)
	if err != nil {
		return nil, err
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	if err := o.writeSpec("specialization "+track+" max", p.Controller, n, maxSpecXP, 100); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "xp": maxSpecXP, "level": 100}, nil
}

// ResetSpec removes a track's row (XP and level back to 0); its keystones are not touched, as in the console.
func (o *Ops) ResetSpec(nums map[string]int64, a Args) (any, error) {
	track := a.Str("track")
	n, err := trackNumber(nums, track)
	if err != nil {
		return nil, err
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var removed int64
	if _, err := o.S.Mutate("specialization "+track+" reset", func(m *save.Mut) (e error) {
		removed, e = deleteSpecTx(m, p.Controller, n)
		return
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "removed": removed}, nil
}

// GrantAllKeystones buys every specialization keystone of every track that the character does not have.
func (o *Ops) GrantAllKeystones() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var n int64
	if _, err := o.S.Mutate("grant all specialization keystones", func(m *save.Mut) error {
		res, e := m.Exec(`insert or ignore into purchased_specialization_keystones(player_id, keystone_id) select ?, id from specialization_keystones_map`, p.Controller)
		if e == nil {
			n, _ = res.RowsAffected()
		}
		return e
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "granted": n}, nil
}

// ResetAllKeystones removes every keystone the character has bought.
func (o *Ops) ResetAllKeystones() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var n int64
	if _, err := o.S.Mutate("reset all specialization keystones", func(m *save.Mut) error {
		res, e := m.Exec(`delete from purchased_specialization_keystones where player_id=?`, p.Controller)
		if e == nil {
			n, _ = res.RowsAffected()
		}
		return e
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "removed": n}, nil
}

// CheckAssign validates "track X is number N": X must be a track, N a small whole number, and no other track may already have N. It returns
// the new mapping to keep.
func CheckAssign(nums map[string]int64, track string, number int64) (map[string]int64, error) {
	if !validTrack(track) {
		return nil, apierr.New(apierr.ErrBadArgument, "unknown specialization track")
	}
	if number < 0 || number > 255 {
		return nil, apierr.New(apierr.ErrBadArgument, "the track number must be from 0 to 255")
	}
	next := map[string]int64{}
	for t, n := range nums {
		if t != track && n == number {
			return nil, fmt.Errorf("number %d is already assigned to %s", number, t)
		}
		next[t] = n
	}
	next[track] = number
	return next, nil
}

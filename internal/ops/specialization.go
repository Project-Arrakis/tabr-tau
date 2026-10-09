package ops

import (
	"errors"
	"fmt"
	"sort"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Specialization as the Dune Docker console's tab has it (console/web/src/features/players/SpecializationTab.tsx and the specialization
// functions of console/api/src/duneDb.js, MIT, RedBlink): the five tracks with their XP, level and keystones, Add XP, Grant Max and Reset
// per track, and Grant All / Reset All Keystones.
//
// One difference in the data: the console's database names the tracks (an enum), the save stores a bare number in
// specialization_tracks.track_type and nothing in the save says which number is which track. Keystones are named by track
// ("Combat_CombatKeystone_..."), so they need no numbers. For XP and level the editor needs the number of each track: it is learned from the
// character's own rows (a track the character has earned XP in has a row) and kept in the editor's settings.

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
	Number        *int64  `json:"number"` // the track_type number in this save, once learned
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
// whose track number is not known yet. nums is the learned track numbers from the editor's settings.
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
	return map[string]any{"rows": rows, "unassigned": unassigned, "maxXp": maxSpecXP, "tracks": SpecTracks}, nil
}

// trackNumber is the learned number of a track, or an error saying how to learn it.
func trackNumber(nums map[string]int64, track string) (int64, error) {
	if !validTrack(track) {
		return 0, errors.New("unknown specialization track")
	}
	n, ok := nums[track]
	if !ok {
		return 0, fmt.Errorf("the number the game uses for %s in this save is not known yet: earn a little XP in it in game, save, and assign the new row to %s below the table", track, track)
	}
	return n, nil
}

func (o *Ops) writeSpec(desc string, controller, number, xp int64, level float64) error {
	_, err := o.S.Mutate(desc, func(m *save.Mut) error {
		_, e := m.Exec(`insert into specialization_tracks(player_id, track_type, xp_amount, level) values(?,?,?,?)
			on conflict(player_id, track_type) do update set xp_amount=excluded.xp_amount, level=excluded.level`, controller, number, xp, level)
		return e
	})
	return err
}

// AddSpecXP adds XP to a track (args track, amount from -44182 to 44182, not 0) and sets the level to match; a track never goes below 0
// or above the XP of its last level.
func (o *Ops) AddSpecXP(nums map[string]int64, a Args) (any, error) {
	track := a.Str("track")
	n, err := trackNumber(nums, track)
	if err != nil {
		return nil, err
	}
	amount, err := a.IntRange("amount", -maxSpecXP, maxSpecXP)
	if err != nil || amount == 0 {
		return nil, errors.New("amount must be a whole number from -44182 to 44182, not 0")
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	cur, _ := o.S.One(`select xp_amount xp from specialization_tracks where player_id=? and track_type=?`, p.Controller, n)
	var old int64
	if len(cur) > 0 {
		old, _ = toInt(cur["xp"])
	}
	next := min(max(old+amount, 0), maxSpecXP)
	if err := o.writeSpec(fmt.Sprintf("specialization %s xp %+d", track, amount), p.Controller, n, next, specLevel(next)); err != nil {
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
	if _, err := o.S.Mutate("specialization "+track+" reset", func(m *save.Mut) error {
		res, e := m.Exec(`delete from specialization_tracks where player_id=? and track_type=?`, p.Controller, n)
		if e == nil {
			removed, _ = res.RowsAffected()
		}
		return e
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
		return nil, errors.New("unknown specialization track")
	}
	if number < 0 || number > 255 {
		return nil, errors.New("the track number must be from 0 to 255")
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

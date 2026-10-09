package ops

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/skills"
)

const (
	moduleDataPath = "$.FLevelComponent[1].ModuleData"
	unspentPath    = "$.FLevelComponent[1].UnspentSkillPoints"
	maxUnspent     = 100000 // the console's limit when it sets unspent points
)

var moduleKeyRe = regexp.MustCompile(`^\(TagName="(Skills\.[^"]+)"\)$`)

// modulePath is the JSON path of one module inside ModuleData. The module key in a save is `(TagName="Skills.Perk.X")`, quotes and
// all; the id must already have passed skills.ValidID, so it holds nothing that could end the quoted key early.
func modulePath(id string) string {
	return moduleDataPath + `."(TagName=\"` + id + `\")"`
}

// SkillRow is one skill module of the character.
type SkillRow struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	School string `json:"school"` // Trooper, Mentat, Planetologist, BeneGesserit, Swordmaster, Hidden or Other
	Kind   string `json:"kind"`
	Rank   int    `json:"rank"`
	Max    int    `json:"max"`
	Spent  int64  `json:"spent"`
	Ladder []int  `json:"ladder"`
	Known  bool   `json:"known"` // in the console's catalog (its ranks and costs are known)
}

// Skills is the character's skill modules as the console's Skill Browser lists them, with the point totals. Spent is what the
// modules' SkillPointsSpent adds up to; the save does not guarantee that spent plus unspent equals the total (some skills are
// granted free), so none of the three is derived from the others.
func (o *Ops) Skills() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	r, _ := o.S.One(`select json_extract(f.components,'$.FLevelComponent[1].TotalSkillPoints') total,
			json_extract(f.components,'$.FLevelComponent[1].UnspentSkillPoints') unspent
		from actor_fgl_entities a join fgl_entities f on f.entity_id=a.entity_id
		where a.actor_id=? and a.slot_name='DuneCharacter' and json_valid(f.components, 8)
			and json_type(f.components,'$.FLevelComponent[1].ModuleData') is not null`, p.Pawn)
	if len(r) == 0 {
		return nil, errors.New("this save has no skill record for the character")
	}
	saved, err := o.S.Query(`select m.key k, json_extract(m.value,'$.SkillPointsSpent') s
		from actor_fgl_entities a join fgl_entities f on f.entity_id=a.entity_id,
			json_each(f.components,'`+moduleDataPath+`') m
		where a.actor_id=? and a.slot_name='DuneCharacter' and json_valid(f.components, 8)`, p.Pawn)
	if err != nil {
		return nil, err
	}
	spentBy := map[string]int64{}
	var spentTotal int64
	for _, s := range saved {
		m := moduleKeyRe.FindStringSubmatch(fmt.Sprint(s["k"]))
		if m == nil {
			continue
		}
		n, _ := toInt(s["s"])
		spentBy[m[1]] = n
		spentTotal += n
	}
	rows := make([]SkillRow, 0, len(spentBy))
	seen := map[string]bool{}
	for _, m := range skills.All() {
		seen[m.ID] = true
		sp := spentBy[m.ID]
		rows = append(rows, SkillRow{ID: m.ID, Name: skills.DisplayName(m), School: m.Category, Kind: skills.Kind(m.ID), Rank: skills.Rank(int(sp), m, true), Max: m.MaxLevel, Spent: sp, Ladder: m.PointLadder, Known: true})
	}
	// modules the save has and the catalog omits are listed, not hidden, but their ranks are not known
	var extra []string
	for id := range spentBy {
		if !seen[id] {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	for _, id := range extra {
		sp := spentBy[id]
		name := id
		if i := strings.LastIndex(id, "."); i >= 0 {
			name = id[i+1:]
		}
		rows = append(rows, SkillRow{ID: id, Name: name, School: "Other", Kind: skills.Kind(id), Rank: skills.Rank(int(sp), skills.Module{}, false), Spent: sp, Known: false})
	}
	total, _ := toInt(r["total"])
	unspent, _ := toInt(r["unspent"])
	return map[string]any{"total": total, "unspent": unspent, "spent": spentTotal, "modules": rows, "schools": skills.Schools}, nil
}

// characterLevel reads the character's level entity inside an edit.
func characterLevel(m *save.Mut, pawn int64) (entity any, unspent int64, err error) {
	rows, err := m.Query(`select f.entity_id id, json_extract(f.components,'`+unspentPath+`') unspent
		from actor_fgl_entities a join fgl_entities f on f.entity_id=a.entity_id
		where a.actor_id=? and a.slot_name='DuneCharacter' and json_valid(f.components, 8)
			and json_type(f.components,'$.FLevelComponent[1].ModuleData') is not null`, pawn)
	if err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return nil, 0, errors.New("this save has no skill record for the character")
	}
	u, ok := toInt(rows[0]["unspent"])
	if !ok {
		return nil, 0, errors.New("this save has no skill record for the character")
	}
	return rows[0]["id"], u, nil
}

// setModuleTx sets one module to a rank by writing the points that rank costs (cumulative). With charge the difference is taken
// from, or given back to, the unspent points. It returns the points before and after.
func setModuleTx(m *save.Mut, pawn int64, id string, rank int, charge bool) (before, after int64, err error) {
	mod, known := skills.Lookup(id)
	if !known {
		return 0, 0, fmt.Errorf("%s is not in the skill catalog, so its ranks and costs are not known", id)
	}
	points, ok := skills.Points(mod, rank)
	if !ok {
		return 0, 0, fmt.Errorf("%s has ranks 0 to %d", id, mod.MaxLevel)
	}
	ent, unspent, err := characterLevel(m, pawn)
	if err != nil {
		return 0, 0, err
	}
	cur, err := m.Query(`select json_extract(components,'`+modulePath(id)+`.SkillPointsSpent') s from fgl_entities where entity_id=?`, ent)
	if err != nil {
		return 0, 0, err
	}
	var had bool
	if len(cur) > 0 {
		before, had = toInt(cur[0]["s"])
	}
	after = int64(points)
	if charge {
		if unspent-(after-before) < 0 {
			return 0, 0, fmt.Errorf("not enough unspent skill points: %s needs %d more than you have", id, after-before-unspent)
		}
		if _, err := m.Exec(`update fgl_entities set components=jsonb_set(components,'`+unspentPath+`', ?) where entity_id=?`, unspent-(after-before), ent); err != nil {
			return 0, 0, err
		}
	}
	if had {
		_, err = m.Exec(`update fgl_entities set components=jsonb_set(components, ?, ?) where entity_id=?`, modulePath(id)+".SkillPointsSpent", after, ent)
	} else {
		_, err = m.Exec(`update fgl_entities set components=jsonb_set(components, ?, jsonb(?)) where entity_id=?`, modulePath(id), fmt.Sprintf(`{"SkillPointsSpent":%d}`, after), ent)
	}
	return before, after, err
}

// SetSkillModule sets a skill module to a rank (args module, level; charge to pay for it from, or refund it to, the unspent
// points; the console's own control does not touch the points, so the default does not either).
func (o *Ops) SetSkillModule(a Args) (any, error) {
	id := a.Str("module")
	if !skills.ValidID(id) {
		return nil, errors.New("invalid skill module")
	}
	rank, err := a.IntRange("level", 0, 100)
	if err != nil {
		return nil, err
	}
	charge := a.Bool("charge", false)
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var before, after int64
	if _, err := o.S.Mutate(fmt.Sprintf("skill %s rank %d", id, rank), func(m *save.Mut) (e error) {
		before, after, e = setModuleTx(m, p.Pawn, id, int(rank), charge)
		return
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "pointsBefore": before, "pointsAfter": after}, nil
}

// SetSkillPoints sets the unspent skill points (0 to 100,000, the console's limit). Nothing else in the level record changes.
func (o *Ops) SetSkillPoints(a Args) (any, error) {
	n, err := a.IntRange("points", 0, maxUnspent)
	if err != nil {
		return nil, err
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var before int64
	if _, err := o.S.Mutate(fmt.Sprintf("unspent skill points = %d", n), func(m *save.Mut) error {
		ent, cur, err := characterLevel(m, p.Pawn)
		if err != nil {
			return err
		}
		before = cur
		if cur == n {
			return nil
		}
		_, err = m.Exec(`update fgl_entities set components=jsonb_set(components,'`+unspentPath+`', ?) where entity_id=?`, n, ent)
		return err
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "before": before, "after": n}, nil
}

// RestoreStarterSkills raises a school's starter skills (its first Key skill and first ability, as the console's preset does) to
// rank 1 when they are lower. It never lowers anything and does not touch the unspent points.
func (o *Ops) RestoreStarterSkills(a Args) (any, error) {
	school := a.Str("school")
	starters := skills.StarterSkills(school)
	if len(starters) == 0 {
		return nil, errors.New("unknown school")
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	changed := 0
	if _, err := o.S.Mutate("restore starter skills: "+school, func(m *save.Mut) error {
		for _, s := range starters {
			ent, _, err := characterLevel(m, p.Pawn)
			if err != nil {
				return err
			}
			mod, _ := skills.Lookup(s.ID)
			cur, err := m.Query(`select json_extract(components,'`+modulePath(s.ID)+`.SkillPointsSpent') s from fgl_entities where entity_id=?`, ent)
			if err != nil {
				return err
			}
			var spent int64
			if len(cur) > 0 {
				spent, _ = toInt(cur[0]["s"])
			}
			if skills.Rank(int(spent), mod, true) >= s.Rank {
				continue
			}
			if _, _, err := setModuleTx(m, p.Pawn, s.ID, s.Rank, false); err != nil {
				return err
			}
			changed++
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "changed": changed}, nil
}

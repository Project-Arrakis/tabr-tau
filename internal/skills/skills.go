// Package skills knows the skill modules of the game (name, school, ranks and what each rank costs).
//
// admin-skill-modules.json is a copy, unchanged, of runtime/data/admin-skill-modules.json of
// https://github.com/Red-Blink/dune-awakening-selfhost-docker (MIT licence, copyright RedBlink; git blob 555e587d); see
// internal/notices/THIRD-PARTY-NOTICES.md. Each module has the cumulative points spent after each rank (pointLadder), which is what
// a save stores as SkillPointsSpent, so a rank is read from the points by that ladder, as the console does
// (console/api/src/duneDb.js rankFromSkillPoints).
//
// It is data and rules only: it never imports the save.
package skills

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

//go:embed admin-skill-modules.json
var raw []byte

// Module is one skill module.
type Module struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"` // the school: Trooper, Mentat, Planetologist, BeneGesserit, Swordmaster, or Hidden
	MaxLevel    int    `json:"maxLevel"`
	PointLadder []int  `json:"pointLadder"`
}

var (
	mods []Module
	byID map[string]Module
)

func init() {
	if err := json.Unmarshal(raw, &mods); err != nil {
		panic("skills: admin-skill-modules.json is not valid: " + err.Error())
	}
	byID = make(map[string]Module, len(mods))
	for _, m := range mods {
		byID[m.ID] = m
	}
	sort.SliceStable(mods, func(i, j int) bool {
		if mods[i].Category != mods[j].Category {
			return mods[i].Category < mods[j].Category
		}
		return mods[i].ID < mods[j].ID
	})
}

// All lists the catalog's modules by school then id. The slice must not be changed.
func All() []Module { return mods }

// Lookup finds a module by id.
func Lookup(id string) (Module, bool) { m, ok := byID[id]; return m, ok }

var idRe = regexp.MustCompile(`^Skills(\.[A-Za-z0-9_]+){1,4}$`)

// ValidID says whether a module id has the shape the game's ids have. The id is put inside a JSON path when a save is edited, so
// nothing else (quotes, brackets, spaces) is ever accepted.
func ValidID(id string) bool { return len(id) <= 120 && idRe.MatchString(id) }

// Rank is the rank a number of spent points stands for, by the module's ladder. A module not in the catalog (the game ships a few
// the catalog omits) cannot be inverted, so it claims one rank for any points spent, never more.
func Rank(points int, m Module, known bool) int {
	if points <= 0 {
		return 0
	}
	if !known || len(m.PointLadder) == 0 {
		return 1
	}
	rank := 0
	for i, c := range m.PointLadder {
		if points >= c {
			rank = i + 1
		}
	}
	return rank
}

// Points is what a save stores for a rank: the cumulative cost after buying it (0 for rank 0). ok is false for a rank the module
// does not have.
func Points(m Module, rank int) (int, bool) {
	if rank == 0 {
		return 0, true
	}
	if rank < 0 || rank > len(m.PointLadder) || rank > m.MaxLevel {
		return 0, false
	}
	return m.PointLadder[rank-1], true
}

var (
	locRe = regexp.MustCompile(`^(XX_|LOC_[A-Za-z]+: )`)
	kind  = regexp.MustCompile(`^Skills\.([A-Za-z]+)\.`)
)

// DisplayName cleans the catalog's name of its placeholder prefixes ("XX_", "LOC_Ability: "); when there is no usable name the id's
// last part is used.
func DisplayName(m Module) string {
	n := strings.TrimSpace(locRe.ReplaceAllString(m.Name, ""))
	if n != "" {
		return n
	}
	if i := strings.LastIndex(m.ID, "."); i >= 0 {
		return m.ID[i+1:]
	}
	return m.ID
}

// Kind is the module's kind from its id: Ability, Perk, Attribute, Key, Spice or Science.
func Kind(id string) string {
	if m := kind.FindStringSubmatch(id); m != nil {
		return m[1]
	}
	return ""
}

// School is a skill school as the console lists it.
type School struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// Schools are the five playable schools, in the console's order.
var Schools = []School{{"Trooper", "Trooper"}, {"Mentat", "Mentat"}, {"Planetologist", "Planetologist"}, {"BeneGesserit", "Bene Gesserit"}, {"Swordmaster", "Swordmaster"}}

// Starter is a module and rank in a school's starter set.
type Starter struct {
	ID   string
	Rank int
}

// StarterSkills are the first Key skill and first ability of a school, as the console's "Restore Starter Skills" sets them.
func StarterSkills(school string) []Starter {
	switch school {
	case "Trooper":
		return []Starter{{"Skills.Key.Trooper1", 1}, {"Skills.Ability.CablePull", 1}}
	case "Mentat":
		return []Starter{{"Skills.Key.Mentat1", 1}, {"Skills.Ability.TurretSeeker", 1}}
	case "Planetologist":
		return []Starter{{"Skills.Key.Planetologist1", 1}, {"Skills.Ability.SuspensorPad", 1}}
	case "BeneGesserit":
		return []Starter{{"Skills.Key.BeneGesserit1", 1}, {"Skills.Ability.VoiceCompel", 1}}
	case "Swordmaster":
		return []Starter{{"Skills.Key.Swordmaster1", 1}, {"Skills.Ability.KneeCharge", 1}}
	}
	return nil
}

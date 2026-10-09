package skills

import "testing"

func TestRankFollowsTheLadderNotTheRawPoints(t *testing.T) {
	m, ok := Lookup("Skills.Ability.Hypersprint") // ladder 1, 3, 6
	if !ok || len(m.PointLadder) != 3 {
		t.Fatalf("hypersprint: %+v %v", m, ok)
	}
	for points, want := range map[int]int{0: 0, 1: 1, 2: 1, 3: 2, 5: 2, 6: 3, 9: 3} {
		if got := Rank(points, m, true); got != want {
			t.Errorf("%d points: rank %d, want %d", points, got, want)
		}
	}
	if Rank(9, Module{}, false) != 1 {
		t.Error("a module the catalog does not know claims one rank at most")
	}
}

func TestPointsAreTheCumulativeCostAndRoundTrip(t *testing.T) {
	for _, m := range All() {
		for r := 0; r <= m.MaxLevel; r++ {
			p, ok := Points(m, r)
			if !ok {
				t.Fatalf("%s rank %d has no points", m.ID, r)
			}
			if got := Rank(p, m, true); got != r {
				t.Fatalf("%s: rank %d stores %d points, which reads back as rank %d", m.ID, r, p, got)
			}
		}
		if _, ok := Points(m, m.MaxLevel+1); ok {
			t.Fatalf("%s has no rank %d", m.ID, m.MaxLevel+1)
		}
	}
}

func TestValidIDRejectsAnythingThatCouldBreakAJSONPath(t *testing.T) {
	for _, id := range []string{"Skills.Perk.BodyShots", "Skills.Ability.CablePull", "Skills.Key.Trooper1"} {
		if !ValidID(id) {
			t.Errorf("%s must be valid", id)
		}
	}
	for _, id := range []string{"", "Skills", `Skills.X")`, "Skills.X].Y", "Skills.X Y", "Skills..X", "skills.Perk.X", "Skills.A.B.C.D.E", "Skills.X\"", "Other.Perk.X"} {
		if ValidID(id) {
			t.Errorf("%q must be refused", id)
		}
	}
}

func TestStarterSkillsExistInTheCatalog(t *testing.T) {
	for _, s := range Schools {
		st := StarterSkills(s.Key)
		if len(st) != 2 {
			t.Fatalf("%s: %v", s.Key, st)
		}
		for _, x := range st {
			if _, ok := Lookup(x.ID); !ok {
				t.Errorf("starter skill %s is not in the catalog", x.ID)
			}
		}
	}
	if StarterSkills("Nope") != nil {
		t.Error("unknown school")
	}
}

func TestDisplayNameDropsPlaceholderPrefixes(t *testing.T) {
	if got := DisplayName(Module{ID: "Skills.Key.X1", Name: "XX_Bene Gesserit Phase 1"}); got != "Bene Gesserit Phase 1" {
		t.Errorf("%q", got)
	}
	if got := DisplayName(Module{ID: "Skills.Ability.Y", Name: "LOC_Ability: Healer Seeker"}); got != "Healer Seeker" {
		t.Errorf("%q", got)
	}
	if got := DisplayName(Module{ID: "Skills.Ability.Y", Name: ""}); got != "Y" {
		t.Errorf("%q", got)
	}
}

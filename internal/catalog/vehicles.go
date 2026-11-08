package catalog

import "regexp"

// The vehicle tables themselves (classes, slots, templates, module rows, the fuel tanks of the chassis) are in kit/gamedata/vehicles; the editor reads them from there. What stays
// here is the one thing that needs the item names: the in-game name of a module.

var leadingZeroGrade = regexp.MustCompile(`_0(\d)$`)
var plainGrade = regexp.MustCompile(`_(\d)$`)

// ModuleName is the in-game name of a module, from the item catalog (the item that installs it), or "" when the catalog has none. The item's id spells the grade of some modules
// otherwise than the module's row does, so both spellings are tried.
func ModuleName(id string) string {
	for _, base := range []string{id, locomotionPosition.ReplaceAllString(id, "Locomotion")} {
		for _, c := range []string{base, leadingZeroGrade.ReplaceAllString(base, "_$1"), plainGrade.ReplaceAllString(base, "_0$1")} {
			if n, ok := Name(c); ok {
				return n
			}
		}
	}
	return ""
}

// locomotionPosition matches the wheel or leg position in a locomotion module's id: the item that installs them is one item for all positions (BuggyLocomotion_6).
var locomotionPosition = regexp.MustCompile(`Locomotion(FrontLeft|FrontRight|BackLeft|BackRight|BackCenter|CenterLeft\d?|CenterRight\d?|Left|Right)`)

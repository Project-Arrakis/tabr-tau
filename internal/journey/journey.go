// Package journey names and orders the Journey nodes of a save the way the Dune Docker console does.
//
// journey-tags.json is a copy, unchanged, of runtime/data/journey-tags.json of
// https://github.com/Red-Blink/dune-awakening-selfhost-docker (MIT licence, copyright RedBlink; git blob 62bd7f08); see
// internal/notices/THIRD-PARTY-NOTICES.md. It gives readable names for story nodes and contracts, the order of sibling nodes, and
// the gameplay tags a node or contract sets. The naming, parent and ordering rules follow the console's
// console/api/src/duneDb/presentation.js.
//
// It is data and rules only: it never imports the save.
package journey

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

//go:embed journey-tags.json
var raw []byte

var cat struct {
	Aliases         map[string]string   `json:"journey_aliases"`
	Children        map[string][]string `json:"journey_children"`
	NodeTags        map[string][]string `json:"journey_node_tags"`
	ContractTags    map[string][]string `json:"contract_tags"`
	ContractAliases map[string]string   `json:"contract_aliases"` // short name -> full id
}

var contractShort map[string]string // full id -> short name

func init() {
	if err := json.Unmarshal(raw, &cat); err != nil {
		panic("journey: journey-tags.json is not valid: " + err.Error())
	}
	contractShort = make(map[string]string, len(cat.ContractAliases))
	for short, full := range cat.ContractAliases {
		contractShort[full] = short
	}
}

// Group says whether a node id is a contract or a story node.
func Group(id string) string {
	if strings.HasPrefix(id, "DA_CT_") || strings.HasPrefix(id, "DA_LDR_") {
		return "contract"
	}
	return "story"
}

var (
	prefixRe = regexp.MustCompile(`^(DA_|CT_|LDR_|FQ_|Dunipedia_)`)
	camel1   = regexp.MustCompile(`([a-z])([A-Z0-9])`)
	camel2   = regexp.MustCompile(`([0-9])([A-Z])`)
	spaces   = regexp.MustCompile(`\s+`)
	numberRe = regexp.MustCompile(`^[0-9]+$`)
)

// DisplayName is the readable name of a node: the catalog's alias when it has one, otherwise the last part of the id with the
// prefixes and underscores tidied.
func DisplayName(id string) string {
	if a := strings.TrimSpace(cat.Aliases[id]); a != "" {
		return a
	}
	// A step that is only a number ("Quest.Place Sensors.1") would read as "1" on its own, so it carries its parent's name.
	if i := strings.LastIndex(id, "."); i > 0 && numberRe.MatchString(id[i+1:]) {
		return DisplayName(id[:i]) + " " + id[i+1:]
	}
	return tidy(id)
}

func tidy(id string) string {
	last := id
	if i := strings.LastIndex(id, "."); i >= 0 {
		last = id[i+1:]
	}
	s := prefixRe.ReplaceAllString(last, "")
	s = strings.ReplaceAll(s, "_", " ")
	s = camel1.ReplaceAllString(s, "$1 $2")
	s = camel2.ReplaceAllString(s, "$1 $2")
	s = strings.TrimSpace(spaces.ReplaceAllString(s, " "))
	if s == "" {
		return id
	}
	return s
}

// ContractName is the readable name of a catalog contract id (DA_CT_...).
func ContractName(fullID string) string {
	short, ok := contractShort[fullID]
	if !ok {
		short = strings.TrimPrefix(fullID, "DA_CT_")
	}
	return tidy(short)
}

// Contracts lists the catalog's contract ids, sorted.
func Contracts() []string {
	out := make([]string, 0, len(cat.ContractAliases))
	for _, full := range cat.ContractAliases {
		if full != "" {
			out = append(out, full)
		}
	}
	sort.Strings(out)
	return out
}

// ContractTags are the player tags that mark a contract complete.
func ContractTags(fullID string) []string { return cat.ContractTags[fullID] }

// NodeTagCount is how many tags a node sets in the catalog.
func NodeTagCount(id string) int { return len(cat.NodeTags[id]) }

// KnownNodes lists the story nodes the catalog knows (names or tags).
func KnownNodes() []string {
	seen := map[string]bool{}
	var out []string
	for id := range cat.Aliases {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for id := range cat.NodeTags {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// Parent is the nearest ancestor of id that is in ids ("" if none): dots separate the levels of a node id.
func Parent(id string, ids map[string]bool) string {
	parts := strings.Split(id, ".")
	for len(parts) > 1 {
		parts = parts[:len(parts)-1]
		if p := strings.Join(parts, "."); ids[p] {
			return p
		}
	}
	return ""
}

// Depth is how many ancestors of id are in ids.
func Depth(id string, ids map[string]bool) int {
	d := 0
	for p := Parent(id, ids); p != ""; p = Parent(p, ids) {
		d++
	}
	return d
}

// Less orders node ids as the console does: a parent before its children, siblings in the catalog's order, then by name.
func Less(a, b string) bool {
	if a == b {
		return false
	}
	ap, bp := strings.Split(a, "."), strings.Split(b, ".")
	n := len(ap)
	if len(bp) < n {
		n = len(bp)
	}
	shared := 0
	for shared < n && ap[shared] == bp[shared] {
		shared++
	}
	if shared == n {
		return len(ap) < len(bp)
	}
	parent := strings.Join(ap[:shared], ".")
	siblings := cat.Children[parent]
	aID, bID := strings.Join(ap[:shared+1], "."), strings.Join(bp[:shared+1], ".")
	ai, bi := indexOf(siblings, aID), indexOf(siblings, bID)
	if ai >= 0 || bi >= 0 {
		if ai < 0 {
			return false
		}
		if bi < 0 {
			return true
		}
		if ai != bi {
			return ai < bi
		}
	}
	return aID < bID
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

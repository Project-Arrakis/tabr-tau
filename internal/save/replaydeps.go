package save

import "regexp"

// Which queued edits depend on one that could not be put back. The journal keeps statements, not intentions, so the dependence is judged from what is there: an edit depends on
// a failed one when it works on a table the failed one worked on and has a value in common with it, as an argument or in its text (the id of the row the failed edit was to add, the item it was to
// change). That errs towards skipping an edit that did not need to be skipped, which is reported and can be made again, rather than towards acting on the game's own row that
// happens to have the id.

// tableRef finds the tables a statement names: after INTO, UPDATE (with its OR clause), FROM and JOIN.
var tableRef = regexp.MustCompile(`(?i)\b(?:into|update(?:\s+or\s+\w+)?|from|join)\s+["` + "`" + `\[]?([A-Za-z_][A-Za-z0-9_]*)`)

func opTables(o opRec) map[string]bool {
	out := map[string]bool{}
	for _, st := range o.Stmts {
		for _, m := range tableRef.FindAllStringSubmatch(st.Q, -1) {
			out[lowerASCII(m[1])] = true
		}
	}
	return out
}

// literalRef finds the values written into a statement's text: quoted strings and numbers.
var literalRef = regexp.MustCompile(`'((?:[^']|'')*)'|\b(\d+(?:\.\d+)?)\b`)

// opValues is every value an edit names, by its text: its arguments and the literals in its statements (0 and 1 are left out).
func opValues(o opRec) map[string]bool {
	out := map[string]bool{}
	add := func(v string) {
		if v != "" && v != "0" && v != "1" { // flags and counts of one say nothing about which row an edit is about
			out[v] = true
		}
	}
	for _, st := range o.Stmts {
		for _, a := range st.A {
			if a.T != "n" {
				add(a.V)
			}
		}
		for _, m := range literalRef.FindAllStringSubmatch(st.Q, -1) {
			add(m[1] + m[2])
		}
	}
	return out
}

func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// dependsOnAny returns the description of the first of failed that o depends on, or "" when it depends on none of them.
func dependsOnAny(o opRec, failed []opRec) string {
	if len(failed) == 0 {
		return ""
	}
	tabs, vals := opTables(o), opValues(o)
	for _, f := range failed {
		shared := false
		for t := range opTables(f) {
			if tabs[t] {
				shared = true
				break
			}
		}
		if !shared {
			continue
		}
		for v := range opValues(f) {
			if vals[v] {
				return f.Desc
			}
		}
	}
	return ""
}

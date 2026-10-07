package diff

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// clean removes control characters (a hostile save can name a table or column with terminal escape sequences).
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

func compact(v any) string {
	if s, ok := v.(string); ok {
		return clean(s)
	}
	b, _ := json.Marshal(v)
	return clean(string(b))
}

// WriteText renders a result as plain text for the CLI.
func (r *Result) WriteText(w io.Writer) {
	if len(r.OnlyBefore)+len(r.OnlyAfter) > 0 {
		fmt.Fprintf(w, "TABLE SET DIFFERS: only-before %q only-after %q\n", r.OnlyBefore, r.OnlyAfter)
	}
	for _, t := range r.Tables {
		fmt.Fprintf(w, "\n## %s  (key=%v)  +%d -%d ~%d\n", clean(t.Name), t.Key, t.NumAdded, t.NumRemoved, t.NumModified)
		for _, x := range t.Added {
			fmt.Fprintf(w, "  + %v: %s\n", x.Key, compact(x.Data))
		}
		for _, x := range t.Removed {
			fmt.Fprintf(w, "  - %v: %s\n", x.Key, compact(x.Data))
		}
		for _, x := range t.Modified {
			fmt.Fprintf(w, "  ~ %v\n", x.Key)
			for _, c := range x.Changes {
				fmt.Fprintf(w, "      %s %s: %s -> %s\n", c.Kind, clean(c.Path), compact(c.Before), compact(c.After))
			}
		}
		if n := t.NumAdded - len(t.Added); n > 0 {
			fmt.Fprintf(w, "  ... %d more + rows not shown\n", n)
		}
		if n := t.NumRemoved - len(t.Removed); n > 0 {
			fmt.Fprintf(w, "  ... %d more - rows not shown\n", n)
		}
		if n := t.NumModified - len(t.Modified); n > 0 {
			fmt.Fprintf(w, "  ... %d more ~ rows not shown\n", n)
		}
	}
	suffix := ""
	if !r.Redacted {
		suffix = "  (UNREDACTED)"
	}
	fmt.Fprintf(w, "\n%d table(s) changed%s\n", len(r.Tables), suffix)
}

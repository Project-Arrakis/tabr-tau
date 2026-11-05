package basefile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The strict check of a console file (#538). The console's import rewrites every string value that is an id of the form "!!act@12" into a new local id, and it refuses a string that is
// an id of any other form ("Non-transfer id !!itm#0 is found at .properties...."). The editor's own reader is kinder than that (it takes "!!act#12" as an id of another save and maps
// it), so a file can pass the reader and still be refused by the console. This check applies the console's rule to the finished text, on top of the reader:
//
//   - every string value of every entry (and every key) that holds an id mark "!!<kind>@" or "!!<kind>#" is, as a whole, "!!<kind>@<number>";
//   - the number is 0 (the null reference, which the console's import turns back into "no id"), or the transfer id of an entry of that kind in the file, or, for an actor, the id of
//     the owner placeholder.
//
// What the console really does with such an id is in its game database functions, which are not in the console's own code: the rule above is read from what the console's tests and its
// stand-ins for those functions do (see docs/export-a-base-for-the-console.md). It is deliberately no looser than that.

// idMark is a string that starts an id in either form; markedWhole is the one form the console takes.
var (
	idMark      = regexp.MustCompile(`!![a-z]{3}[#@]`)
	markedWhole = regexp.MustCompile(`^!!([a-z]{3})@([0-9]+)$`)
)

// maxStrictProblems is how many problems are listed in the message; the rest are counted.
const maxStrictProblems = 5

// ConsoleStrict is what the strict check counted in a file it accepted.
type ConsoleStrict struct {
	Entries     int // entries read
	Refs        int // ids found inside the JSON of the entries, the null reference included
	NoneRefs    int // of those, the null reference "@0"
	Placeholder int // of those, the ids of the owner placeholder
}

// CheckConsoleStrict applies the console's rule for ids to a finished console file. It does not look at anything else; Load does. The error is plain and names the first problems with
// the entry and the place in it.
func CheckConsoleStrict(data []byte) (*ConsoleStrict, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var f struct {
		Placeholder json.Number `json:"ownerPlaceholderTransferId"`
		Entries     []struct {
			ID   json.Number `json:"id"`
			Kind string      `json:"kind"`
			Data any         `json:"data"`
		} `json:"entries"`
	}
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("the file is not JSON the strict check can read: %w", friendly(err))
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("there is more after the first JSON object")
	}
	ph, err := f.Placeholder.Int64()
	if err != nil || ph <= 0 {
		return nil, fmt.Errorf("the file has no owner placeholder id")
	}
	// the transfer ids by kind of entry: an id "!!itm@7" names the entry of kind "itm" whose id is 7
	have := map[string]map[int64]bool{}
	for _, e := range f.Entries {
		id, err := e.ID.Int64()
		if err != nil {
			return nil, fmt.Errorf("an entry of kind %q has an id that is not a whole number", clip(e.Kind))
		}
		if have[e.Kind] == nil {
			have[e.Kind] = map[int64]bool{}
		}
		have[e.Kind][id] = true
	}
	res := &ConsoleStrict{Entries: len(f.Entries)}
	var problems []string
	nProblems := 0
	for _, e := range f.Entries {
		id, _ := e.ID.Int64()
		where := fmt.Sprintf("entry %d (%s)", id, clip(e.Kind))
		bad := func(path, msg string) {
			if nProblems++; nProblems <= maxStrictProblems {
				problems = append(problems, fmt.Sprintf("%s holds %s at %s", where, msg, path))
			}
		}
		var check func(v any, path string)
		str := func(s, path string) {
			if !strings.Contains(s, "!!") || !idMark.MatchString(s) {
				return
			}
			m := markedWhole.FindStringSubmatch(s)
			if m == nil {
				if loc := idMark.FindStringIndex(s); loc[0] == 0 && len(s) <= 40 && !strings.ContainsAny(s, " \t\r\n") {
					bad(path, fmt.Sprintf("the non-transfer id %s", s))
				} else {
					bad(path, "an id inside other text")
				}
				return
			}
			n, err := strconv.ParseInt(m[2], 10, 64)
			res.Refs++
			switch {
			case err != nil:
				bad(path, fmt.Sprintf("the id %s, whose number does not fit", s))
			case n == 0:
				res.NoneRefs++
			case m[1] == "act" && n == ph:
				res.Placeholder++
			case !have[m[1]][n]:
				bad(path, fmt.Sprintf("the id %s, which names no %s entry of the file", s, m[1]))
			}
		}
		check = func(v any, path string) {
			switch x := v.(type) {
			case string:
				str(x, path)
			case []any:
				for _, c := range x {
					check(c, path+".*")
				}
			case map[string]any:
				keys := make([]string, 0, len(x))
				for k := range x {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					str(k, path+"."+clip(k))
					check(x[k], path+"."+k)
				}
			}
		}
		check(e.Data, "")
	}
	if nProblems > 0 {
		msg := strings.Join(problems, "; ")
		if nProblems > maxStrictProblems {
			msg += fmt.Sprintf("; and %d more", nProblems-maxStrictProblems)
		}
		return nil, fmt.Errorf("the console would refuse the file: %s", msg)
	}
	return res, nil
}

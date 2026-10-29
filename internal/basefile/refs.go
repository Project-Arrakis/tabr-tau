package basefile

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// The ids the game writes inside JSON as strings: "!!act#12" (an actor), "!!itm#7" (an item), "!!inv#3" (an inventory), "!!fgl#..." (an entity), "!!bbp#..." (a blueprint). #0 is the null
// reference. A base carries a few: a totem's own id, a crafting station's item ids, a queued recipe's reference to the character. When the base goes into another save every one has to
// name the new row, and none may be left naming a row of the save the file came from.

// The string of a reference, anchored: the whole string is the reference.
var refString = regexp.MustCompile(`^!!([a-z]{3})#(-?[0-9]+)$`)

// A reference-looking mark inside a longer string: something this editor cannot rewrite safely.
var refInside = regexp.MustCompile(`!![a-z]{3}#`)

// The console's own file (console.go) writes a reference as "!!act@12", with a transfer id after the @; a file of the console can also hold "!!act#12", an id of the server it came
// from. Both are read by RewriteMarked.
var (
	markedString = regexp.MustCompile(`^!!([a-z]{3})([#@])(-?[0-9]+)$`)
	markedInside = regexp.MustCompile(`!![a-z]{3}[#@]`)
)

// MaxJSONDepth is how deeply a JSON cell may nest. The game's own components nest a handful of levels; SQLite's JSON functions stop at 1,000.
const MaxJSONDepth = 100

// Ref is one reference found in a JSON text.
type Ref struct {
	Kind string // "act", "itm", ...
	ID   int64
	Sep  byte // '#' for an id of a save, '@' for a transfer id of the console's file
}

// RewriteRefs returns text with every string that is a whole reference replaced by what fn answers for it. Everything else in the text, to the byte, is left as it was (the order
// of keys, the spelling of numbers, a 64-bit number above 2^53), because the text is cut and joined, never parsed and written again. text must be valid JSON; a string that holds a
// reference inside a longer text, a nesting deeper than MaxJSONDepth, or a reference whose number does not fit 64 bits is an error. fn returns the whole replacement string, such as
// "!!act#40", or an error to stop.
func RewriteRefs(text string, fn func(Ref) (string, error)) (string, error) {
	return rewriteRefs(text, refString, refInside, 2, fn)
}

// RewriteMarked is RewriteRefs for a text that may hold references of both forms, "!!act#12" and "!!act@12"; fn is told which one it has (Ref.Sep).
func RewriteMarked(text string, fn func(Ref) (string, error)) (string, error) {
	return rewriteRefs(text, markedString, markedInside, 3, fn)
}

func rewriteRefs(text string, whole, inside *regexp.Regexp, numGroup int, fn func(Ref) (string, error)) (string, error) {
	var out strings.Builder
	depth := 0
	last := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '{', '[':
			if depth++; depth > MaxJSONDepth {
				return "", fmt.Errorf("a JSON value is nested more than %d levels deep", MaxJSONDepth)
			}
		case '}', ']':
			depth--
		case '"':
			j := i + 1
			esc := false
			for ; j < len(text); j++ {
				if text[j] == '\\' {
					esc = true
					j++
					continue
				}
				if text[j] == '"' {
					break
				}
			}
			if j >= len(text) {
				return "", fmt.Errorf("a JSON string is not closed")
			}
			s := text[i+1 : j]
			if esc {
				if err := json.Unmarshal([]byte(text[i:j+1]), &s); err != nil {
					return "", fmt.Errorf("a JSON string cannot be read")
				}
			}
			if strings.Contains(s, "!!") {
				m := whole.FindStringSubmatch(s)
				switch {
				case m != nil:
					n, err := strconv.ParseInt(m[numGroup], 10, 64)
					if err != nil {
						return "", fmt.Errorf("the reference %q has a number that does not fit", clip(s))
					}
					sep := byte('#')
					if numGroup == 3 {
						sep = m[2][0]
					}
					repl, err := fn(Ref{Kind: m[1], ID: n, Sep: sep})
					if err != nil {
						return "", err
					}
					out.WriteString(text[last : i+1])
					out.WriteString(repl)
					last = j
				case inside.MatchString(s):
					return "", fmt.Errorf("the text %q holds a reference to an id inside other text, which cannot be rewritten safely", clip(s))
				}
			}
			i = j
		}
	}
	if last == 0 {
		return text, nil
	}
	out.WriteString(text[last:])
	return out.String(), nil
}

// FindRefs lists the references in a JSON text, in order, by the same reading as RewriteRefs.
func FindRefs(text string) ([]string, error) {
	var found []string
	_, err := RewriteRefs(text, func(r Ref) (string, error) {
		s := "!!" + r.Kind + "#" + strconv.FormatInt(r.ID, 10)
		found = append(found, s)
		return s, nil
	})
	return found, err
}

// Package hygiene is the public-text rule as code: public text (the tracked files, and the titles and bodies of issues, pull requests and
// comments) carries no reserved word, no path under a personal profile and no AI attribution line. Reserved words are kept only as SHA-256
// digests, so this package does not spell what it keeps out, and a report names the place of a hit and never the text of it.
package hygiene

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// Kind says which rule a hit broke.
type Kind string

const (
	// KindReserved is a reserved word (a word, or up to four words in a row, whose digest is in the list).
	KindReserved Kind = "reserved word"
	// KindProfilePath is a path under a personal profile folder that names an account.
	KindProfilePath Kind = "path under a personal profile"
	// KindAttribution is an AI co-author trailer or attribution line.
	KindAttribution Kind = "AI attribution line"
)

// Hit is one place that breaks the rule. Text is the offending text; callers that print to a public log must print Kind and Line only.
type Hit struct {
	Kind Kind
	Line int
	Text string
}

// reservedDigests are SHA-256 digests of lower-case reserved words (audit 2026-11-13, D11-1). A pair of words is hashed as the two words,
// lower-case, joined by one space. The values carry no label on purpose: a label would say what the list keeps out.
var reservedDigests = []string{
	"f3d5bd2b82bcb38fd930d9baeb8ca4438470c53b3e31b70f947c85fff36f5e72",
	"9d0b5d30f807e1081a0a75a3e5fa0b29ee086315ad3d42e963a4782b8ef2abfe",
	"bb5865066104da838c54688c4450dd4069e65b065048f59e5a31fab81e228180",
	"463bbf36edbb88cc27a7138ee858b33da2874e02f1038581a1ce7698d069eb4f",
	"603804cfd7a43745f9dc3053241ba70591aa5784d455fad1c40233dcb191e8e2",
}

// maxWords is the most words in a row that one digest may stand for.
const maxWords = 4

// Digest returns the digest of a word or of words in a row, the form the lists hold. It lower-cases and joins the words by one space.
func Digest(words ...string) string {
	return digestOf(strings.ToLower(strings.Join(words, " ")))
}

// Checker holds the digests to look for. The zero value is not usable; use New.
type Checker struct {
	digests map[string]bool
}

// New returns a Checker over the built-in digests and the extra ones given (hex digests separated by commas, spaces or new lines; an
// entry that is not 64 hex characters is ignored). The extra list is how a private list reaches CI without being in the repository.
func New(extra string) *Checker {
	c := &Checker{digests: map[string]bool{}}
	for _, d := range reservedDigests {
		c.digests[d] = true
	}
	for _, d := range strings.FieldsFunc(extra, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\r' || r == '\t' }) {
		d = strings.ToLower(strings.TrimSpace(d))
		if len(d) == 64 && isHex(d) {
			c.digests[d] = true
		}
	}
	return c
}

// NewWithDigests returns a Checker over exactly the digests given (for tests, which use words of their own).
func NewWithDigests(digests ...string) *Checker {
	c := &Checker{digests: map[string]bool{}}
	for _, d := range digests {
		c.digests[strings.ToLower(d)] = true
	}
	return c
}

func isHex(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// A word is a run of letters, digits, hyphens and underscores, so a file name with an extension or a possessive is caught by its stem.
var wordRE = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9_-]*`)

// A path under a personal profile: a drive path under Users, or a Unix path under Users or home, then the account name. A placeholder
// (<name>, %USERPROFILE%, $HOME) has no account-name characters after the folder, so it does not match.
var (
	winProfileRE  = regexp.MustCompile(`(?i)\b[a-z]:[\\/]+users[\\/]+([^\\/\s"'<>|*?%$` + "`" + `(){}\[\],;]+)`)
	unixProfileRE = regexp.MustCompile(`(?i)(?:^|[\s"'(=` + "`" + `])/(?:users|home)/([^/\s"'<>|*?%$` + "`" + `(){}\[\],;]+)`)
)

// placeholders are account names that are not an account: generic examples and the names of build machines' accounts.
var placeholders = map[string]bool{
	"user": true, "username": true, "you": true, "yourname": true, "your-name": true, "your_name": true, "name": true, "me": true,
	"public": true, "default": true, "all": true, "runner": true, "runneradmin": true, "example": true, "foo": true, "someone": true,
	"account": true, "profile": true, "person": true, "player": true, "current": true, "x": true, "...": true, "..": true, ".": true,
	"testuser": true, "tester": true, "alice": true, "bob": true, "a": true, "first": true, "jane": true, "john": true,
}

// attributionRE is the commit-message rule of scripts/check-commit-messages.sh: an AI assistant or vendor as co-author, or a
// "generated with" line.
var attributionRE = regexp.MustCompile(`(?i)^(co-authored-by:.*(claude|anthropic|openai|chatgpt|copilot|gemini|cursor)|.*generated (with|by) .*(claude|chatgpt|copilot|gemini))`)

// Check returns the hits in s: reserved words and phrases, and profile paths.
func (c *Checker) Check(s string) []Hit {
	var hits []Hit
	for i, line := range strings.Split(s, "\n") {
		lineNo := i + 1
		words := wordRE.FindAllString(line, -1)
		for j := range words {
			for n := 1; n <= maxWords && j+n <= len(words); n++ {
				if c.digests[Digest(words[j:j+n]...)] {
					hits = append(hits, Hit{KindReserved, lineNo, strings.Join(words[j:j+n], " ")})
				}
			}
		}
		for _, re := range []*regexp.Regexp{winProfileRE, unixProfileRE} {
			for _, m := range re.FindAllStringSubmatch(line, -1) {
				if !placeholders[strings.ToLower(m[1])] {
					hits = append(hits, Hit{KindProfilePath, lineNo, m[0]})
				}
			}
		}
	}
	return hits
}

// CheckAttribution returns the lines of s that are an AI co-author trailer or an AI attribution line.
func CheckAttribution(s string) []Hit {
	var hits []Hit
	for i, line := range strings.Split(s, "\n") {
		if attributionRE.MatchString(strings.TrimSpace(line)) {
			hits = append(hits, Hit{KindAttribution, i + 1, line})
		}
	}
	return hits
}

func digestOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

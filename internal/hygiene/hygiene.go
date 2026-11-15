// Package hygiene is the public-text rule as code: public text (the tracked files, and the titles and bodies of issues, pull requests and
// comments) carries no reserved word, no path under a personal profile and no AI attribution line. Reserved words are kept only as SHA-256
// digests, so this package does not spell what it keeps out, and a report names the place of a hit and never the text of it.
package hygiene

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"unicode"
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

// joiners are what a run of words is joined with before it is hashed: the form a list holds (a space between the words of a phrase, a hyphen
// inside a hyphenated name) and the ways the same words are written otherwise (an underscore, or nothing at all).
var joiners = [...]string{" ", "-", "_", ""}

// A path under a personal profile: the users or home folder, then the account name, written after whatever starts a path. A placeholder
// (<name>, %USERPROFILE%, $HOME) has no account-name characters after the folder, so it does not match.
//
// The forms are: a drive path (C:\Users\NAME, C:/Users/NAME); a Unix path (/home/NAME, /Users/NAME) after the start of the text or after any
// character that is not part of a word or of a longer path, so after a colon (a search path), a bracket or a greater-than sign as well as a
// space, a quote or an equals sign, and after file:// (a stack frame); the POSIX-shell forms of a drive path (/c/Users/NAME, /mnt/c/Users/NAME,
// /cygdrive/c/Users/NAME); and a network path (\\host\Users\NAME, \\host\C$\Users\NAME, \\host\Ubuntu\home\NAME). A web address is not matched
// because the host's last character is a word character in front of its /users/ or /home/ segment, never a character that may start a path.
const (
	nameClass = `[^\\/\s"'<>|*?%$:` + "`" + `(){}\[\],;]+` // the characters of one folder name (an account name, a host)
	hostClass = `[^\\/\s"'<>|*?%:` + "`" + `(){}\[\],;]+`  // a host or a share: as a name, but a dollar sign is allowed (a WSL host, a C$ share)
	nameChars = `(` + nameClass + `)`
)

var (
	winProfileRE  = regexp.MustCompile(`(?i)\b[a-z]:[\\/]+users[\\/]+` + nameChars)
	uncProfileRE  = regexp.MustCompile(`(?i)\\\\` + hostClass + `\\(?:[a-z]\$\\|` + hostClass + `\\)?(?:users|home)\\+` + nameChars)
	unixProfileRE = regexp.MustCompile(`(?i)(?:^|[^\pL\pN_.~%$@/\\)\]}-]|file:/{2}(?:localhost)?)/+(?:(?:(?:mnt|cygdrive)/)?[a-z]/users|users|home)/+` + nameChars)
)

// placeholders are account names that are not an account: generic examples and the names of build machines' accounts. A first name is not
// one: a profile path of an account with that name is a path under a personal profile.
var placeholders = map[string]bool{
	"user": true, "username": true, "you": true, "yourname": true, "your-name": true, "your_name": true, "name": true, "me": true,
	"public": true, "default": true, "all": true, "runner": true, "runneradmin": true, "example": true, "foo": true, "someone": true,
	"account": true, "profile": true, "person": true, "player": true, "current": true, "x": true, "...": true, "..": true, ".": true,
	"testuser": true, "tester": true, "a": true, "first": true,
}

// attributionRE is the commit-message rule of scripts/check-commit-messages.sh: an AI assistant or vendor as co-author, or a
// "generated with" line.
var attributionRE = regexp.MustCompile(`(?i)^(co-authored-by:.*(claude|anthropic|openai|chatgpt|copilot|gemini|cursor)|.*generated (with|by) .*(claude|chatgpt|copilot|gemini))`)

// fold returns s with the forms that hide a word folded away: combining marks are dropped, every dash and the minus sign become a hyphen, the
// full-width forms of the ASCII characters become the ASCII characters, and a format character (a zero-width space or joiner, the soft hyphen,
// a direction mark, the byte-order mark) becomes hidden. Line ends are kept, so lines are the same lines before and after. It is not full NFKC
// and it has no table of look-alike letters: a letter of another script that only looks like a Latin one is not mapped.
//
// The words are looked for in the text folded with hidden = " ": a format character where a hyphen or a space was is then a word break, and
// one inside a word still joins its two halves (the checker joins the parts of a word). The paths are looked for with hidden = "", so one
// inside the folder name or the account name does not break the pattern.
func fold(s, hidden string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteRune(r)
		case unicode.Is(unicode.Cf, r):
			b.WriteString(hidden)
		case unicode.Is(unicode.Mn, r):
		case r >= 0xFF01 && r <= 0xFF5E:
			b.WriteRune(r - 0xFEE0)
		case r == '\u2212' || unicode.Is(unicode.Pd, r):
			b.WriteByte('-')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// token is a word of the text as written, with the line it is on and its parts (the runs between its hyphens and underscores).
type token struct {
	raw   string
	line  int
	parts []string
}

func tokens(s string) []token {
	locs := wordRE.FindAllStringIndex(s, -1)
	toks := make([]token, len(locs))
	line, prev := 1, 0
	for k, l := range locs {
		line += strings.Count(s[prev:l[0]], "\n")
		prev = l[0]
		raw := s[l[0]:l[1]]
		toks[k] = token{raw, line, strings.FieldsFunc(raw, func(r rune) bool { return r == '-' || r == '_' })}
	}
	return toks
}

// reserved finds the reserved words and phrases of the whole text (normalized), over line ends. Two views of the words are tried: the parts of
// every word one by one (so a name with a part added, or with a space or markup where its hyphen was, is found) and, when a word has parts,
// the words as written. Every run of up to maxWords is joined in each of the joiners.
func (c *Checker) reserved(s string) []Hit {
	toks := tokens(s)
	type atom struct {
		text string
		line int
	}
	var atoms []atom
	multi := false
	for _, t := range toks {
		if len(t.parts) > 1 {
			multi = true
		}
		for _, p := range t.parts {
			atoms = append(atoms, atom{p, t.line})
		}
	}
	var hits []Hit
	seen := map[Hit]bool{}
	try := func(words []string, line int) {
		for ji, j := range joiners {
			if ji > 0 && len(words) == 1 {
				return
			}
			if c.digests[digestOf(strings.ToLower(strings.Join(words, j)))] {
				h := Hit{KindReserved, line, strings.Join(words, " ")}
				if !seen[h] {
					seen[h] = true
					hits = append(hits, h)
				}
				return
			}
		}
	}
	words := make([]string, 0, maxWords)
	for i := range atoms {
		words = words[:0]
		for n := 0; n < maxWords && i+n < len(atoms); n++ {
			words = append(words, atoms[i+n].text)
			try(words, atoms[i].line)
		}
	}
	if multi {
		for i := range toks {
			words = words[:0]
			for n := 0; n < maxWords && i+n < len(toks); n++ {
				words = append(words, toks[i+n].raw)
				try(words, toks[i].line)
			}
		}
	}
	return hits
}

// Check returns the hits in s: reserved words and phrases, and profile paths. The text is folded first (see fold), and a phrase may run over
// a line end; the hit is on the line of its first word.
func (c *Checker) Check(s string) []Hit {
	hits := c.reserved(fold(s, " "))
	for i, line := range strings.Split(fold(s, ""), "\n") {
		named := map[string]bool{} // one hit for an account name on a line, whichever pattern saw it first (a drive path is also seen after its colon)
		for _, re := range []*regexp.Regexp{winProfileRE, uncProfileRE, unixProfileRE} {
			for _, m := range re.FindAllStringSubmatch(line, -1) {
				if name := strings.ToLower(m[1]); !placeholders[name] && !named[name] {
					named[name] = true
					hits = append(hits, Hit{KindProfilePath, i + 1, m[0]})
				}
			}
		}
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].Line < hits[b].Line })
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

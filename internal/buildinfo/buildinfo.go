// Package buildinfo holds the version of this build: a SemVer version, a build counter, the short commit and the commit date.
//
// The four variables are set by the linker (scripts/buildinfo.ps1 prints the flags) and are never written by the program. A plain go build, go run or go test leaves them
// empty, and the version then reads "dev" with the commit that Go itself records in the binary, when it is there. The values are shown on the page and printed by --version, so
// anything that does not have the expected shape is thrown away rather than shown.
package buildinfo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
)

// Set by the linker with -X (see scripts/buildinfo.ps1). The names are pinned by a test that builds the program with a stamp and runs --version.
var (
	Version string // "0.14.2", or "0.14.2+5" five commits after the tag, with ".dirty" added for a tree with local changes
	Build   string // the number of commits that lead to the built commit, "317"
	Commit  string // the first characters of the commit id, "a1b2c3d"
	Date    string // the commit date, "2026-11-16"
)

// Channel is the only release channel there is.
const Channel = "stable"

// DevVersion is what the version reads when nothing was stamped.
const DevVersion = "dev"

var (
	versionRE = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)
	buildRE   = regexp.MustCompile(`^[0-9]{1,9}$`)
	commitRE  = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	dateRE    = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
)

// Info is the version of the running build, with every field either valid or empty.
type Info struct {
	Version string // "0.14.2", "0.14.2+5" or "dev"
	Build   string // "317", or "" when not stamped
	Commit  string // 7 characters, or "" when unknown
	Date    string // "2026-11-16", or "" when not stamped
	Channel string // always "stable"
	Arch    string // for example "windows/amd64"
	Dev     bool   // not exactly a tagged release: unstamped, commits after the tag, or a tree with local changes
}

// Get is the version of this build.
func Get() Info {
	var vcs string
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" {
				vcs = s.Value
			}
		}
	}
	return resolve(Version, Build, Commit, Date, vcs, runtime.GOOS+"/"+runtime.GOARCH)
}

// resolve checks the stamped values and fills in what is missing. vcs is the revision Go recorded in the binary, used for the commit only when none was stamped.
func resolve(version, build, commit, date, vcs, arch string) Info {
	i := Info{Channel: Channel, Arch: arch}
	if versionRE.MatchString(version) && len(version) <= 64 {
		i.Version = version
	}
	if buildRE.MatchString(build) {
		i.Build = build
	}
	if commitRE.MatchString(commit) {
		i.Commit = commit[:7]
	} else if commitRE.MatchString(strings.ToLower(vcs)) {
		i.Commit = strings.ToLower(vcs)[:7]
	}
	if dateRE.MatchString(date) {
		i.Date = date
	}
	if i.Version == "" {
		i.Version = DevVersion
	}
	i.Dev = IsDev(i.Version)
	return i
}

// IsDev says whether a version is anything but a plain release: the word dev, build metadata after the tag (commits since it, or a dirty tree) or a pre-release part.
func IsDev(version string) bool {
	return version == DevVersion || version == "" || strings.ContainsAny(version, "+-")
}

// Label is the short text of the header: "v0.14.2", "v0.14.2+5", or "dev".
func (i Info) Label() string {
	if i.Version == DevVersion || i.Version == "" {
		return DevVersion
	}
	return "v" + i.Version
}

// Line is the one line that names the build: "v0.14.2 (build 317, a1b2c3d)". Parts that are not known are left out, and an unstamped build is "dev (a1b2c3d)" or "dev".
func (i Info) Line() string {
	var parts []string
	if i.Build != "" {
		parts = append(parts, "build "+i.Build)
	}
	if i.Commit != "" {
		parts = append(parts, i.Commit)
	}
	if len(parts) == 0 {
		return i.Label()
	}
	return fmt.Sprintf("%s (%s)", i.Label(), strings.Join(parts, ", "))
}

// notesMax is the most of a notes file that is read, and the most that is shown.
const notesMax = 64 << 10

// ReadNotes returns the release notes kept next to the editor, the text of notes.md in dir, or "" when there is none. It reads a plain file only (not a link, not a folder), at
// most 64 KiB, and gives back text with the control characters taken out; nothing is written, and nothing is fetched from anywhere. truncated says that the file was longer.
func ReadNotes(dir string) (text string, truncated bool) {
	if dir == "" {
		return "", false
	}
	p := filepath.Join(dir, "notes.md")
	fi, err := os.Lstat(p)
	if err != nil || !fi.Mode().IsRegular() {
		return "", false
	}
	f, err := os.Open(p)
	if err != nil {
		return "", false
	}
	defer f.Close()
	buf, _ := io.ReadAll(io.LimitReader(f, notesMax+1))
	if len(buf) > notesMax {
		buf, truncated = buf[:notesMax], true
	}
	return clean(buf), truncated
}

// clean keeps valid UTF-8 text with tabs and line breaks (CRLF becomes LF) and drops every other control character, so the notes are plain text on the page.
func clean(b []byte) string {
	s := strings.ToValidUTF8(string(b), "�")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var out strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			out.WriteRune(r)
		case r < 0x20 || (r >= 0x7f && r < 0xa0) || r == 0x2028 || r == 0x2029 || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069):
			// control and bidirectional-override characters are dropped
		default:
			out.WriteRune(r)
		}
	}
	return strings.TrimSpace(out.String())
}

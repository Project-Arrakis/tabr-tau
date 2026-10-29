package save

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Which file is the game's own (#498). The game-running guard protects a file the game could have loaded. A save somewhere else (a copy kept for testing, a backup opened
// by hand) is not a file the game opens, so waiting for the game to close protects nothing there and only blocks the person. The game's own files are everything under
//
//	%LOCALAPPDATA%\DuneSandbox\Saved\Cloud\PlayerClientStorage\<service>\<id>\   (game.db, game_prepatch.db, game-1.db, the autosave ring)
//
// the tree Discover walks. The question is answered for the path in every spelling it can have, and whenever the answer is not clear it is "yes, the game's own": a wrong
// "yes" costs the wait the editor always had, a wrong "no" writes into a file the game may be holding.
//
// A path is the game's own when ANY of these holds:
//   - its spelling (made absolute and cleaned, with \\?\ taken off, ".." and "." resolved, any case, / or \) lies under the tree, or contains the same
//     Saved\Cloud\PlayerClientStorage\<something> layout anywhere (the game run by another user, a redirected profile);
//   - the same, after links, junctions and 8.3 short names are resolved (filepath.EvalSymlinks; for a file that does not exist yet, its nearest existing folder);
//   - it is the very same file as one of the game's files (a hard link);
//   - the folder of the tree cannot be told (LOCALAPPDATA is not set), the path cannot be made absolute, it is a device path, or it still has an unresolved 8.3 short name.
//
// Folders are compared name by name, so "PlayerClientStorage2" next to the tree is not under it.

// ownSaveFor is a variable so tests can stand in for the machine: the test binaries save in temp folders and treat every save as the game's own, as before this rule
// existed (see testhook.go); a test of the rule itself puts the real one back.
var ownSaveFor = gamesOwnSave

// IsGamesOwnSave says whether the file at path is one the game could have loaded (see above). It is false only for a file that is clearly somewhere else.
func IsGamesOwnSave(path string) bool { return ownSaveFor(path) }

// shortName matches an 8.3 short name component such as DUNESA~1 or GAME~1.DB.
var shortName = regexp.MustCompile(`^[^~]{1,8}~[0-9]+(\.[^.]{0,3})?$`)

func gamesOwnSave(path string) bool {
	if strings.TrimSpace(path) == "" {
		return true
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return true
	}
	forms := []string{abs}
	resolved, rest, ok := resolveExisting(abs)
	if ok && resolved != abs {
		forms = append(forms, resolved)
	}
	if !ok { // nothing of the path could be resolved
		rest = abs
	}
	// a short name in the part of the path that does not exist (or cannot be resolved) may be a spelling of a folder of the tree
	for _, c := range strings.FieldsFunc(rest, func(r rune) bool { return r == '/' || r == '\\' }) {
		if shortName.MatchString(c) {
			return true
		}
	}
	local := os.Getenv("LOCALAPPDATA")
	if strings.TrimSpace(local) == "" {
		return true
	}
	tree := filepath.Join(local, "DuneSandbox", "Saved", "Cloud", "PlayerClientStorage")
	roots := []string{tree}
	if real, _, ok := resolveExisting(tree); ok && real != tree {
		roots = append(roots, real)
	}
	for _, f := range forms {
		parts, odd := nameParts(f)
		if odd || hasGameLayout(parts) {
			return true
		}
		for _, r := range roots {
			rp, _ := nameParts(r)
			if isUnder(parts, rp) {
				return true
			}
		}
	}
	return sameFileAsGames(abs, roots)
}

// resolveExisting is filepath.EvalSymlinks of p, or, when p does not exist, of its nearest existing folder with the rest of the name put back (rest says what was put
// back). false when nothing could be resolved.
func resolveExisting(p string) (real, rest string, ok bool) {
	cur := p
	for i := 0; i < 64; i++ {
		if r, err := filepath.EvalSymlinks(cur); err == nil {
			if rest == "" {
				return r, "", true
			}
			return filepath.Join(r, rest), rest, true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", "", false
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
	return "", "", false
}

// nameParts splits a path into lower-case folder names, with / and \ both separators, a \\?\ prefix taken off, and "." and ".." resolved by spelling. odd is set for a path
// the rules cannot judge (a device path, a volume given by its GUID).
func nameParts(p string) (parts []string, odd bool) {
	s := strings.ReplaceAll(p, `\`, "/")
	switch {
	case strings.HasPrefix(s, "//./"):
		return nil, true
	case strings.HasPrefix(s, "//?/"):
		s = s[len("//?/"):]
	}
	for _, e := range strings.Split(s, "/") {
		switch e {
		case "", ".":
		case "..":
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
		default:
			parts = append(parts, strings.ToLower(e))
		}
	}
	if len(parts) > 0 && (strings.HasPrefix(parts[0], "volume{") || parts[0] == "globalroot") {
		return parts, true
	}
	return parts, false
}

// isUnder says whether parts lie below root, folder by folder.
func isUnder(parts, root []string) bool {
	if len(root) == 0 || len(parts) < len(root) {
		return false
	}
	for i, r := range root {
		if parts[i] != r {
			return false
		}
	}
	return true
}

// hasGameLayout says whether the names run ...\Saved\Cloud\PlayerClientStorage\<something>, wherever that sits.
func hasGameLayout(parts []string) bool {
	for i := 0; i+3 < len(parts); i++ {
		if parts[i] == "saved" && parts[i+1] == "cloud" && parts[i+2] == "playerclientstorage" {
			return true
		}
	}
	return false
}

// sameFileAsGames says whether the file at path is the same file as one of the game's saves in the tree (a hard link has another name and the same data).
func sameFileAsGames(path string, roots []string) bool {
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	for _, r := range roots {
		for _, pat := range []string{filepath.Join(r, "*", "*", "game*.db"), filepath.Join(r, "*", "*", "autosave", "*")} {
			m, _ := filepath.Glob(pat)
			if len(m) > 200 {
				m = m[:200]
			}
			for _, g := range m {
				if gi, err := os.Stat(g); err == nil && os.SameFile(fi, gi) {
					return true
				}
			}
		}
	}
	return false
}

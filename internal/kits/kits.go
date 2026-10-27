// Package kits is the editor's care package (#417): a named kit of items, XP and skill points that is kept in the editor's own settings file, so it survives a new save, and
// is applied to a character in one edit. A kit remembers nothing about any save. This file is the model: what a kit may hold, the checks every kit passes before it is kept or
// applied, the list operations (put, delete, duplicate) and the small JSON file kits are exported to and imported from. Nothing here touches a save or a file.
package kits

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Project-Arrakis/tabr-tau/internal/augments"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
)

// The limits. The line limit is the one of a Give Items queue (a kit is applied as one), the XP and skill point limits those of Give XP and Set unspent points.
const (
	MaxKits        = 20
	MaxLines       = 50
	MaxNameRunes   = 60
	MaxQuantity    = 1_000_000
	MaxGrade       = 5
	MaxXP          = 100_000_000
	MaxSkillPoints = 100_000
	// MaxFileBytes is the largest kit file that is read. MaxStoredBytes is the most the kits may take in the settings file, as that file writes them (indented; the file is read whole and refuses a file past its own
	// limit, so a kit list that outgrew it would cost the person every other setting).
	MaxFileBytes   = 64 << 10
	MaxStoredBytes = 256 << 10
)

// Line is one item of a kit: what Give Items takes for it. Grade is the item's grade (0 to 5; 0 is what the game gives with none chosen, and an augment item with no grade gets the
// best one). Augments are applied to the item, at AugmentGrade (1 to 5; 0 means 1).
type Line struct {
	Item         string   `json:"item"`
	Quantity     int64    `json:"quantity"`
	Grade        int64    `json:"grade,omitempty"`
	Augments     []string `json:"augments,omitempty"`
	AugmentGrade int64    `json:"augmentGrade,omitempty"`
}

// Kit is a named kit. A kit holds at least one of items, XP and skill points.
type Kit struct {
	Name        string `json:"name"`
	Items       []Line `json:"items"`
	XP          int64  `json:"xp,omitempty"`
	SkillPoints int64  `json:"skillPoints,omitempty"`
}

// Normalize trims the name and the item ids and drops empty augment entries, and gives a kit with no items an empty list (not null) in its file.
func (k *Kit) Normalize() {
	k.Name = strings.TrimSpace(k.Name)
	if k.Items == nil {
		k.Items = []Line{}
	}
	for i := range k.Items {
		l := &k.Items[i]
		l.Item = strings.TrimSpace(l.Item)
		var aug []string
		for _, a := range l.Augments {
			if a = strings.TrimSpace(a); a != "" {
				aug = append(aug, a)
			}
		}
		l.Augments = aug
	}
}

// CheckName says whether s can be a kit name: 1 to 60 characters of letters, digits, spaces and a few marks. A name is shown on the page and may be typed into a file name by a
// person, so it carries no path separator, no markup and no control character.
func CheckName(s string) error {
	if s != strings.TrimSpace(s) || s == "" {
		return errors.New("a kit needs a name (no spaces at either end)")
	}
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > MaxNameRunes {
		return fmt.Errorf("a kit name is at most %d characters", MaxNameRunes)
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(" -_.,'()+&#!", r) {
			continue
		}
		return fmt.Errorf("a kit name may hold letters, digits, spaces and - _ . , ' ( ) + & # ! only; %q is not allowed", r)
	}
	return nil
}

// Validate says whether the kit can be kept and applied. It checks the kit by itself, against the item catalog and the augment data; it knows nothing of any save.
func (k Kit) Validate() error {
	if err := CheckName(k.Name); err != nil {
		return err
	}
	if len(k.Items) > MaxLines {
		return fmt.Errorf("a kit holds at most %d items (this one has %d)", MaxLines, len(k.Items))
	}
	if len(k.Items) == 0 && k.XP == 0 && k.SkillPoints == 0 {
		return errors.New("a kit needs at least one item, some XP or some skill points")
	}
	for i, l := range k.Items {
		if err := l.validate(); err != nil {
			return fmt.Errorf("item %d: %w", i+1, err)
		}
	}
	if k.XP < 0 || k.XP > MaxXP {
		return fmt.Errorf("XP must be a whole number from 0 to %d", MaxXP)
	}
	if k.SkillPoints < 0 || k.SkillPoints > MaxSkillPoints {
		return fmt.Errorf("skill points must be a whole number from 0 to %d", MaxSkillPoints)
	}
	return nil
}

func (l Line) validate() error {
	name, ok := catalog.Name(l.Item)
	if !ok {
		return fmt.Errorf("%q is not an item in the editor's catalog: pick an item from the list", clip(l.Item))
	}
	if l.Quantity < 1 || l.Quantity > MaxQuantity {
		return fmt.Errorf("%s: the quantity must be a whole number from 1 to %d", name, MaxQuantity)
	}
	if l.Grade < 0 || l.Grade > MaxGrade {
		return fmt.Errorf("%s: the grade must be from 0 to %d", name, MaxGrade)
	}
	if len(l.Augments) == 0 {
		if l.AugmentGrade != 0 {
			return fmt.Errorf("%s: an augment grade is given but no augments", name)
		}
		return nil
	}
	if l.AugmentGrade < 0 || l.AugmentGrade > MaxGrade {
		return fmt.Errorf("%s: the augment grade must be from 1 to %d", name, MaxGrade)
	}
	if err := augments.Check(l.Item, name, catalog.Category(l.Item), l.Augments); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// clip shortens text that came from a file before it goes into a message.
func clip(s string) string {
	if r := []rune(s); len(r) > 40 {
		return string(r[:40]) + "..."
	}
	return s
}

// Find is the place of the kit with this name in list (names are compared without regard to case), or -1.
func Find(list []Kit, name string) int {
	for i, k := range list {
		if strings.EqualFold(k.Name, name) {
			return i
		}
	}
	return -1
}

// CheckStored says whether list may be kept in the settings file: not more kits than MaxKits, not more bytes than MaxStoredBytes, no two with one name.
func CheckStored(list []Kit) error {
	if len(list) > MaxKits {
		return fmt.Errorf("at most %d kits are kept: delete one first", MaxKits)
	}
	seen := map[string]bool{}
	for _, k := range list {
		key := strings.ToLower(k.Name)
		if seen[key] {
			return fmt.Errorf("two kits are named %q", k.Name)
		}
		seen[key] = true
	}
	b, err := json.MarshalIndent(list, "  ", "  ") // as the settings file holds them
	if err != nil {
		return err
	}
	if len(b) > MaxStoredBytes {
		return fmt.Errorf("the kits would take %d KiB of the settings file; at most %d KiB are kept: delete or shorten a kit", len(b)>>10, MaxStoredBytes>>10)
	}
	return nil
}

// Get is the kit called name (names are compared without regard to case).
func Get(list []Kit, name string) (Kit, error) {
	at := Find(list, name)
	if at < 0 {
		return Kit{}, noKit(name)
	}
	return list[at], nil
}

func noKit(name string) error {
	return fmt.Errorf("there is no kit called %q (it may have been deleted)", clip(name))
}

// Put returns list with k in it: a new kit when was is empty, else the kit called was replaced (and renamed, when k has another name). k is checked first, and so is the list that
// results; the list it was given is not changed.
func Put(list []Kit, was string, k Kit) ([]Kit, error) {
	k.Normalize()
	if err := k.Validate(); err != nil {
		return nil, err
	}
	out := append([]Kit(nil), list...)
	at := -1
	if was != "" {
		if at = Find(out, was); at < 0 {
			return nil, noKit(was)
		}
	}
	if i := Find(out, k.Name); i >= 0 && i != at {
		return nil, fmt.Errorf("a kit called %q exists already", out[i].Name)
	}
	if at >= 0 {
		out[at] = k
	} else {
		out = append(out, k)
	}
	if err := CheckStored(out); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete returns list without the kit called name.
func Delete(list []Kit, name string) ([]Kit, error) {
	at := Find(list, name)
	if at < 0 {
		return nil, noKit(name)
	}
	return append(append([]Kit(nil), list[:at]...), list[at+1:]...), nil
}

// uniqueName is name with " (mark)" after it, or " (mark 2)", " (mark 3)" ... when that is taken, with name cut so the result is still a valid name.
func uniqueName(list []Kit, name, mark string) string {
	for n := 1; ; n++ {
		suffix := " (" + mark + ")"
		if n > 1 {
			suffix = fmt.Sprintf(" (%s %d)", mark, n)
		}
		base := []rune(name)
		if room := MaxNameRunes - utf8.RuneCountInString(suffix); len(base) > room {
			base = base[:room]
		}
		if c := strings.TrimSpace(string(base)) + suffix; Find(list, c) < 0 {
			return c
		}
	}
}

// Duplicate returns list with a copy of the kit called name added at the end, called "<name> (copy)", and the copy.
func Duplicate(list []Kit, name string) ([]Kit, Kit, error) {
	at := Find(list, name)
	if at < 0 {
		return nil, Kit{}, noKit(name)
	}
	cp := list[at]
	cp.Items = make([]Line, len(list[at].Items))
	for i, l := range list[at].Items {
		l.Augments = append([]string(nil), l.Augments...)
		cp.Items[i] = l
	}
	cp.Name = uniqueName(list, cp.Name, "copy")
	out := append(append([]Kit(nil), list...), cp)
	if err := CheckStored(out); err != nil {
		return nil, Kit{}, err
	}
	return out, cp, nil
}

// The kit file: a small JSON object, kits and nothing else. It names its format and version so a file of another kind is recognised, carries no path, no program and no setting
// of the editor, and is read strictly: a key it does not know is a refusal.
const (
	fileFormat  = "tabr-tau-kits"
	fileVersion = 1
)

type file struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	Kits    []Kit  `json:"kits"`
}

// Export is the file for the kits called names (all of list when names is empty), indented so a person can read it.
func Export(list []Kit, names []string) ([]byte, error) {
	f := file{Format: fileFormat, Version: fileVersion, Kits: []Kit{}}
	if len(names) == 0 {
		f.Kits = append(f.Kits, list...)
	}
	for _, n := range names {
		at := Find(list, n)
		if at < 0 {
			return nil, noKit(n)
		}
		f.Kits = append(f.Kits, list[at])
	}
	if len(f.Kits) == 0 {
		return nil, errors.New("there is no kit to export")
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Parse reads a kit file and returns its kits, each normalized and checked. Anything that is not a well-formed kit file is refused whole, with the reason: a file over
// MaxFileBytes, text that is not one JSON object, a format or version that is not this one, a key that is not part of a kit, an item the catalog does not know, a quantity out
// of range, augments that do not fit, two kits of one name. Nothing is kept from a file that is refused in part.
func Parse(data []byte) ([]Kit, error) {
	if len(data) > MaxFileBytes {
		return nil, fmt.Errorf("the file is larger than %d KiB: a kit file is small", MaxFileBytes>>10)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f file
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("this is not a kit file: %w", Friendly(err))
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("this is not a kit file: there is more after the first JSON object")
	}
	if f.Format != fileFormat {
		return nil, fmt.Errorf("this is not a kit file (its format is %q, not %q)", clip(f.Format), fileFormat)
	}
	if f.Version != fileVersion {
		return nil, fmt.Errorf("this kit file is version %d; this editor reads version %d", f.Version, fileVersion)
	}
	if len(f.Kits) == 0 {
		return nil, errors.New("the file holds no kits")
	}
	if len(f.Kits) > MaxKits {
		return nil, fmt.Errorf("the file holds %d kits; at most %d are read at once", len(f.Kits), MaxKits)
	}
	for i := range f.Kits {
		f.Kits[i].Normalize()
		if err := f.Kits[i].Validate(); err != nil {
			return nil, fmt.Errorf("kit %d (%s): %w", i+1, clip(f.Kits[i].Name), err)
		}
	}
	if err := CheckStored(f.Kits); err != nil {
		return nil, err
	}
	return f.Kits, nil
}

// Friendly puts a decoder's error in the words a person can use: no Go type names.
func Friendly(err error) error {
	var te *json.UnmarshalTypeError
	var se *json.SyntaxError
	switch {
	case errors.As(err, &te):
		return fmt.Errorf("the value of %q is of the wrong kind", te.Field)
	case errors.As(err, &se):
		return fmt.Errorf("it is not valid JSON (at byte %d)", se.Offset)
	}
	return err
}

// Merge adds imported kits to list. A kit whose name is taken is kept under "<name> (imported)" (or "(imported 2)" ...): an import never replaces a kit. It returns the new list and
// how many of the imported kits had to be renamed.
func Merge(list, imported []Kit) ([]Kit, int, error) {
	out := append([]Kit(nil), list...)
	renamed := 0
	for _, k := range imported {
		if Find(out, k.Name) >= 0 {
			k.Name = uniqueName(out, k.Name, "imported")
			renamed++
		}
		out = append(out, k)
	}
	if err := CheckStored(out); err != nil {
		return nil, 0, err
	}
	return out, renamed, nil
}

// Hash is a short fingerprint of the kit as it is used (the name, the items, the XP and the skill points, after Normalize). The page gets it with the dry run and sends it back with
// the apply, so that a kit that was changed, deleted and made again, or replaced by an import, between the question and the answer is not applied in place of the one that was shown.
func (k Kit) Hash() string {
	k.Items = append([]Line{}, k.Items...) // Normalize works in place on the lines: leave the caller's kit alone
	for i := range k.Items {
		k.Items[i].Augments = append([]string(nil), k.Items[i].Augments...)
	}
	k.Normalize()
	b, _ := json.Marshal(k) // a Kit holds only strings and numbers: Marshal cannot fail
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

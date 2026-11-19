package config

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// BadValueError is the error of Number for a value that is in the file and is not a finite number from Min to Max. Any other error of Number (the file cannot be opened or read) is not this type,
// so a caller can tell a value to correct from a file that could not be read (#753).
type BadValueError struct {
	Key, File, Raw string
	Min, Max       float64
}

func (e *BadValueError) Error() string {
	return fmt.Sprintf("%s in %s is %q, not a number from %g to %g", e.Key, e.File, e.Raw, e.Min, e.Max)
}

// NumberKey names one numeric setting for Numbers: the key and the range its value must lie in.
type NumberKey struct {
	Key      string
	Min, Max float64
}

// NumberValue is what Numbers found for one key: Found is false when the key is not in the file, and Err is a *BadValueError when its value is not a finite number in the key's range.
type NumberValue struct {
	Value float64
	Found bool
	Err   error
}

// Number reads one numeric setting of a file without changing anything: the section and the key are matched without regard to case, as the game reads its settings files; the value of the last line of the section that names the key (the game takes the last one), the number
// found is false when the file or the key is not there, and an error says the value is not a finite number from min to max. Nothing is written and no backup is taken.
func (d Dir) Number(name, section, key string, min, max float64) (value float64, found bool, err error) {
	vs, err := d.Numbers(name, section, []NumberKey{{key, min, max}})
	if err != nil {
		return 0, false, err
	}
	return vs[0].Value, vs[0].Found, vs[0].Err
}

// Numbers reads several numeric settings of one section with one read of the file, each as Number reads it. The error is the file's: it cannot be opened or read (a file that does not exist
// is not one: every key is then not found). The answer for each key is in the same position as the key; a value that is not a number in range is that key's Err only.
func (d Dir) Numbers(name, section string, keys []NumberKey) ([]NumberValue, error) {
	p, err := d.file(name)
	if err != nil {
		return nil, err
	}
	out := make([]NumberValue, len(keys))
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	entries := parseDoc(string(b)).entries()
	for i, k := range keys {
		raw, ok := "", false
		for _, e := range entries {
			if strings.EqualFold(e.Section, section) && strings.EqualFold(e.Key, k.Key) {
				raw, ok = e.Value, true
			}
		}
		if !ok {
			continue
		}
		out[i].Found = true
		v, perr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if perr != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < k.Min || v > k.Max {
			out[i].Err = &BadValueError{Key: k.Key, File: name, Raw: raw, Min: k.Min, Max: k.Max}
			continue
		}
		out[i].Value = v
	}
	return out, nil
}

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

// Number reads one numeric setting of a file without changing anything: the section and the key are matched without regard to case, as the game reads its settings files; the value of the last line of the section that names the key (the game takes the last one), the number
// found is false when the file or the key is not there, and an error says the value is not a finite number from min to max. Nothing is written and no backup is taken.
func (d Dir) Number(name, section, key string, min, max float64) (value float64, found bool, err error) {
	p, err := d.file(name)
	if err != nil {
		return 0, false, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	raw, ok := "", false
	for _, e := range parseDoc(string(b)).entries() {
		if strings.EqualFold(e.Section, section) && strings.EqualFold(e.Key, key) {
			raw, ok = e.Value, true
		}
	}
	if !ok {
		return 0, false, nil
	}
	v, perr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if perr != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < min || v > max {
		return 0, true, &BadValueError{Key: key, File: name, Raw: raw, Min: min, Max: max}
	}
	return v, true, nil
}

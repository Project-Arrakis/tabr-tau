package config

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// Number reads one numeric setting of a file without changing anything: the value of the last line of the section that names the key (the game takes the last one), the number
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
		if e.Section == section && e.Key == key {
			raw, ok = e.Value, true
		}
	}
	if !ok {
		return 0, false, nil
	}
	v, perr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if perr != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < min || v > max {
		return 0, true, fmt.Errorf("%s in %s is %q, not a number from %g to %g", key, name, raw, min, max)
	}
	return v, true, nil
}

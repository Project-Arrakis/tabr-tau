package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/layoutroll"
)

const layoutVetUsage = "usage: layout-roll vet <N> | vet <N> pass|fail [note] | vet list"

// runLayoutVet is `tabr-tau layout-roll vet ...` (#1222): set a layout in Game.ini so it can be checked in the game, record the result of the check, or list the layouts with their last result.
// The words after "vet" are plain words, not flags; everything after pass or fail is the note.
func runLayoutVet(args []string, cfgDir string, out io.Writer, base layoutroll.Options) error {
	o := base
	o.ConfigDir = cfgDir
	var r layoutroll.Result
	var err error
	switch {
	case len(args) == 1 && args[0] == "list":
		r, err = layoutroll.VetList(o)
	case len(args) == 0:
		return errors.New(layoutVetUsage)
	default:
		n, perr := strconv.Atoi(args[0])
		if perr != nil || strings.TrimLeft(args[0], "0123456789") != "" {
			return fmt.Errorf("%q is not a layout number (use a whole number, for example vet 9). %s", args[0], layoutVetUsage)
		}
		switch {
		case len(args) == 1:
			r, err = layoutroll.VetSet(o, n)
		case args[1] == "pass" || args[1] == "fail":
			r, err = layoutroll.VetRecord(o, n, args[1] == "pass", strings.Join(args[2:], " "))
		default:
			return fmt.Errorf("after the layout number put pass or fail, not %q. %s", args[1], layoutVetUsage)
		}
	}
	for _, l := range r.Lines {
		fmt.Fprintln(out, l)
	}
	return err
}

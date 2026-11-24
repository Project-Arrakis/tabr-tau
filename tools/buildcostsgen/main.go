package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	dir := flag.String("dumps", "", "the folder with the table dumps")
	build := flag.String("build", "", "the game build the dumps were read from")
	read := flag.String("read", "", "the date they were read (YYYY-MM-DD)")
	out := flag.String("out", "internal/catalog/buildcosts.json", "where to write the file")
	flag.Parse()
	if *dir == "" || *build == "" || *read == "" {
		fmt.Fprintln(os.Stderr, "all of -dumps -build -read are needed")
		os.Exit(2)
	}
	f, rep, err := Generate(*dir, *build, *read)
	if err != nil {
		fmt.Fprintln(os.Stderr, "buildcostsgen:", err)
		os.Exit(1)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", " ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(f); err != nil {
		fmt.Fprintln(os.Stderr, "buildcostsgen:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "buildcostsgen:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s: %d piece types, %d placeable types, build %s\n", *out, len(f.Pieces), len(f.Placeables), f.Build)
	fmt.Printf("left out, no cost in the tables (%d): %s\n", len(rep.NoCost), strings.Join(rep.NoCost, ", "))
	fmt.Printf("left out, two tables disagree (%d): %s\n", len(rep.Conflicts), strings.Join(rep.Conflicts, ", "))
}

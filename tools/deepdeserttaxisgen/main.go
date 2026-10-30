package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	var in Inputs
	out := flag.String("out", "internal/ops/deep-desert-taxis.json", "where to write the file")
	flag.StringVar(&in.Level, "level", "", "dump of the level DeepDesert_1_Travel")
	flag.StringVar(&in.Names, "names", "internal/ops/place-names.json", "place-names.json (its keys of the kind taxi)")
	flag.StringVar(&in.Read, "read", "", "the date the dump was read (YYYY-MM-DD)")
	flag.Parse()
	if in.Level == "" || in.Read == "" {
		fmt.Fprintln(os.Stderr, "both -level and -read are needed")
		os.Exit(2)
	}
	f, err := Generate(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "deepdeserttaxisgen:", err)
		os.Exit(1)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", " ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(f); err != nil {
		fmt.Fprintln(os.Stderr, "deepdeserttaxisgen:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "deepdeserttaxisgen:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s: %d taxi service points\n", *out, len(f.Taxis))
}

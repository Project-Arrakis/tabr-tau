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
	out := flag.String("out", "internal/ops/place-names.json", "where to write the file")
	flag.StringVar(&in.Locations, "locations", "", "DT_StaticLocationData dump")
	flag.StringVar(&in.Dialogue, "dialogue", "", "ST_Localization_Dialogue dump")
	flag.StringVar(&in.Build, "build", "", "the game build the dumps were read from")
	flag.StringVar(&in.Read, "read", "", "the date they were read (YYYY-MM-DD)")
	flag.Parse()
	if in.Locations == "" || in.Dialogue == "" || in.Build == "" || in.Read == "" {
		fmt.Fprintln(os.Stderr, "all of -locations -dialogue -build -read are needed")
		os.Exit(2)
	}
	f, err := Generate(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "placenamesgen:", err)
		os.Exit(1)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", " ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(f); err != nil {
		fmt.Fprintln(os.Stderr, "placenamesgen:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "placenamesgen:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s: %d trading-post keys, build %s\n", *out, len(f.Places), f.Build)
}

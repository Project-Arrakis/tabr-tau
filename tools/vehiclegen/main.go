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
	out := flag.String("out", "internal/catalog/vehicles.json", "where to write the catalog")
	flag.StringVar(&in.Modules, "modules", "", "CDT_BaseVehicleModules dump")
	flag.StringVar(&in.Schematics, "schematics", "", "DT_VehicleSchematics dump")
	flag.StringVar(&in.Templates, "templates", "", "DT_VehicleTemplates dump")
	flag.StringVar(&in.Decoded, "decoded", "", "the decoded module lists of the templates")
	flag.StringVar(&in.Weapons, "weapons", "", "DT_VehicleWeaponModules dump")
	flag.StringVar(&in.Build, "build", "", "the game build the dumps were read from")
	flag.StringVar(&in.Read, "read", "", "the date they were read (YYYY-MM-DD)")
	flag.Parse()
	if in.Modules == "" || in.Schematics == "" || in.Templates == "" || in.Decoded == "" || in.Weapons == "" || in.Build == "" || in.Read == "" {
		fmt.Fprintln(os.Stderr, "all of -modules -schematics -templates -decoded -weapons -build -read are needed")
		os.Exit(2)
	}
	d, err := Generate(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vehiclegen:", err)
		os.Exit(1)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", " ")
	if err := enc.Encode(d); err != nil {
		fmt.Fprintln(os.Stderr, "vehiclegen:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "vehiclegen:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s: %d classes, %d modules, build %s\n", *out, len(d.Classes), len(d.Modules), d.Build)
}

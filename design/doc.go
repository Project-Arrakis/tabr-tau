// Package design carries the shared look of the Tabr programs as files embedded in the binary that imports it. Version 0.1.0 holds the
// design tokens only: colour, type, spacing, shape, elevation and motion, as CSS custom properties in assets/tokens.css, for a dark
// theme (the default) and a light one (by the system setting, or by data-theme on the root element). Components, icons and a word list
// come in later versions.
//
// A program serves the stylesheet from its own origin, next to its own files, so that its content security policy does not change:
//
//	mux.Handle("/design/", http.StripPrefix("/design/", design.Handler()))
//
// and links it before its own stylesheet:
//
//	<link rel="stylesheet" href="/design/tokens.css">
//
// The module imports the standard library only, and never an internal package of the editor it lives beside. Its tests check both.
// It is version 0.x: a name can change between minor versions until v1 (see CHANGELOG.md).
package design

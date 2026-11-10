// Package design carries the shared look of the Tabr programs as files embedded in the binary that imports it. Version 0.2.0 holds the
// tokens of version 0.1.0 (assets/tokens.css, unchanged), three looks (Fremen, the default, Awakening and Sietch), each with tokens, components
// and a dark and a light theme, the components they share, the Motion setting, an icon sprite, a grain picture and the heading typeface Dune
// Rise with its licence. See README.md for how a program links a look.
//
// A program serves the files from its own origin, next to its own files, so that its content security policy does not change:
//
//	mux.Handle("/design/", http.StripPrefix("/design/", design.Handler()))
//
// and links the four files of a look before its own stylesheet, in the order design.LookFiles gives:
//
//	<link rel="stylesheet" href="/design/tokens-fremen.css">
//	<link rel="stylesheet" href="/design/components-fremen.css">
//	<link rel="stylesheet" href="/design/components.css">
//	<link rel="stylesheet" href="/design/motion.css">
//
// The module imports the standard library only, and never an internal package of the editor it lives beside. Its tests check both.
// It is version 0.x: a name can change between minor versions until v1 (see CHANGELOG.md).
package design

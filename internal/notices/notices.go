// Package notices embeds the third-party licence texts so they travel inside the executable.
package notices

import _ "embed"

// Text is the full THIRD-PARTY-NOTICES.md.
//
//go:embed THIRD-PARTY-NOTICES.md
var Text string

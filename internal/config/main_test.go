package config

import (
	"os"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Editing a config file is refused while the game runs; these tests must not depend on whether the developer has Dune open (#102).
func TestMain(m *testing.M) {
	save.NoGameRunningForTests()
	os.Exit(m.Run())
}

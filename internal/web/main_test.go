package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

var testUserConfig string

// New() reads the user's real settings.json (%APPDATA%\tabr-tau); the tests must not depend on, or write to, the developer's.
func TestMain(m *testing.M) {
	dir, cleanup := testsave.IsolateUserConfig()
	testUserConfig = dir
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func TestNewServerDoesNotUseTheDevelopersSettings(t *testing.T) {
	s := New(testsave.Empty(t), config.Dir{Path: t.TempDir()})
	rel, err := filepath.Rel(testUserConfig, s.SettingsPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		t.Fatalf("settings path %q is outside the test's own config folder %q", s.SettingsPath, testUserConfig)
	}
	if got := settings.Load(s.SettingsPath); got.AutoRefillOnOpen || len(got.TrackNumbers) != 0 {
		t.Fatalf("a fresh test config folder must give the defaults, got %+v", got)
	}
}

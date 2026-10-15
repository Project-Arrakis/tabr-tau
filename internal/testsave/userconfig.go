package testsave

import "os"

// IsolateUserConfig points the user's config folder at a fresh temporary folder for the rest of the test binary and returns a
// function that removes it. Call it from TestMain before m.Run. Without it, code that reads os.UserConfigDir (the editor's
// settings.json under %APPDATA%\tabr-tau) would read and write the developer's real settings, so results would depend on the
// machine the tests run on (audit QA-7).
func IsolateUserConfig() (dir string, cleanup func()) {
	dir, err := os.MkdirTemp("", "tabr-tau-userconfig-*")
	if err != nil {
		panic(err)
	}
	// os.UserConfigDir reads %AppData% on Windows, $HOME/Library/Application Support on macOS and $XDG_CONFIG_HOME or $HOME/.config elsewhere.
	for _, k := range []string{"AppData", "XDG_CONFIG_HOME", "HOME"} {
		os.Setenv(k, dir)
	}
	return dir, func() { os.RemoveAll(dir) }
}

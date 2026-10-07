package gui

import "testing"

func TestWantWindow(t *testing.T) {
	cases := []struct {
		goos                       string
		supported, web, subcommand bool
		want                       bool
	}{
		{"windows", true, false, false, true},   // the default on Windows: own window
		{"windows", true, true, false, false},   // --web
		{"windows", true, false, true, false},   // a subcommand such as diff never opens a window
		{"windows", false, false, false, false}, // built without window support
		{"linux", false, false, false, false},
		{"darwin", true, false, false, false}, // only Windows has the WebView2 shell
	}
	for _, c := range cases {
		if got := WantWindow(c.goos, c.supported, c.web, c.subcommand); got != c.want {
			t.Errorf("WantWindow(%q, supported=%v, web=%v, sub=%v) = %v, want %v", c.goos, c.supported, c.web, c.subcommand, got, c.want)
		}
	}
}

func TestStubsOffWindows(t *testing.T) {
	if Supported() {
		t.Skip("the Windows implementation is exercised by hand on a Windows PC")
	}
	if err := Run("http://127.0.0.1:1/", "t", 100, 100, nil); err != ErrUnsupported {
		t.Errorf("Run = %v", err)
	}
	if p, err := PickFile("t"); p != "" || err != ErrUnsupported {
		t.Errorf("PickFile = %q, %v", p, err)
	}
	MessageBox("t", "x") // must not panic
	if !Confirm("t", "x") {
		t.Error("the stub must agree so callers never block")
	}
	AttachConsole()
}

// tabr-tau inspects and edits a Dune: Awakening single-player save and its game configs.
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/web"
)

var version = "dev"

func usage() {
	fmt.Fprintf(os.Stderr, `tabr-tau %s

Usage:
  tabr-tau [flags]                      start the local web UI
  tabr-tau decode <save> <out.sqlite>   unpack a save (game.db / *.bak) to plain SQLite
  tabr-tau encode <in.sqlite> <out.db>  pack a SQLite file into the game's save format
  tabr-tau find                         list saves found on this machine

Flags:
`, version)
	flag.PrintDefaults()
}

func main() {
	savePath := flag.String("save", "", "save file or folder (default: auto-detect game.db)")
	cfgDir := flag.String("config", config.DefaultDir(), "game config folder containing the .ini files")
	addr := flag.String("addr", "127.0.0.1:8090", "listen address (keep it on localhost)")
	noOpen := flag.Bool("no-browser", false, "do not open the browser")
	flag.Usage = usage
	flag.Parse()

	if args := flag.Args(); len(args) > 0 {
		if err := runCommand(args); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	path := *savePath
	if path == "" {
		found := save.Discover()
		switch len(found) {
		case 0:
			fmt.Fprintln(os.Stderr, "No game.db found automatically; pass --save <path>.")
			os.Exit(1)
		case 1:
			path = found[0]
		default:
			fmt.Fprintln(os.Stderr, "Several saves found; pass --save <path> with one of:")
			for _, f := range found {
				fmt.Fprintln(os.Stderr, "  ", f)
			}
			os.Exit(1)
		}
	}
	s, err := save.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer s.Close()

	cd := config.Dir{Path: *cfgDir}
	if v := cd.Validate(); v["ok"] != true {
		fmt.Fprintln(os.Stderr, "warning: config check found problems (see the Config tab):")
		if e, has := v["error"]; has {
			fmt.Fprintln(os.Stderr, "  ", e)
		}
		for _, f := range v["files"].([]map[string]any) {
			if f["exists"] != true {
				fmt.Fprintln(os.Stderr, "   missing:", f["name"])
			} else if m := f["missingSections"].([]string); len(m) > 0 {
				fmt.Fprintln(os.Stderr, "   ", f["name"], "lacks sections:", m)
			}
		}
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	url := "http://" + ln.Addr().String() + "/"
	fmt.Printf("tabr-tau %s\n  save:   %s\n  config: %s\n  ui:     %s\n", version, s.Path, *cfgDir, url)
	if !*noOpen {
		go func() { time.Sleep(300 * time.Millisecond); openBrowser(url) }()
	}
	srv := &http.Server{Handler: web.New(s, config.Dir{Path: *cfgDir}).Handler(), ReadHeaderTimeout: 10 * time.Second}
	if err := srv.Serve(ln); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runCommand(args []string) error {
	switch args[0] {
	case "find":
		for _, f := range save.Discover() {
			fmt.Println(f)
		}
	case "decode":
		if len(args) != 3 {
			return fmt.Errorf("usage: decode <save> <out.sqlite>")
		}
		b, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		raw, err := save.Decode(b)
		if err != nil {
			return err
		}
		return os.WriteFile(args[2], raw, 0o644)
	case "encode":
		if len(args) != 3 {
			return fmt.Errorf("usage: encode <in.sqlite> <out.db>")
		}
		b, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		blob, err := save.Encode(b)
		if err != nil {
			return err
		}
		return os.WriteFile(args[2], blob, 0o644)
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
	return nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
	_ = filepath.Separator
}

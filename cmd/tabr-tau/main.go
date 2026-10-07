// tabr-tau inspects and edits a Dune: Awakening single-player save and its game configs.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/diff"
	"github.com/Project-Arrakis/tabr-tau/internal/gui"
	"github.com/Project-Arrakis/tabr-tau/internal/notices"
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
  tabr-tau licenses                     print the third-party licence notices
  tabr-tau diff <before> <after> [--tables a,b] [--ignore-tables a,b] [--ignore-columns t.c,c] [--noise] [--float-eps E] [--max-rows N] [--no-redact]
                                        row- and JSON-path-level diff of two saves (read-only; ids redacted by default)

Flags:
`, version)
	flag.PrintDefaults()
}

func main() {
	savePath := flag.String("save", "", "save file or folder (default: auto-detect game.db)")
	cfgDir := flag.String("config", config.DefaultDir(), "game config folder containing the .ini files")
	addr := flag.String("addr", "127.0.0.1:8090", "listen address (must be loopback unless --allow-remote)")
	allowRemote := flag.Bool("allow-remote", false, "DANGEROUS: allow a non-loopback --addr and connections from other machines (no login exists)")
	noOpen := flag.Bool("no-browser", false, "do not open the browser")
	forceWeb := flag.Bool("web", false, "use the browser instead of the built-in window (Windows opens its own window by default)")
	flag.Usage = usage
	gui.AttachConsole() // a GUI-subsystem exe has no console of its own; reattach to the parent's so CLI output shows
	flag.Parse()
	addrExplicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "addr" {
			addrExplicit = true
		}
	})

	if args := flag.Args(); len(args) > 0 {
		if err := runCommand(args); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	window := gui.WantWindow(runtime.GOOS, gui.Supported(), *forceWeb, false)
	// die reports a startup problem on stderr and, in window mode (where there may be no console), in a message box.
	die := func(format string, a ...any) {
		msg := fmt.Sprintf(format, a...)
		fmt.Fprintln(os.Stderr, msg)
		if window {
			gui.MessageBox("TABR TAU", msg)
		}
		os.Exit(1)
	}

	path := *savePath
	if path == "" {
		found := save.Discover()
		switch {
		case len(found) == 1:
			path = found[0]
		case window:
			picked, err := gui.PickFile("Choose the save to edit (game.db)")
			if err != nil {
				die("error: %v", err)
			}
			if picked == "" {
				return // cancelled
			}
			path = picked
		case len(found) == 0:
			die("No game.db found automatically; pass --save <path>.")
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
		die("error: %v", err)
	}
	defer s.Close()
	// os.Exit skips deferred calls, and Close deletes the decoded copies of the save from the temp folder, so every
	// exit after this point goes through here.
	die = func(format string, a ...any) {
		s.Close()
		msg := fmt.Sprintf(format, a...)
		fmt.Fprintln(os.Stderr, msg)
		if window {
			gui.MessageBox("TABR TAU", msg)
		}
		os.Exit(1)
	}

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

	if window && (*allowRemote) {
		die("--allow-remote needs the browser mode: add --web")
	}
	if err := web.CheckListenAddr(*addr, *allowRemote); err != nil {
		die("error: %v", err)
	}
	remoteOK := false
	if *allowRemote {
		fmt.Fprintln(os.Stderr, "WARNING: --allow-remote lets other machines connect to an editor with NO login that can rewrite your save and config.")
		fmt.Fprint(os.Stderr, "Type I UNDERSTAND to continue: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.TrimSpace(line) != "I UNDERSTAND" {
			die("not confirmed; exiting")
		}
		remoteOK = true
	}

	ln, err := web.Listen(*addr, addrExplicit)
	if err != nil {
		die("error: %v", err)
	}
	base := "http://" + ln.Addr().String()
	ws := web.New(s, config.Dir{Path: *cfgDir})
	ws.AllowRemote = remoteOK
	bootURL := ws.BootURL(base) // single use, valid for 10 minutes; sets this browser's session cookie
	fmt.Printf("tabr-tau %s\n  save:   %s\n  config: %s\n  ui:     %s\n", version, s.Path, *cfgDir, bootURL)
	srv := web.NewHTTPServer(ws.Handler())
	if window {
		// Serve in the background and show the editor in its own window; closing the window ends the program.
		// Closing with unsaved edits asks first (the window host destroys the window directly on close, so the
		// page's own beforeunload prompt, which still guards browser mode, would never run).
		go srv.Serve(ln)
		confirmClose := func() bool {
			return !s.Dirty() || gui.Confirm("TABR TAU", "You have unsaved changes that have not been written to your save.\n\nClose without saving?")
		}
		err := gui.Run(bootURL, "TABR TAU - Dune: Awakening save editor", 1280, 860, confirmClose)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			srv.Shutdown(ctx)
			return
		}
		// No WebView2 runtime (or the window could not be created): fall back to the browser rather than fail.
		if *noOpen {
			die("%v\n\n--no-browser is set, so the editor was not opened anywhere.", err)
		}
		gui.MessageBox("TABR TAU", fmt.Sprintf("%v\n\nOpening the editor in your browser instead.", err))
		openBrowser(bootURL)
		select {} // keep serving until the process is killed, as in browser mode
	}
	fmt.Println("  (open the ui link above; it works once, for 10 minutes)")
	if !*noOpen {
		go func() { time.Sleep(300 * time.Millisecond); openBrowser(bootURL) }()
	}
	if err := srv.Serve(ln); err != nil {
		die("error: %v", err)
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
	case "diff":
		return runDiff(args[1:])
	case "licenses":
		fmt.Print(notices.Text)
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

func runDiff(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	tables := fs.String("tables", "", "only these tables (comma separated)")
	noise := fs.Bool("noise", false, "hide columns that change without player action")
	maxRows := fs.Int("max-rows", 40, "rows listed per table and kind")
	noRedact := fs.Bool("no-redact", false, "show identifiers (private local analysis only)")
	ignoreTables := fs.String("ignore-tables", "", "skip these tables (comma separated)")
	ignoreCols := fs.String("ignore-columns", "", "skip these columns: name or table.name (comma separated)")
	eps := fs.Float64("float-eps", 1e-6, "relative tolerance for fractional numbers (integers always compare exactly)")
	// accept flags after the two paths as well as before them
	var paths []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return err
		}
		args = fs.Args()
		if len(args) > 0 {
			paths = append(paths, args[0])
			args = args[1:]
		}
	}
	if len(paths) != 2 {
		return fmt.Errorf("usage: diff <before> <after> [flags]")
	}
	a, closeA, err := save.OpenReadOnly(paths[0])
	if err != nil {
		return err
	}
	defer closeA()
	b, closeB, err := save.OpenReadOnly(paths[1])
	if err != nil {
		return err
	}
	defer closeB()
	opt := diff.Options{Noise: *noise, MaxRows: *maxRows, NoRedact: *noRedact, FloatEps: *eps}
	if *ignoreTables != "" {
		opt.IgnoreTables = strings.Split(*ignoreTables, ",")
	}
	if *ignoreCols != "" {
		opt.IgnoreColumns = map[string]bool{}
		for _, c := range strings.Split(*ignoreCols, ",") {
			opt.IgnoreColumns[c] = true
		}
	}
	if *tables != "" {
		opt.Tables = strings.Split(*tables, ",")
	}
	res, err := diff.Compare(a, b, opt)
	if err != nil {
		return err
	}
	res.WriteText(os.Stdout)
	return nil
}

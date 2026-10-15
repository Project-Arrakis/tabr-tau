// Package web serves the local UI and JSON API.
package web

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

//go:embed static/*
var static embed.FS

type Server struct {
	Save  *save.Save
	Ops   *ops.Ops
	Cfg   config.Dir
	token string // session token: the value of the HttpOnly session cookie. Never sent to a script or placed in a page.
	mux   *http.ServeMux

	now    func() time.Time // injectable for tests
	bootMu sync.Mutex
	boots  map[string]time.Time // one-time bootstrap tokens -> expiry

	// SettingsPath is the editor's own preferences file (see internal/settings); tests point it at a temp folder.
	SettingsPath string
	startupMu    sync.Mutex
	startupNote  string // what the open-time refill did, shown once by the UI
	startupWarn  bool   // the note is a problem or a skip, so the UI keeps it on screen
	lastAuto     string // the last automatic save, kept for the Automatic refill card

	// bound is the address the editor listens on, set by BindAddr once it is known ("" until then, as in most tests). It scopes the
	// session cookie to this port and the accepted Host header to this listener (NET-1, NET-2, SEC-7 of the 2026-10-15 audit).
	bound string
	port  string
}

func New(s *save.Save, cfg config.Dir) *Server {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed; refusing to start without an unpredictable token: " + err.Error())
	}
	srv := &Server{Save: s, Ops: &ops.Ops{S: s}, Cfg: cfg, token: hex.EncodeToString(b), mux: http.NewServeMux(), now: time.Now, boots: map[string]time.Time{}, SettingsPath: settings.Path()}
	srv.routes()
	return srv
}

const (
	cookieName = "tabr_session"
	bootTTL    = 10 * time.Minute
)

// notSignedIn is what a page is told when its session is gone (the editor was restarted, or the cookie was cleared). It is right for both
// ways of running: the editor's own window has no terminal, and a browser tab needs the new link.
const notSignedIn = "This window is no longer signed in to the editor, usually because the editor was closed and started again. " +
	"Close tabr-tau completely and start it again: the editor's own window opens signed in; in a browser, open the new link printed in the terminal."

// NewBootToken returns a single-use token, valid for bootTTL, that a browser can exchange for the session cookie
// by opening /?boot=<token>. The process prints it (and opens the browser with it) at startup.
func (s *Server) NewBootToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed; refusing to issue a bootstrap token: " + err.Error())
	}
	tok := hex.EncodeToString(b)
	s.bootMu.Lock()
	defer s.bootMu.Unlock()
	now := s.now()
	for k, exp := range s.boots { // drop expired entries
		if now.After(exp) {
			delete(s.boots, k)
		}
	}
	s.boots[tok] = now.Add(bootTTL)
	return tok
}

// BootURL is base (for example http://127.0.0.1:8090) plus a fresh one-time bootstrap query.
func (s *Server) BootURL(base string) string { return base + "/?boot=" + s.NewBootToken() }

func (s *Server) consumeBoot(tok string) bool {
	if tok == "" {
		return false
	}
	s.bootMu.Lock()
	defer s.bootMu.Unlock()
	exp, ok := s.boots[tok]
	delete(s.boots, tok) // single use, even if expired
	return ok && !s.now().After(exp)
}

// BindAddr tells the server the address its listener has (for example 127.0.0.1:8090). From then on the session cookie is named after
// the port, and a request is served only when its Host header names this listener. Browsers share cookies between ports of one host,
// so two editors on one machine (the second one starts on another port when 8090 is busy) used to overwrite each other's cookie, and
// the cookie of one was sent to the other.
func (s *Server) BindAddr(addr string) {
	s.bound = addr
	if _, p, err := net.SplitHostPort(addr); err == nil {
		s.port = p
	}
}

// sessionCookie is the name of this editor's session cookie: per port once the listener is known.
func (s *Server) sessionCookie() string {
	if s.port == "" {
		return cookieName
	}
	return cookieName + "_" + s.port
}

// hasSession accepts the request when any cookie of this editor's name holds the token. A request can carry several cookies of one
// name (another program on the same host may plant a shadow one, which a browser sends before or after the real one), so the first is
// not the only one looked at.
func (s *Server) hasSession(r *http.Request) bool {
	name := s.sessionCookie()
	for _, c := range r.Cookies() {
		if c.Name == name && subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.token)) == 1 {
			return true
		}
	}
	return false
}

// hostOK is hostAllowed once the listener is known: the Host header must be this listener's own address, or localhost with its port.
func (s *Server) hostOK(hostport string) bool {
	if s.bound == "" {
		return hostAllowed(hostport)
	}
	if strings.EqualFold(hostport, s.bound) || strings.EqualFold(hostport, "localhost:"+s.port) {
		return true
	}
	return s.port == "80" && hostAllowed(hostport) // a browser leaves the port off for the default one
}

// Handler enforces, in order: response security headers, loopback Host, loopback peer,
// the session cookie on /api/, and for /api/ requests: same-origin fetch metadata / Origin, and a JSON
// Content-Type on anything that changes state. See docs/design (section 4.4) and findings F-04 / #5.
func (s *Server) Handler() http.Handler {
	return recoverPanic(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		if !s.hostOK(r.Host) {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if !isLoopbackPeer(r.RemoteAddr) {
			http.Error(w, "forbidden: this editor only accepts connections from this computer", http.StatusForbidden)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if !s.hasSession(r) {
				// JSON, so the page can show this sentence: a plain-text body made the page fall back to a generic message (UX-10).
				writeJSON(w, http.StatusForbidden, map[string]any{"error": notSignedIn, "code": "not_signed_in"})
				return
			}
			if !sameOriginRequest(r) {
				http.Error(w, "forbidden: cross-origin request", http.StatusForbidden)
				return
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
					http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
					return
				}
			}
		}
		s.mux.ServeHTTP(w, r)
	}))
}

// recoverPanic turns a panic in any handler into a JSON 500. net/http would recover it too, but by closing the connection, and the page
// would see a reset instead of a message (SEC-5 of the 2026-10-15 audit: a hostile save with a NULL where the code expected text).
// The panic and its stack go to the terminal log; the page gets a sentence that carries no detail from the save.
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			log.Printf("tabr-tau: %s %s panicked: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
			setSecurityHeaders(w)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "The editor hit an internal error while answering this request. Nothing was changed. If it keeps happening, the save may hold a value the editor does not expect; the details are in the terminal window."})
		}()
		next.ServeHTTP(w, r)
	})
}

// setSecurityHeaders applies to every response, including errors. The CSP allows only same-origin scripts and
// styles (no inline code, no eval), same-origin connections, and forbids framing, base tags, plugins and forms.
func setSecurityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("X-Frame-Options", "DENY")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; font-src 'self'; "+
		"frame-ancestors 'none'; base-uri 'none'; object-src 'none'; form-action 'none'")
}

func hostAllowed(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	switch host {
	case "127.0.0.1", "localhost", "[::1]", "::1":
		return true
	}
	return false
}

func isLoopbackPeer(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// sameOriginRequest rejects browser requests that originate from another site. Requests without Origin and
// Sec-Fetch-Site headers (curl, the address bar) are allowed; the token still gates them.
func sameOriginRequest(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
	default:
		return false
	}
	if o := r.Header.Get("Origin"); o != "" {
		u, err := url.Parse(o)
		if err != nil || u.Scheme != "http" || !strings.EqualFold(u.Host, r.Host) {
			return false
		}
	}
	return true
}

// writeJSON marshals first and only then writes the status line, so a value JSON cannot carry (a NaN or infinite number that
// came out of a hostile save, say) answers a clear JSON 500 and not a 200 with an empty body (SEC-1 of the 2026-10-15 audit).
func writeJSON(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		status = http.StatusInternalServerError
		b = []byte(`{"error":"The editor could not encode this answer. A value in the save is not representable (for example a number that is not finite)."}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(append(b, '\n')) // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter (JSON built by encoding/json; nosniff and CSP set)
}

// publicError is err's message with the places a person's name hides taken out: a file path in an operating-system error
// is cut to its file name, and the user's home folder anywhere in the text becomes "~" (SEC-10 of the 2026-10-15 audit). Text copied from
// the page into a bug report then does not carry the Windows user name.
func publicError(err error) string {
	msg := err.Error()
	var paths []string
	collectPaths(err, &paths)
	for _, p := range paths {
		// either separator, whatever system the editor runs on; a root such as "/" or `C:\` has no file name to keep, so it is left alone
		if base := p[strings.LastIndexAny(p, `/\`)+1:]; p != "" && base != "" {
			msg = strings.ReplaceAll(msg, p, base)
		}
	}
	if re := homeRe(); re != nil {
		msg = re.ReplaceAllString(msg, "~")
	}
	return msg
}

// homeRe matches the user's home folder in either spelling, ignoring case; nil when the folder is not known or too short to be safe.
func homeRe() *regexp.Regexp {
	home, e := os.UserHomeDir()
	if e != nil || len(home) <= 3 {
		return nil
	}
	homeReCache.Lock()
	defer homeReCache.Unlock()
	if homeReCache.re == nil || homeReCache.home != home { // compiled once per home folder, not once per error
		homeReCache.home = home
		homeReCache.re = regexp.MustCompile(`(?i)` + regexp.QuoteMeta(home) + `|` + regexp.QuoteMeta(filepath.ToSlash(home)))
	}
	return homeReCache.re
}

var homeReCache struct {
	sync.Mutex
	home string
	re   *regexp.Regexp
}

func collectPaths(err error, out *[]string) {
	switch e := err.(type) {
	case nil:
		return
	case *fs.PathError:
		*out = append(*out, e.Path)
	case *os.LinkError:
		*out = append(*out, e.Old, e.New)
	}
	switch u := err.(type) {
	case interface{ Unwrap() error }:
		collectPaths(u.Unwrap(), out)
	case interface{ Unwrap() []error }:
		for _, e := range u.Unwrap() {
			collectPaths(e, out)
		}
	}
}

func respond(w http.ResponseWriter, v any, err error) {
	if err != nil {
		body := map[string]any{"error": publicError(err)}
		switch { // a stable code, so the page never has to match message wording
		case errors.Is(err, save.ErrChangedOnDisk):
			body["code"] = "changed_on_disk"
		case errors.Is(err, save.ErrReviewChanged):
			body["code"] = "review_changed"
		}
		writeJSON(w, http.StatusBadRequest, body)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) get(path string, fn func(r *http.Request) (any, error)) {
	s.mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
		v, err := fn(r)
		respond(w, v, err)
	})
}

func (s *Server) post(path string, fn func(a ops.Args) (any, error)) {
	s.mux.HandleFunc("POST "+path, func(w http.ResponseWriter, r *http.Request) {
		a := ops.Args{}
		r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
		if err := json.NewDecoder(r.Body).Decode(&a); err != nil && err.Error() != "EOF" {
			respond(w, nil, errors.New("invalid JSON body"))
			return
		}
		v, err := fn(a)
		respond(w, v, err)
	})
}

func qint(r *http.Request, k string, def int) int {
	if n, err := strconv.Atoi(r.URL.Query().Get(k)); err == nil {
		return n
	}
	return def
}

func (s *Server) routes() {
	o := s.Ops
	// Static UI: a fixed list of asset paths, nothing else from the embedded folder is reachable.
	for path, ct := range map[string]string{
		"/app.css": "text/css; charset=utf-8", "/app.js": "text/javascript; charset=utf-8", "/html.js": "text/javascript; charset=utf-8",
		"/livemap.js": "text/javascript; charset=utf-8", "/maps/hagga-basin.png": "image/png", "/maps/deep-desert.png": "image/png"} {
		file, ctype := "static"+path, ct
		s.mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
			b, err := static.ReadFile(file)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", ctype)
			w.Write(b) // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter (embedded, fixed, non-HTML asset; nosniff + CSP set)
		})
	}

	// The Live Map's marker icons: the files of the embedded maps/icons folder, listed once here. The list comes from the build, never
	// from the request, so a request can only name a file that was embedded (anything else is a 404).
	if icons, err := static.ReadDir("static/maps/icons"); err == nil {
		for _, e := range icons {
			name := e.Name()
			ctype := map[string]string{".webp": "image/webp", ".png": "image/png"}[strings.ToLower(filepath.Ext(name))]
			if e.IsDir() || ctype == "" {
				continue
			}
			file, ct := "static/maps/icons/"+name, ctype
			s.mux.HandleFunc("GET /maps/icons/"+name, func(w http.ResponseWriter, r *http.Request) {
				b, err := static.ReadFile(file)
				if err != nil {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", ct)
				w.Write(b) // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter (embedded, fixed, non-HTML asset; nosniff + CSP set)
			})
		}
	}

	// Index: served only to a browser that holds the session cookie. A valid one-time boot token is exchanged for the
	// cookie and the URL is cleaned by redirect. Anyone else gets a static page that contains no secret.
	s.mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if boot := r.URL.Query().Get("boot"); boot != "" {
			if s.consumeBoot(boot) {
				// Deliberately no Secure flag: the editor speaks plain HTTP on loopback, where there is no network path to sniff,
				// and some browsers refuse to store Secure cookies from an http origin, which would lock every user out.
				// The cookie is HttpOnly, SameSite=Strict and session-scoped.
				http.SetCookie(w, &http.Cookie{Name: s.sessionCookie(), Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode}) // nosemgrep: go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
		} else if s.hasSession(r) {
			b, _ := static.ReadFile("static/index.html")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(b) // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter (embedded, fixed page with no dynamic content)
			return
		}
		b, _ := static.ReadFile("static/locked.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write(b) // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter (embedded, fixed page with no dynamic content)
	})

	s.get("/api/overview", func(r *http.Request) (any, error) { return o.Overview() })
	s.get("/api/settings", func(r *http.Request) (any, error) {
		cur, perr := settings.LoadStrict(s.SettingsPath)
		problem := ""
		if perr != nil { // reported, not hidden: changing a setting will be refused until the file is fixed or removed
			problem, cur = perr.Error(), settings.Load(s.SettingsPath)
		}
		s.startupMu.Lock()
		defer s.startupMu.Unlock()
		return map[string]any{"autoRefillOnOpen": cur.AutoRefillOnOpen, "last": s.lastAuto, "problem": problem}, nil
	})
	s.post("/api/settings", func(a ops.Args) (any, error) {
		v, isBool := a["autoRefillOnOpen"].(bool)
		if !isBool {
			return nil, errors.New("autoRefillOnOpen must be true or false")
		}
		next, err := settings.Update(s.SettingsPath, func(cur *settings.Settings) error { cur.AutoRefillOnOpen = v; return nil })
		if err != nil {
			return nil, err
		}
		s.startupMu.Lock()
		defer s.startupMu.Unlock()
		return map[string]any{"autoRefillOnOpen": next.AutoRefillOnOpen, "last": s.lastAuto}, nil
	})
	s.get("/api/startup", func(r *http.Request) (any, error) {
		note, warn := s.takeStartup()
		return map[string]any{"note": note, "warn": warn}, nil
	})
	s.get("/api/catalog/items", func(r *http.Request) (any, error) { return catalog.All(), nil })
	s.get("/api/save/state", func(r *http.Request) (any, error) { return o.State() })

	// players
	s.get("/api/player", func(r *http.Request) (any, error) { return o.Player() })
	s.post("/api/player/teleport", o.Teleport)
	s.get("/api/player/destinations", func(r *http.Request) (any, error) {
		return o.Destinations(r.URL.Query().Get("kind"), r.URL.Query().Get("q"))
	})
	s.get("/api/player/inventory", func(r *http.Request) (any, error) { return o.Inventory() })
	s.post("/api/player/give", o.GiveItem)
	s.post("/api/player/give-items", o.GiveItems)
	s.post("/api/items/augment-options", o.AugmentOptions)
	s.post("/api/items/augment", o.AugmentItem)
	s.post("/api/player/repair", func(ops.Args) (any, error) { return o.RepairGear() })
	s.post("/api/player/refill", func(ops.Args) (any, error) { return o.RefillContainers() })
	s.post("/api/player/solari", o.AddSolari)
	s.post("/api/player/intel", o.AddIntel)
	s.post("/api/player/xp", o.AddXP)
	s.post("/api/player/currency", o.AddCurrency)
	s.post("/api/items/update", o.SetItem)
	s.post("/api/items/delete", o.DeleteItem)
	s.get("/api/player/factions", func(r *http.Request) (any, error) { return o.Factions() })
	s.post("/api/player/factions", o.SetReputation)
	s.get("/api/player/specs", func(r *http.Request) (any, error) { return o.Specs(settings.Load(s.SettingsPath).TrackNumbers) })
	s.post("/api/player/specs/xp", func(a ops.Args) (any, error) { return o.AddSpecXP(settings.Load(s.SettingsPath).TrackNumbers, a) })
	s.post("/api/player/specs/max", func(a ops.Args) (any, error) { return o.GrantMaxSpec(settings.Load(s.SettingsPath).TrackNumbers, a) })
	s.post("/api/player/specs/reset", func(a ops.Args) (any, error) { return o.ResetSpec(settings.Load(s.SettingsPath).TrackNumbers, a) })
	s.post("/api/player/specs/keystones/grant", func(ops.Args) (any, error) { return o.GrantAllKeystones() })
	s.post("/api/player/specs/keystones/reset", func(ops.Args) (any, error) { return o.ResetAllKeystones() })
	s.post("/api/player/specs/assign", func(a ops.Args) (any, error) {
		n, err := a.Int("number")
		if err != nil {
			return nil, err
		}
		if _, err := settings.Update(s.SettingsPath, func(cur *settings.Settings) error {
			next, err := ops.CheckAssign(cur.TrackNumbers, a.Str("track"), n)
			if err != nil {
				return err
			}
			cur.TrackNumbers = next
			return nil
		}); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "track": a.Str("track"), "number": n}, nil
	})
	s.post("/api/player/specs", o.SetSpec)
	s.get("/api/player/journey", func(r *http.Request) (any, error) { return o.Journey(r.URL.Query().Get("q")) })
	s.get("/api/player/journey/browse", func(r *http.Request) (any, error) { return o.JourneyBrowse() })
	s.post("/api/player/journey", o.JourneySet)
	s.get("/api/player/tutorials", func(r *http.Request) (any, error) { return o.Tutorials() })
	s.post("/api/player/tutorials", o.TutorialSet)
	s.get("/api/player/tags", func(r *http.Request) (any, error) { return o.Tags() })
	s.post("/api/player/tags", o.TagSet)
	s.get("/api/player/recipes", func(r *http.Request) (any, error) { return o.Recipes() })

	// bases
	s.get("/api/bases", func(r *http.Request) (any, error) { return o.Bases() })
	s.get("/api/bases/storage", func(r *http.Request) (any, error) { return o.Storage() })
	s.get("/api/bases/storage/items", func(r *http.Request) (any, error) { return o.StorageItems(int64(qint(r, "inventory", 0))) })
	s.post("/api/bases/give", o.GiveToInventory)
	s.post("/api/bases/repair", func(ops.Args) (any, error) { return o.RepairBuildings() })
	s.post("/api/bases/clear-sand", func(ops.Args) (any, error) { return o.ClearSand() })
	s.get("/api/bases/claim", func(r *http.Request) (any, error) { return o.LandClaims() })
	s.get("/api/player/summary", func(r *http.Request) (any, error) { return o.Summary() })
	s.get("/api/bases/list", func(r *http.Request) (any, error) { return o.BaseList() })
	s.get("/api/bases/power", func(r *http.Request) (any, error) { return o.BasePower(int64(qint(r, "base", 0))) })
	s.get("/api/bases/water", func(r *http.Request) (any, error) { return o.BaseWater(int64(qint(r, "base", 0))) })
	s.get("/api/bases/inventory", func(r *http.Request) (any, error) { return o.BaseInventory(int64(qint(r, "base", 0))) })
	s.get("/api/livemap", func(r *http.Request) (any, error) { return o.LiveMap(r.URL.Query().Get("map")) })
	s.post("/api/bases/claim/expand", o.ExpandLandClaim)
	s.post("/api/bases/claim/apply", o.ApplyLandClaim)
	s.post("/api/bases/claim/shrink", o.ShrinkLandClaim)
	s.post("/api/bases/refill-water", func(ops.Args) (any, error) { return o.RefillBaseWater() })
	s.post("/api/bases/refill-generators", func(ops.Args) (any, error) { return o.RefillGenerators() })

	// vehicles / vendors / landsraad
	s.get("/api/vehicles", func(r *http.Request) (any, error) { return o.Vehicles() })
	s.post("/api/vehicles/bring", o.BringVehicle)
	s.post("/api/vehicles/repair", func(a ops.Args) (any, error) {
		pct := int64(100)
		if _, has := a["threshold"]; has {
			n, err := a.IntRange("threshold", 1, 100)
			if err != nil {
				return nil, err
			}
			pct = n
		}
		var vehicle int64
		if _, has := a["vehicle_id"]; has {
			v, err := a.Int("vehicle_id")
			if err != nil {
				return nil, err
			}
			vehicle = v
		}
		return o.RepairVehiclesWhere(pct, vehicle)
	})
	s.get("/api/player/faction", func(r *http.Request) (any, error) { return o.Faction() })
	s.post("/api/player/faction", o.SetFaction)
	s.get("/api/player/skills", func(r *http.Request) (any, error) { return o.Skills() })
	s.post("/api/player/skills/module", o.SetSkillModule)
	s.post("/api/player/skills/points", o.SetSkillPoints)
	s.post("/api/player/skills/starter", o.RestoreStarterSkills)
	s.get("/api/player/research", func(r *http.Request) (any, error) { return o.Research() })
	s.post("/api/player/research/unlock", o.UnlockResearch)
	s.get("/api/player/crafting", func(r *http.Request) (any, error) { return o.Crafting() })
	s.post("/api/player/crafting/unlock", o.UnlockRecipe)
	s.get("/api/player/building-sets", func(r *http.Request) (any, error) { return o.BuildingSets() })
	s.get("/api/player/customizations", func(r *http.Request) (any, error) { return o.Customizations() })
	s.post("/api/player/customizations/grant", o.GrantCustomizations)
	s.post("/api/player/building-sets/unlock-all", func(ops.Args) (any, error) { return o.UnlockAllBuildingSets() })
	s.post("/api/player/research/unlock-all", func(ops.Args) (any, error) { return o.UnlockAllResearch() })
	s.post("/api/player/crafting/unlock-all", func(ops.Args) (any, error) { return o.UnlockAllRecipes() })
	s.post("/api/player/skills/max", o.MaxSkills)
	s.get("/api/vendors", func(r *http.Request) (any, error) { return o.Vendors() })
	s.post("/api/vendors/reset", o.ResetVendors)
	s.get("/api/landsraad", func(r *http.Request) (any, error) { return o.Landsraad() })
	s.post("/api/landsraad/progress", o.SetTaskProgress)
	s.post("/api/landsraad/complete", o.CompleteTask)
	s.post("/api/landsraad/decree", o.SetDecree)
	s.post("/api/landsraad/term", o.SetTerm)

	// database
	s.get("/api/db/tables", func(r *http.Request) (any, error) { return o.Tables() })
	s.get("/api/db/table", func(r *http.Request) (any, error) {
		return o.TableRows(r.URL.Query().Get("name"), r.URL.Query().Get("q"), qint(r, "limit", 200), qint(r, "offset", 0))
	})
	s.post("/api/db/update", o.UpdateRow)
	s.post("/api/db/sql", func(a ops.Args) (any, error) { return o.SQL(a.Str("sql")) })
	s.post("/api/db/exec", func(a ops.Args) (any, error) { return o.ExecSQL(a.Str("sql")) })
	s.mux.HandleFunc("GET /api/db/export", func(w http.ResponseWriter, r *http.Request) {
		b, ct, err := o.Export(r.URL.Query().Get("name"), r.URL.Query().Get("format"))
		if err != nil {
			respond(w, nil, err)
			return
		}
		w.Header().Set("Content-Type", ct)
		ext := "csv"
		if ct == "application/json" {
			ext = "json"
		}
		w.Header().Set("Content-Disposition", `attachment; filename="`+safeFilename(r.URL.Query().Get("name"))+"."+ext+`"`)
		// CSV/JSON export of save data, not HTML. Content sniffing and CSV formula injection are tracked in F-03 (#4); S0 adds nosniff and cell escaping. Remove this nosemgrep when #4 lands.
		w.Write(b) // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter
	})

	// save file lifecycle
	// Saving is only reachable from the review: the request must quote the token of the state that was shown, and the
	// save checks it under its own lock. The game-running check cannot be switched off from here.
	s.post("/api/save/commit", func(a ops.Args) (any, error) { return s.Save.CommitReviewed(a.Str("reviewed"), false) })
	s.post("/api/save/discard", func(ops.Args) (any, error) { return map[string]any{"ok": true}, s.Save.Discard() })
	// The game-running guard looks at this computer only. For a save on a share, a removable drive or outside the game's own folder it asks
	// the person to say the game is closed everywhere; this is that answer, kept until the editor closes.
	s.post("/api/save/confirm-location", func(ops.Args) (any, error) {
		if !save.GameStateFor(s.Save.Path, true).NeedsConfirm { // only a location the guard is actually refusing can be confirmed
			return nil, errors.New("this save's location is not in question: there is nothing to confirm")
		}
		save.ConfirmLocation(s.Save.Path)
		return map[string]any{"ok": true}, nil
	})
	s.get("/api/save/review", func(r *http.Request) (any, error) {
		return o.Review(r.URL.Query().Get("unredacted") == "1", qint(r, "limit", 40))
	})
	s.get("/api/save/backups", func(r *http.Request) (any, error) { return s.Save.Backups(), nil })
	s.post("/api/save/restore", func(a ops.Args) (any, error) { return map[string]any{"ok": true}, s.Save.Restore(a.Str("name")) })

	// game config (.ini)
	s.get("/api/config/validate", func(r *http.Request) (any, error) { return s.Cfg.Validate(), nil })
	s.post("/api/config/create", func(a ops.Args) (any, error) {
		return map[string]any{"ok": true}, s.Cfg.Create(a.Str("name"))
	})
	s.get("/api/config/files", func(r *http.Request) (any, error) { return s.Cfg.List() })
	s.get("/api/config/file", func(r *http.Request) (any, error) { return s.Cfg.Read(r.URL.Query().Get("name")) })
	s.post("/api/config/set", func(a ops.Args) (any, error) {
		line := -1
		if _, has := a["line"]; has {
			l, err := a.Int("line")
			if err != nil {
				return nil, err
			}
			line = int(l)
		}
		return map[string]any{"ok": true}, s.Cfg.Set(a.Str("name"), a.Str("section"), a.Str("key"), a.Str("value"), line)
	})
	s.post("/api/config/delete", func(a ops.Args) (any, error) {
		l, err := a.Int("line")
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, s.Cfg.Delete(a.Str("name"), a.Str("key"), int(l))
	})
	s.post("/api/config/raw", func(a ops.Args) (any, error) {
		return map[string]any{"ok": true}, s.Cfg.WriteRaw(a.Str("name"), a.Str("text"))
	})
	s.get("/api/config/backups", func(r *http.Request) (any, error) { return s.Cfg.Backups(r.URL.Query().Get("name")), nil })
	s.post("/api/config/restore", func(a ops.Args) (any, error) {
		return map[string]any{"ok": true}, s.Cfg.Restore(a.Str("name"), a.Str("backup"))
	})
}

// RunStartupTasks does what the editor's settings ask for when it opens. Today that is the optional refill of base
// water storage and power buildings. This is a deliberate exception to "Save only from the review pane" (operator decision,
// 2026-10-08), for this opt-in setting only: the refill is applied and written straight away so it takes effect the
// next time the game loads, without a manual review, save, exit and reload. The write still goes through the whole
// pipeline (single-player check, changed-on-disk check, integrity check, backup first, read-back verification), and
// it is skipped when a single-player session is running. If the write is refused, the edits stay pending so the
// person can see and review them.
func (s *Server) RunStartupTasks() {
	if !settings.Load(s.SettingsPath).AutoRefillOnOpen {
		return
	}
	set := func(note string, warn bool) {
		s.startupMu.Lock()
		s.startupNote, s.startupWarn = note, warn
		s.startupMu.Unlock()
	}
	if g := save.GameStateFor(s.Save.Path, true); g.Blocked {
		set("Automatic refill skipped: "+g.Reason+".", true)
		return
	}
	if len(s.Save.Pending()) > 0 { // never fold someone else's pending edits into an unreviewed save
		set("Automatic refill skipped: there are unsaved edits.", true)
		return
	}
	w, err1 := s.Ops.RefillBaseWater()
	g, err2 := s.Ops.RefillGenerators()
	if err1 != nil || err2 != nil {
		queued := ""
		if n := len(s.Save.Pending()); n > 0 {
			queued = fmt.Sprintf(" %d prepared edit(s) are waiting under Review & save; nothing was written.", n)
		}
		set("Automatic refill could not finish: "+errors.Join(err1, err2).Error()+"."+queued, true)
		return
	}
	nw, ng := w.(map[string]any)["filled"].(int), g.(map[string]any)["filled"].(int)
	if nw+ng == 0 {
		set("Automatic refill: water storage, generators, wind turbines and windtraps were already full.", false)
		return
	}
	var what []string
	for _, part := range []string{w.(map[string]any)["summary"].(string), g.(map[string]any)["summary"].(string)} {
		if part != "" {
			what = append(what, part)
		}
	}
	filledText := strings.Join(what, "; ")
	res, err := s.Save.Commit(false)
	if err != nil {
		set(fmt.Sprintf("Automatic refill prepared %s but could not save it: %v. It is waiting under Review & save.", filledText, err), true)
		return
	}
	backup := ""
	if b, ok := res["backup"].(string); ok {
		backup = filepath.Base(b)
	}
	msg := fmt.Sprintf("Automatic refill saved %s to the game (the previous file is backed up as %s).", filledText, backup)
	warn := false
	if w, ok := res["warning"].(string); ok && w != "" { // written, but the editor could not reload the file
		msg += " Warning: " + w
		warn = true
	}
	s.startupMu.Lock()
	s.lastAuto = time.Now().Format("2006-01-02 15:04") + ": " + strings.TrimSuffix(msg, ".")
	s.startupMu.Unlock()
	set(msg, warn)
}

// NoteStartup adds a sentence to what the page shows once when it opens (after what RunStartupTasks left, if anything). warn keeps it on
// screen until dismissed. The window has no console, so this is how a startup fact such as a changed port reaches a person who is not
// looking at a terminal.
func (s *Server) NoteStartup(msg string, warn bool) {
	s.startupMu.Lock()
	defer s.startupMu.Unlock()
	if s.startupNote != "" {
		s.startupNote += " "
	}
	s.startupNote += msg
	s.startupWarn = s.startupWarn || warn
}

func (s *Server) takeStartup() (string, bool) {
	s.startupMu.Lock()
	defer s.startupMu.Unlock()
	n, w := s.startupNote, s.startupWarn
	s.startupNote, s.startupWarn = "", false
	return n, w
}

func (s *Server) takeStartupNote() string { n, _ := s.takeStartup(); return n }

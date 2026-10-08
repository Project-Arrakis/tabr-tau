// Package web serves the local UI and JSON API.
package web

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"mime"
	"net"
	"net/http"
	"net/url"
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

	// AllowRemote permits non-loopback peers. Off by default; only set by an explicit operator flag.
	AllowRemote bool
}

func New(s *save.Save, cfg config.Dir) *Server {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed; refusing to start without an unpredictable token: " + err.Error())
	}
	srv := &Server{Save: s, Ops: &ops.Ops{S: s}, Cfg: cfg, token: hex.EncodeToString(b), mux: http.NewServeMux(), now: time.Now, boots: map[string]time.Time{}}
	srv.routes()
	return srv
}

const (
	cookieName = "tabr_session"
	bootTTL    = 10 * time.Minute
)

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

func (s *Server) hasSession(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	return err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.token)) == 1
}

// Handler enforces, in order: response security headers, loopback Host, loopback peer (unless AllowRemote),
// the session cookie on /api/, and for /api/ requests: same-origin fetch metadata / Origin, and a JSON
// Content-Type on anything that changes state. See docs/design (section 4.4) and findings F-04 / #5.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		if !hostAllowed(r.Host) {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if !s.AllowRemote && !isLoopbackPeer(r.RemoteAddr) {
			http.Error(w, "forbidden: this editor only accepts connections from this computer", http.StatusForbidden)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if !s.hasSession(r) {
				http.Error(w, "not signed in: open the link printed in the terminal", http.StatusForbidden)
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func respond(w http.ResponseWriter, v any, err error) {
	if err != nil {
		body := map[string]any{"error": err.Error()}
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
	// Static UI: three fixed asset paths, nothing else from the embedded folder is reachable.
	for path, ct := range map[string]string{"/app.css": "text/css; charset=utf-8", "/app.js": "text/javascript; charset=utf-8", "/html.js": "text/javascript; charset=utf-8"} {
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

	// Index: served only to a browser that holds the session cookie. A valid one-time boot token is exchanged for the
	// cookie and the URL is cleaned by redirect. Anyone else gets a static page that contains no secret.
	s.mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if boot := r.URL.Query().Get("boot"); boot != "" {
			if s.consumeBoot(boot) {
				// Deliberately no Secure flag: the editor speaks plain HTTP on loopback, where there is no network path to sniff,
				// and some browsers refuse to store Secure cookies from an http origin, which would lock every user out.
				// The cookie is HttpOnly, SameSite=Strict and session-scoped.
				http.SetCookie(w, &http.Cookie{Name: cookieName, Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode}) // nosemgrep: go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
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
	s.get("/api/catalog/items", func(r *http.Request) (any, error) { return catalog.All(), nil })
	s.get("/api/save/state", func(r *http.Request) (any, error) { return o.State() })

	// players
	s.get("/api/player", func(r *http.Request) (any, error) { return o.Player() })
	s.post("/api/player/teleport", o.Teleport)
	s.get("/api/player/inventory", func(r *http.Request) (any, error) { return o.Inventory() })
	s.post("/api/player/give", o.GiveItem)
	s.post("/api/player/repair", func(ops.Args) (any, error) { return o.RepairGear() })
	s.post("/api/player/refill", func(ops.Args) (any, error) { return o.RefillContainers() })
	s.post("/api/player/solari", o.AddSolari)
	s.post("/api/items/update", o.SetItem)
	s.post("/api/items/delete", o.DeleteItem)
	s.get("/api/player/factions", func(r *http.Request) (any, error) { return o.Factions() })
	s.post("/api/player/factions", o.SetReputation)
	s.get("/api/player/specs", func(r *http.Request) (any, error) { return o.Specs() })
	s.post("/api/player/specs", o.SetSpec)
	s.get("/api/player/journey", func(r *http.Request) (any, error) { return o.Journey(r.URL.Query().Get("q")) })
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
	s.post("/api/bases/refill-water", func(ops.Args) (any, error) { return o.RefillBaseWater() })
	s.post("/api/bases/refill-generators", func(ops.Args) (any, error) { return o.RefillGenerators() })
	s.post("/api/bases/health", o.SetPieceHealth)

	// vehicles / exchange / landsraad
	s.get("/api/vehicles", func(r *http.Request) (any, error) { return o.Vehicles() })
	s.post("/api/vehicles/bring", o.BringVehicle)
	s.post("/api/vehicles/repair", func(ops.Args) (any, error) { return o.RepairVehicles() })
	s.post("/api/vehicles/durability", o.SetRecoveredDurability)
	s.get("/api/exchange", func(r *http.Request) (any, error) { return o.Exchange() })
	s.post("/api/exchange/reset", o.ResetVendors)
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

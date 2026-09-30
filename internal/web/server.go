// Package web serves the local UI and JSON API.
package web

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

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
	token string
	mux   *http.ServeMux
}

func New(s *save.Save, cfg config.Dir) *Server {
	b := make([]byte, 16)
	rand.Read(b)
	srv := &Server{Save: s, Ops: &ops.Ops{S: s}, Cfg: cfg, token: hex.EncodeToString(b), mux: http.NewServeMux()}
	srv.routes()
	return srv
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "127.0.0.1" && host != "localhost" && host != "[::1]" && host != "::1" {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Header.Get("X-Tabr-Token") != s.token {
			http.Error(w, "missing or invalid token", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		s.mux.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func respond(w http.ResponseWriter, v any, err error) {
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
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
	// index (token injected)
	s.mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		b, _ := static.ReadFile("static/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(strings.Replace(string(b), "__TOKEN__", s.token, 1)))
	})

	s.get("/api/overview", func(r *http.Request) (any, error) { return o.Overview() })

	// players
	s.get("/api/player", func(r *http.Request) (any, error) { return o.Player() })
	s.post("/api/player/teleport", o.Teleport)
	s.get("/api/player/inventory", func(r *http.Request) (any, error) { return o.Inventory() })
	s.post("/api/player/give", o.GiveItem)
	s.post("/api/player/repair", func(ops.Args) (any, error) { return o.RepairGear() })
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
	s.post("/api/bases/health", o.SetPieceHealth)

	// vehicles / exchange / landsraad
	s.get("/api/vehicles", func(r *http.Request) (any, error) { return o.Vehicles() })
	s.post("/api/vehicles/bring", o.BringVehicle)
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
		w.Header().Set("Content-Disposition", `attachment; filename="`+strings.NewReplacer(`"`, "").Replace(r.URL.Query().Get("name"))+`.`+map[bool]string{true: "json", false: "csv"}[ct == "application/json"]+`"`)
		w.Write(b)
	})

	// save file lifecycle
	s.post("/api/save/commit", func(a ops.Args) (any, error) { return s.Save.Commit(a.Bool("force", false)) })
	s.post("/api/save/discard", func(ops.Args) (any, error) { return map[string]any{"ok": true}, s.Save.Discard() })
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

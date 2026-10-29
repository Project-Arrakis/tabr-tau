package web

import (
	"net/http"
	"strconv"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// The base backup routes (#475, phase 1): the save's base backups, and one of them as a base file. Both only read the save. The file is not written anywhere by the editor: it is
// the answer of the request, and the page hands it to the browser as a download, like the care package kit file, so the browser's own save dialog says where it goes and no path
// ever reaches the editor. Nothing here changes the save, so nothing is written, queued or asked first, and it works the same with the game closed or running.
func (s *Server) baseBackupRoutes() {
	s.get("/api/bases/backups", func(r *http.Request) (any, error) { return s.Ops.BaseBackups() })
	s.mux.HandleFunc("GET /api/bases/backups/export", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.URL.Query().Get("backup"), 10, 64)
		if err != nil || id <= 0 {
			respond(w, nil, apierr.New(apierr.ErrBadArgument, "backup must be the number of a base backup of this save"))
			return
		}
		out, err := s.Ops.ExportBaseBackup(id, s.now(), s.EditorVersion)
		if err != nil {
			respond(w, nil, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="`+safeFilename(out.FileName)+`"`)
		w.Write(out.Data) // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter (JSON built by encoding/json, sent as an attachment with nosniff)
	})
}

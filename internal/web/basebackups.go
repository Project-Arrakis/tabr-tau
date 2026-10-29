package web

import (
	"net/http"
	"strconv"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/basefile"
	"github.com/Project-Arrakis/tabr-tau/internal/ops"
)

// The base backup routes (#475): the save's base backups, and one of them as a base file (phase 1, both only read the save), and a base file imported as a new backup (phase 2, below). The file is not written anywhere by the editor: it is
// the answer of the request, and the page hands it to the browser as a download, like the care package kit file, so the browser's own save dialog says where it goes and no path
// ever reaches the editor. The list and the export change nothing, so nothing is written, queued or asked first, and they work the same with the game closed or running.
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

	// Import (#475, phase 2). "text" is the content of a base file; the editor reads and checks it whole, so a file that is not a good base file changes nothing. Preview works out what
	// an import would add by doing it in the edit and rolling the edit back: it never writes or queues anything. Import is that edit kept; it is the ordinary one, written after a
	// backup or queued while a game process exists (post does that), and it only adds rows, so it asks nothing first. It must be sent the "fileHash" the preview gave, and is
	// refused (409) when the text is not the file that was previewed.
	s.postMax("/api/bases/backups/import", baseImportBodyMax, func(a ops.Args) (any, error) {
		data, err := baseFileText(a)
		if err != nil {
			return nil, err
		}
		want, _ := a["fileHash"].(string)
		if want == "" {
			return nil, apierr.New(apierr.ErrBadArgument, "fileHash is missing: a file is imported as the preview showed it")
		}
		if want != basefile.Hash(data) {
			return nil, apierr.New(apierr.ErrConflict, "This is not the file that was previewed. Nothing was imported. Choose the file again.")
		}
		return s.Ops.ImportBaseBackup(data)
	})
	s.postSettingsMax("/api/bases/backups/import-preview", baseImportBodyMax, func(a ops.Args) (any, error) {
		data, err := baseFileText(a)
		if err != nil {
			return nil, err
		}
		return s.Ops.PreviewBaseImport(data)
	})
}

// baseImportBodyMax is the largest request body of the import routes: the 16 MiB a base file may be, written as a JSON string (quotes and line ends are escaped), with room.
const baseImportBodyMax = 40 << 20

func baseFileText(a ops.Args) ([]byte, error) {
	text, ok := a["text"].(string)
	if !ok {
		return nil, apierr.New(apierr.ErrBadArgument, "text must be the content of a base file")
	}
	return []byte(text), nil
}

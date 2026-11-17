package web

import (
	"net/http"

	"github.com/Project-Arrakis/tabr-tau/internal/buildinfo"
)

// notesFile is the name of the release notes the About panel shows when the file is next to the editor.
const notesFile = "notes.md"

// about answers GET /api/about: the version of this build (the header label and the About panel). It reads nothing from the save and takes no input but the one fixed query
// notes=1, which adds the text of notes.md from the editor's own folder when there is one. Nothing is fetched from anywhere.
func (s *Server) about(r *http.Request) (any, error) {
	i := buildinfo.Get()
	out := map[string]any{
		"version": i.Version, "label": i.Label(), "line": i.Line(), "build": i.Build, "commit": i.Commit, "date": i.Date,
		"channel": i.Channel, "arch": i.Arch, "dev": i.Dev,
	}
	if r.URL.Query().Get("notes") == "1" {
		text, cut := buildinfo.ReadNotes(s.NotesDir)
		out["notes"], out["notesTruncated"], out["notesFile"] = text, cut, notesFile
	}
	return out, nil
}

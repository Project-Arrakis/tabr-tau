package web

import (
	"net/http"

	"github.com/Project-Arrakis/tabr-tau/docs/help"
)

// helpAnswer answers GET /api/help: the Help pages embedded in this build (docs/help/*.md), in reading order, and the address of the same pages in the repository for the optional "also on
// GitHub" link. It reads nothing from the save, takes no input and fetches nothing from anywhere, so the Help tab works with no network (#980). The page draws the text with createElement and
// textContent only: a page is never markup.
func (s *Server) helpAnswer(r *http.Request) (any, error) {
	return map[string]any{"pages": help.Pages(), "repo": help.RepoURL}, nil
}

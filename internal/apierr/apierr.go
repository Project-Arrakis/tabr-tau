// Package apierr gives the editor's refusals a kind, so the layer that answers a request can pick a stable code and an HTTP status without reading the words of a
// message. A message is for a person and may change; the kind is what a caller can rely on. An error made here prints exactly the message it was given.
package apierr

import (
	"errors"
	"fmt"
)

// The kinds. They are values to compare with errors.Is.
var (
	ErrBadRequest  = errors.New("bad request")                             // the request itself is malformed (not JSON)
	ErrBadArgument = errors.New("bad argument")                            // a value the request carried is missing, of the wrong type, out of range or not allowed
	ErrNotFound    = errors.New("not found")                               // the thing the request names is not in the save
	ErrReadOnly    = errors.New("save is read-only")                       // the editor opened this save for reading only
	ErrGameBlocked = errors.New("the game may be using the save")          // the game-running check refused the write (or could not be made)
	ErrConflict    = errors.New("the data changed since it was read")      // an edit was made from a value that is not what the save holds now
	ErrNoRoom      = errors.New("no room for it")                          // the container has no free slot for what the request adds; nothing was changed
	ErrQueueHeld   = errors.New("queued changes are held")                 // edits waiting from before conflict with the game's newer file; nothing new is accepted
	ErrQueued      = errors.New("changes are still waiting to be written") // the action needs an empty queue
	ErrIntegrity   = errors.New("the edited save failed its own checks")   // a fault of the editor or the disk, not of the request: the integrity check or the round trip of the edited copy failed
	ErrWriteFailed = errors.New("the file could not be written")           // a fault of the disk or of another program holding the file, not of the request
)

// E is a message with a kind, and optionally the error that caused it (kept so that a caller can still look inside it, for instance for the paths it names).
type E struct {
	Msg   string
	Kind  error
	Cause error
}

func (e *E) Error() string        { return e.Msg }
func (e *E) Is(target error) bool { return target == e.Kind }
func (e *E) Unwrap() error        { return e.Cause }

// Wrap is New with the error that caused the refusal.
func Wrap(kind error, msg string, cause error) error { return &E{Msg: msg, Kind: kind, Cause: cause} }

// New makes an error of the given kind with exactly this message.
func New(kind error, msg string) error { return &E{Msg: msg, Kind: kind} }

// Newf is New with a format.
func Newf(kind error, format string, a ...any) error {
	return &E{Msg: fmt.Sprintf(format, a...), Kind: kind}
}

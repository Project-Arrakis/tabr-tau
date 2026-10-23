// Package apierr gives the editor's refusals a kind, so the layer that answers a request can pick a stable code and an HTTP status without reading the words of a
// message. A message is for a person and may change; the kind is what a caller can rely on. An error made here prints exactly the message it was given.
package apierr

import (
	"errors"
	"fmt"
)

// The kinds. They are values to compare with errors.Is.
var (
	BadRequest  = errors.New("bad request")                             // the request itself is malformed (not JSON)
	BadArgument = errors.New("bad argument")                            // a value the request carried is missing, of the wrong type, out of range or not allowed
	NotFound    = errors.New("not found")                               // the thing the request names is not in the save
	ReadOnly    = errors.New("save is read-only")                       // the editor opened this save for reading only
	GameBlocked = errors.New("the game may be using the save")          // the game-running check refused the write (or could not be made)
	QueueHeld   = errors.New("queued changes are held")                 // edits waiting from before conflict with the game's newer file; nothing new is accepted
	Queued      = errors.New("changes are still waiting to be written") // the action needs an empty queue
)

// E is a message with a kind.
type E struct {
	Msg  string
	Kind error
}

func (e *E) Error() string        { return e.Msg }
func (e *E) Is(target error) bool { return target == e.Kind }

// New makes an error of the given kind with exactly this message.
func New(kind error, msg string) error { return &E{Msg: msg, Kind: kind} }

// Newf is New with a format.
func Newf(kind error, format string, a ...any) error {
	return &E{Msg: fmt.Sprintf(format, a...), Kind: kind}
}

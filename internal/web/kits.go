package web

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/kits"
	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
)

// The care package routes (#417): the kits the editor keeps in its own settings file. These routes change that file and never the save, so they are not written to the save, are
// not queued and carry no outcome: they work the same with the game closed or running. Applying a kit to a character is a save edit (the last routes below).

// postSettings is post for a route that changes only the editor's own files, or nothing: no flush of the save, no outcome in the answer.
func (s *Server) postSettings(path string, fn func(a ops.Args) (any, error)) {
	s.mux.HandleFunc("POST "+path, func(w http.ResponseWriter, r *http.Request) {
		a, ok := postArgs(w, r)
		if !ok {
			return
		}
		v, err := fn(a)
		respond(w, v, err)
	})
}

// kitList is what the page needs to draw the kits: each with the reason it cannot be applied, when it cannot (a settings file edited by hand), and the limits.
func kitList(cur settings.Settings) map[string]any {
	list := make([]map[string]any, 0, len(cur.Kits))
	for _, k := range cur.Kits {
		k.Normalize()
		problem := ""
		if err := k.Validate(); err != nil {
			problem = err.Error()
		}
		list = append(list, map[string]any{"name": k.Name, "items": k.Items, "xp": k.XP, "skillPoints": k.SkillPoints, "problem": problem})
	}
	return map[string]any{"kits": list, "problem": "", "limits": map[string]any{"kits": kits.MaxKits, "items": kits.MaxLines, "name": kits.MaxNameRunes, "quantity": kits.MaxQuantity,
		"xp": kits.MaxXP, "skillPoints": kits.MaxSkillPoints, "fileBytes": kits.MaxFileBytes}}
}

// kitArg reads the kit of a request: an object with the keys of a kit and no others.
func kitArg(raw any) (kits.Kit, error) {
	var k kits.Kit
	if _, ok := raw.(map[string]any); !ok {
		return k, apierr.New(apierr.ErrBadArgument, "kit must be an object")
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return k, apierr.New(apierr.ErrBadArgument, "kit could not be read")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&k); err != nil {
		return k, apierr.Newf(apierr.ErrBadArgument, "kit: %v", err)
	}
	return k, nil
}

// changeKits applies fn to the stored kits under the settings lock and answers with the list as written plus what extra says (called after fn, so it can use what fn worked out).
// A refusal of the kit rules is a bad argument (422). A damaged settings file is reported and left alone, like every setting.
func (s *Server) changeKits(fn func(list []kits.Kit) ([]kits.Kit, error), extra func() map[string]any) (any, error) {
	next, err := settings.Update(s.SettingsPath, func(cur *settings.Settings) error {
		list, err := fn(cur.Kits)
		if err != nil {
			return apierr.Newf(apierr.ErrBadArgument, "%v", err)
		}
		cur.Kits = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := kitList(next)
	for k, v := range extra() {
		out[k] = v
	}
	return out, nil
}

// kitNamed is the kit a request names, read from the settings file (a damaged file is reported, not guessed at).
func (s *Server) kitNamed(a ops.Args) (kits.Kit, error) {
	cur, err := settings.LoadStrict(s.SettingsPath)
	if err != nil {
		return kits.Kit{}, err
	}
	k, err := kits.Get(cur.Kits, a.Str("name"))
	if err != nil {
		return k, apierr.Newf(apierr.ErrBadArgument, "%v", err)
	}
	return k, nil
}

func (s *Server) kitRoutes() {
	s.get("/api/kits", func(r *http.Request) (any, error) {
		cur, note, perr := settings.LoadNote(s.SettingsPath)
		out := kitList(cur)
		out["notice"] = s.logSettingsNote(note)
		if perr != nil { // reported, not hidden: changing a kit is refused until the file is fixed or removed
			out["problem"] = perr.Error()
		}
		return out, nil
	})
	// save: a new kit (no "was"), or the kit called "was" replaced by "kit" (and renamed when the name differs).
	s.postSettings("/api/kits/save", func(a ops.Args) (any, error) {
		k, err := kitArg(a["kit"])
		if err != nil {
			return nil, err
		}
		was := a.Str("was")
		k.Normalize()
		return s.changeKits(func(list []kits.Kit) ([]kits.Kit, error) { return kits.Put(list, was, k) },
			func() map[string]any { return map[string]any{"ok": true, "saved": k.Name} })
	})
	s.postSettings("/api/kits/delete", func(a ops.Args) (any, error) {
		name := a.Str("name")
		return s.changeKits(func(list []kits.Kit) ([]kits.Kit, error) { return kits.Delete(list, name) },
			func() map[string]any { return map[string]any{"ok": true, "deleted": name} })
	})
	s.postSettings("/api/kits/duplicate", func(a ops.Args) (any, error) {
		var copied string
		return s.changeKits(func(list []kits.Kit) ([]kits.Kit, error) {
			out, cp, err := kits.Duplicate(list, a.Str("name"))
			copied = cp.Name
			return out, err
		}, func() map[string]any { return map[string]any{"ok": true, "saved": copied} })
	})
	// apply: the kit called "name" goes to the character as one edit, written after a backup or queued while a game process exists, like every edit of the save (post does that).
	// preview: the same, worked out and rolled back: what would be given and what would not fit. It changes nothing, so it is never written or queued.
	s.post("/api/kits/apply", func(a ops.Args) (any, error) {
		k, err := s.kitNamed(a)
		if err != nil {
			return nil, err
		}
		return s.Ops.ApplyKit(k, false)
	})
	s.postSettings("/api/kits/preview", func(a ops.Args) (any, error) {
		k, err := s.kitNamed(a)
		if err != nil {
			return nil, err
		}
		return s.Ops.ApplyKit(k, true)
	})
	// export: the kit file for the kits named in the query (all when none is), as the JSON object the page offers as a download. It reads the settings and writes nothing.
	s.get("/api/kits/export", func(r *http.Request) (any, error) {
		cur, err := settings.LoadStrict(s.SettingsPath)
		if err != nil {
			return nil, err
		}
		b, err := kits.Export(cur.Kits, r.URL.Query()["name"])
		if err != nil {
			return nil, apierr.Newf(apierr.ErrBadArgument, "%v", err)
		}
		return json.RawMessage(b), nil
	})
	// import: "text" is the content of a kit file. The server reads and checks it whole (the page does not): a file that is not a good kit file changes nothing. Kits of the
	// file whose name is taken are kept under another name, never in place of the kit that is there.
	s.postSettings("/api/kits/import", func(a ops.Args) (any, error) {
		text, ok := a["text"].(string)
		if !ok {
			return nil, apierr.New(apierr.ErrBadArgument, "text must be the content of a kit file")
		}
		in, err := kits.Parse([]byte(text))
		if err != nil {
			return nil, apierr.Newf(apierr.ErrBadArgument, "%v", err)
		}
		renamed := 0
		return s.changeKits(func(list []kits.Kit) ([]kits.Kit, error) {
			out, n, err := kits.Merge(list, in)
			renamed = n
			return out, err
		}, func() map[string]any { return map[string]any{"ok": true, "imported": len(in), "renamed": renamed} })
	})
}

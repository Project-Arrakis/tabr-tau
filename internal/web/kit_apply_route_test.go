package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/kits"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Apply kit (#417) over HTTP: the kit is read from the settings file by name and goes to the character as ONE edit through the ordinary write path, so it is written after a backup with a
// line in the write log, or queued while a game process exists and then nothing is written. A preview is a dry run: it answers what the apply would do and changes nothing.

const levelSQL = `insert into fgl_entities(entity_id,components) values (-10, jsonb('{"FLevelComponent":[[],{"ModuleData":{},"TotalXPEarned":0,"TotalSkillPoints":0,"UnspentSkillPoints":0}]}'));
	insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (2,-10,'DuneCharacter');`

func applyServer(t *testing.T) (*Server, string) {
	t.Helper()
	s := New(testsave.PlayerWithSQL(t, levelSQL), config.Dir{Path: t.TempDir()})
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	list, err := kits.Put(nil, "", kits.Kit{Name: "Starter", Items: []kits.Line{{Item: "SpicedFuelCell", Quantity: 300}, {Item: "Combat_Heavy_Unique_Reinforced_Helmet_06", Quantity: 1, Grade: 3, Augments: []string{"T6_Augment_Armor10"}, AugmentGrade: 4}}, XP: 215, SkillPoints: 3})
	if err != nil {
		t.Fatal(err)
	}
	if err := settings.Save(s.SettingsPath, settings.Settings{Kits: list}); err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func fileState(t *testing.T, path string) (fuel, helmets, xp int64) {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	r, err := chk.One(`select coalesce(sum(case when template_id='SpicedFuelCell' then stack_size end),0) f, coalesce(sum(case when template_id like 'Combat_Heavy%' then 1 end),0) h from items`)
	if err != nil || r == nil {
		t.Fatal(err)
	}
	x, err := chk.One(`select json_extract(components,'$.FLevelComponent[1].TotalXPEarned') xp from fgl_entities where entity_id=-10`)
	if err != nil || x == nil {
		t.Fatal(err)
	}
	return r["f"].(int64), r["h"].(int64), x["xp"].(int64)
}

func TestApplyKitIsWrittenOnceWithABackupAndOneLineInTheWriteLog(t *testing.T) {
	s, dir := applyServer(t)
	w := postEdit(t, s, "/api/kits/apply", applyBody(t, s, "starter"))
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true || m["outcome"] != "written" || m["given"] != float64(301) || m["complete"] != true || m["dryRun"] != false {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if f, h, xp := fileState(t, s.Save.Path); f != 300 || h != 1 || xp != 215 {
		t.Fatalf("the file holds fuel=%d helmets=%d xp=%d", f, h, xp)
	}
	if len(s.Save.Backups()) != 1 {
		t.Fatalf("one backup: %d", len(s.Save.Backups()))
	}
	b, err := os.ReadFile(filepath.Join(dir, "write-log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var l struct {
		Edits int      `json:"edits"`
		Ops   []string `json:"ops"`
	}
	if err := json.Unmarshal(b, &l); err != nil || l.Edits != 1 || len(l.Ops) != 1 || l.Ops[0] != `Apply kit "Starter": 2 items, 301 units, +215 XP, +3 skill points` {
		t.Fatalf("one edit, one line: %v %s", err, b)
	}
}

// The regression the operator asked for: nothing writes the save while a game process exists (#419, #421), in the situations that are not the menu (#449). The kit is accepted and queued as one edit.
func TestApplyKitWhileAGameProcessExistsIsQueuedAndWritesNothing(t *testing.T) {
	for name, hold := range map[string]func() func(){
		"a single-player session is active":       save.ActiveSessionForTests,
		"left the world 23 s ago (the #419 case)": save.GameJustLeftWorldForTests,
		"the game check cannot say":               func() func() { return save.FailedGameCheckForTests("process list unavailable") },
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := applyServer(t)
			before, _ := os.ReadFile(s.Save.Path)
			restore := hold()
			defer restore()
			w := postEdit(t, s, "/api/kits/apply", applyBody(t, s, "Starter"))
			m := decodeBody(t, w)
			if w.Code != 200 || m["ok"] != true || m["outcome"] != "queued" || m["given"] != float64(301) {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			after, _ := os.ReadFile(s.Save.Path)
			if string(before) != string(after) {
				t.Fatal("the save file was written while the game was running")
			}
			if len(s.Save.Backups()) != 0 {
				t.Fatal("a backup was made for a write that did not happen")
			}
			if p := s.Save.Pending(); len(p) != 1 || p[0] != `Apply kit "Starter": 2 items, 301 units, +215 XP, +3 skill points` {
				t.Fatalf("queued as one edit: %v", p)
			}
		})
	}
}

func TestAQueuedKitIsWrittenOnceTheGameHasClosed(t *testing.T) {
	s, _ := applyServer(t)
	restore := save.ActiveSessionForTests()
	postEdit(t, s, "/api/kits/apply", applyBody(t, s, "Starter"))
	restore()
	if f := s.Save.Flush(); !f.Saved || f.Queued != 0 {
		t.Fatalf("%+v", f)
	}
	if f, h, xp := fileState(t, s.Save.Path); f != 300 || h != 1 || xp != 215 {
		t.Fatalf("fuel=%d helmets=%d xp=%d", f, h, xp)
	}
}

func TestAPreviewSaysWhatTheApplyWouldDoAndChangesNothing(t *testing.T) {
	s, dir := applyServer(t)
	// 3 free slots, 2,000 cells need 4: the preview says what would not fit
	list, _ := kits.Put(nil, "", kits.Kit{Name: "Big", Items: []kits.Line{{Item: "SpicedFuelCell", Quantity: 2000}}, XP: 40})
	settings.Save(s.SettingsPath, settings.Settings{Kits: list})
	before, _ := os.ReadFile(s.Save.Path)
	restore := save.ActiveSessionForTests() // a preview works while the game runs: it writes and queues nothing
	defer restore()
	w := postEdit(t, s, "/api/kits/preview", `{"name":"Big"}`)
	m := decodeBody(t, w)
	ng, _ := m["notGiven"].([]any)
	if w.Code != 200 || m["dryRun"] != true || m["outcome"] != nil || m["given"] != float64(1500) || m["requested"] != float64(2000) || m["complete"] != false || len(ng) != 1 ||
		ng[0].(map[string]any)["short"] != float64(500) || ng[0].(map[string]any)["reason"] != "the backpack is full" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if after, _ := os.ReadFile(s.Save.Path); string(before) != string(after) || s.Save.Dirty() || len(s.Save.Pending()) != 0 || len(s.Save.Backups()) != 0 {
		t.Fatal("a preview changed something")
	}
	if _, err := os.Stat(filepath.Join(dir, "write-log.jsonl")); err == nil {
		t.Fatal("a preview wrote to the write log")
	}
	// with an edit already queued, the preview still adds no outcome and does not try to write it
	if w := postEdit(t, s, "/api/player/solari", `{"amount":5}`); decodeBody(t, w)["outcome"] != "queued" {
		t.Fatalf("%s", w.Body.String())
	}
	if m := decodeBody(t, postEdit(t, s, "/api/kits/preview", `{"name":"Big"}`)); m["outcome"] != nil || m["dryRun"] != true {
		t.Fatalf("%v", m)
	}
	if n := len(s.Save.Pending()); n != 1 {
		t.Fatalf("the preview left the queue as it was: %v", s.Save.Pending())
	}
}

func TestApplyKitRefusals(t *testing.T) {
	s, _ := applyServer(t)
	for _, c := range []struct {
		route, body string
		code        int
		words       string
	}{
		{"/api/kits/apply", `{"name":"Missing"}`, http.StatusUnprocessableEntity, "no kit called"},
		{"/api/kits/apply", `{}`, http.StatusUnprocessableEntity, "no kit called"},
		{"/api/kits/preview", `{"name":"Missing"}`, http.StatusUnprocessableEntity, "no kit called"},
	} {
		w := postEdit(t, s, c.route, c.body)
		if m := decodeBody(t, w); w.Code != c.code || !strings.Contains(m["error"].(string), c.words) || m["code"] != "bad_argument" {
			t.Errorf("%s %s: %d %s", c.route, c.body, w.Code, w.Body.String())
		}
	}
	// a kit edited by hand into something invalid is refused, not applied in part
	os.WriteFile(s.SettingsPath, []byte(`{"kits":[{"name":"Hand made","items":[{"item":"NoSuchThing","quantity":1}],"xp":5}]}`), 0o600)
	w := postEdit(t, s, "/api/kits/apply", applyBody(t, s, "Hand made"))
	if m := decodeBody(t, w); w.Code != http.StatusUnprocessableEntity || !strings.Contains(m["error"].(string), "cannot be used") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	// a damaged settings file is reported, not guessed at
	os.WriteFile(s.SettingsPath, []byte(`{"kits": [`), 0o600)
	if w := postEdit(t, s, "/api/kits/apply", `{"name":"Starter","kitHash":"x"}`); w.Code == 200 || !strings.Contains(w.Body.String(), "damaged") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if f, h, xp := fileState(t, s.Save.Path); f != 0 || h != 0 || xp != 0 || s.Save.Dirty() {
		t.Fatal("a refused apply changed the save")
	}
}

func TestAKitThatFitsNothingIsA409AndWritesNothing(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, `update inventories set max_item_count=2 where id=1;`), config.Dir{Path: t.TempDir()})
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	list, _ := kits.Put(nil, "", kits.Kit{Name: "Fuel", Items: []kits.Line{{Item: "SpicedFuelCell", Quantity: 10}}})
	if err := settings.Save(s.SettingsPath, settings.Settings{Kits: list}); err != nil {
		t.Fatal(err)
	}
	w := postEdit(t, s, "/api/kits/apply", applyBody(t, s, "Fuel"))
	if m := decodeBody(t, w); w.Code != http.StatusConflict || m["code"] != "no_room" || m["outcome"] != nil {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if s.Save.Dirty() || len(s.Save.Backups()) != 0 {
		t.Fatal("nothing was written and no backup was made")
	}
}

// applyBody is the body the page sends to apply the kit called name: the name and the hash of the kit as the settings file holds it now.
func applyBody(t *testing.T, s *Server, name string) string {
	t.Helper()
	cur, err := settings.LoadStrict(s.SettingsPath)
	if err != nil {
		t.Fatal(err)
	}
	k, err := kits.Get(cur.Kits, name)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]any{"name": name, "kitHash": k.Hash()})
	return string(b)
}

// CR-3: the person confirms the dry run of one version of a kit. The apply is refused when the kit is not that version any more (edited, deleted and made again, replaced by an import),
// and it applies the kit when it is.
func TestApplyKitIsRefusedWhenTheKitChangedAfterTheDryRun(t *testing.T) {
	s, _ := applyServer(t)
	pv := decodeBody(t, postEdit(t, s, "/api/kits/preview", `{"name":"Starter"}`))
	hash, _ := pv["kitHash"].(string)
	if hash == "" {
		t.Fatalf("the dry run answers with the hash of the kit: %v", pv)
	}
	// the same name, a different kit (edited in another page)
	cur, _ := settings.LoadStrict(s.SettingsPath)
	list, err := kits.Put(cur.Kits, "Starter", kits.Kit{Name: "Starter", Items: []kits.Line{{Item: "SpicedFuelCell", Quantity: 500}}, XP: 9})
	if err != nil {
		t.Fatal(err)
	}
	if err := settings.Save(s.SettingsPath, settings.Settings{Kits: list}); err != nil {
		t.Fatal(err)
	}
	w := postEdit(t, s, "/api/kits/apply", `{"name":"Starter","kitHash":"`+hash+`"}`)
	if m := decodeBody(t, w); w.Code != http.StatusConflict || m["code"] != "conflict" || !strings.Contains(m["error"].(string), "has changed since you were asked") || m["outcome"] != nil {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if f, h, xp := fileState(t, s.Save.Path); f != 0 || h != 0 || xp != 0 || s.Save.Dirty() || len(s.Save.Backups()) != 0 {
		t.Fatal("a refused apply changed the save")
	}
	// asked again about the new kit, the apply goes through
	pv = decodeBody(t, postEdit(t, s, "/api/kits/preview", `{"name":"Starter"}`))
	if pv["kitHash"] == hash {
		t.Fatal("a different kit has the same hash")
	}
	if w := postEdit(t, s, "/api/kits/apply", `{"name":"Starter","kitHash":"`+pv["kitHash"].(string)+`"}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if f, _, xp := fileState(t, s.Save.Path); f != 500 || xp != 9 {
		t.Fatalf("fuel=%d xp=%d", f, xp)
	}
}

func TestApplyKitNeedsTheHashOfTheDryRun(t *testing.T) {
	s, _ := applyServer(t)
	w := postEdit(t, s, "/api/kits/apply", `{"name":"Starter"}`)
	if m := decodeBody(t, w); w.Code != http.StatusUnprocessableEntity || m["code"] != "bad_argument" || !strings.Contains(m["error"].(string), "kitHash") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if s.Save.Dirty() {
		t.Fatal("a refused apply changed the save")
	}
}

func TestKitHashIgnoresWhatNormalizeIgnoresAndSeesEverythingElse(t *testing.T) {
	a := kits.Kit{Name: " Starter ", Items: []kits.Line{{Item: " SpicedFuelCell", Quantity: 3, Augments: []string{"", "T6_Augment_Armor10"}}}, XP: 1}
	b := kits.Kit{Name: "Starter", Items: []kits.Line{{Item: "SpicedFuelCell", Quantity: 3, Augments: []string{"T6_Augment_Armor10"}}}, XP: 1}
	if a.Hash() != b.Hash() {
		t.Fatal("spaces and empty augment entries do not make another kit")
	}
	if a.Items[0].Item != " SpicedFuelCell" {
		t.Fatal("hashing changed the kit it was asked about")
	}
	for name, k := range map[string]kits.Kit{
		"name": {Name: "Other", Items: b.Items, XP: 1}, "quantity": {Name: "Starter", Items: []kits.Line{{Item: "SpicedFuelCell", Quantity: 4, Augments: b.Items[0].Augments}}, XP: 1},
		"xp": {Name: "Starter", Items: b.Items, XP: 2}, "points": {Name: "Starter", Items: b.Items, XP: 1, SkillPoints: 1},
	} {
		if k.Hash() == b.Hash() {
			t.Errorf("a different %s has the same hash", name)
		}
	}
}

// CR-7: a kit of the wrong kind from a hand-made request is answered in words a person can use, not with the decoder's Go type names.
func TestAKitOfTheWrongKindIsAnsweredWithoutGoTypeNames(t *testing.T) {
	s, _ := applyServer(t)
	for _, body := range []string{
		`{"kit":{"name":"X","items":[{"item":"SpicedFuelCell","quantity":"many"}]}}`,
		`{"kit":{"name":"X","items":[{"item":"SpicedFuelCell","quantity":1.5}]}}`,
		`{"kit":{"name":7,"items":[]}}`,
	} {
		w := postEdit(t, s, "/api/kits/save", body)
		m := decodeBody(t, w)
		msg, _ := m["error"].(string)
		if w.Code != http.StatusUnprocessableEntity || m["code"] != "bad_argument" || !strings.Contains(msg, "is of the wrong kind") || strings.Contains(msg, "Go ") || strings.Contains(msg, "int64") || strings.Contains(msg, "struct") {
			t.Errorf("%s: %d %s", body, w.Code, w.Body.String())
		}
	}
}

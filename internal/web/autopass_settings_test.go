package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/settings"
)

// GET and POST /api/settings with the switch of every automatic job (#352).

func settingsCall(t *testing.T, s *Server, method, body string) (int, map[string]any) {
	t.Helper()
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "application/json"}
	var b []byte
	if body != "" {
		b = []byte(body)
	}
	w := do(t, s, method, "http://127.0.0.1/api/settings", loop, h, b)
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m
}

func jobsOf(t *testing.T, m map[string]any) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	list, _ := m["jobs"].([]any)
	for _, j := range list {
		jm := j.(map[string]any)
		out[jm["key"].(string)] = jm
	}
	return out
}

// The list the card draws: every job once, in order, with label, description, key, whether it is ready and why not, and its switch.
func TestSettingsListsEveryJobWithItsSwitch(t *testing.T) {
	s := newTestServer(t)
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	_, m := settingsCall(t, s, "GET", "")
	if m["autoRefillOnOpen"] != false {
		t.Fatalf("the master switch defaults to off: %v", m)
	}
	list, _ := m["jobs"].([]any)
	if len(list) != len(settings.JobKeys) {
		t.Fatalf("%d jobs listed, %d keys", len(list), len(settings.JobKeys))
	}
	for i, j := range list {
		jm := j.(map[string]any)
		if jm["key"] != settings.JobKeys[i] || jm["label"] == "" || jm["desc"] == "" || jm["id"] == "" {
			t.Errorf("job %d: %v", i, jm)
		}
		if jm["on"] != true {
			t.Errorf("%v: every job defaults to on", jm["key"])
		}
		if (jm["ready"] == true) != (jm["reason"] == "") {
			t.Errorf("a job that is not ready says why, one that is does not: %v", jm)
		}
	}
}

// Each key round-trips: switched off it reads back off and is in the file, switched on again it reads back on.
func TestSettingsRoundTripsEachJobSwitch(t *testing.T) {
	s := newTestServer(t)
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	for _, k := range settings.JobKeys {
		code, m := settingsCall(t, s, "POST", `{"`+k+`":false}`)
		if code != 200 || jobsOf(t, m)[k]["on"] != false {
			t.Fatalf("%s off: %d %v", k, code, m)
		}
		if _, got := settingsCall(t, s, "GET", ""); jobsOf(t, got)[k]["on"] != false {
			t.Fatalf("%s did not stick: %v", k, got)
		}
		if settings.Load(s.SettingsPath).JobOn(k, true) {
			t.Fatalf("%s is not off in the file", k)
		}
		if code, m = settingsCall(t, s, "POST", `{"`+k+`":true}`); code != 200 || jobsOf(t, m)[k]["on"] != true {
			t.Fatalf("%s on: %d %v", k, code, m)
		}
	}
	// the master switch and a job in one request, and the other jobs stay as they were
	if code, m := settingsCall(t, s, "POST", `{"autoRefillOnOpen":true,"autoRepairGear":false}`); code != 200 || m["autoRefillOnOpen"] != true || jobsOf(t, m)["autoRepairGear"]["on"] != false {
		t.Fatalf("%d %v", code, m)
	}
	if st := settings.Load(s.SettingsPath); !st.AutoRefillOnOpen || st.JobOn("autoRepairGear", true) || !st.JobOn("autoRepairVehicles", true) {
		t.Fatalf("%+v", st)
	}
}

// A value that is not true or false is refused for every key, and nothing is written; so is a request with none of the keys.
func TestSettingsRefuseANonBooleanForEveryKey(t *testing.T) {
	s := newTestServer(t)
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	for _, k := range append([]string{"autoRefillOnOpen"}, settings.JobKeys...) {
		for _, bad := range []string{`"yes"`, `1`, `null`, `[true]`} {
			code, m := settingsCall(t, s, "POST", `{"`+k+`":`+bad+`}`)
			msg, _ := m["error"].(string)
			if code == 200 || !strings.Contains(msg, k) {
				t.Errorf("%s=%s must be refused naming the key: %d %v", k, bad, code, m)
			}
		}
	}
	if code, _ := settingsCall(t, s, "POST", `{"somethingElse":true}`); code == 200 {
		t.Error("a request naming no switch must be refused")
	}
	// one bad value among good ones changes none of them
	if code, _ := settingsCall(t, s, "POST", `{"autoRefillOnOpen":true,"autoRepairGear":"no"}`); code == 200 {
		t.Error("a mixed request with a bad value must be refused")
	}
	if _, err := os.Stat(s.SettingsPath); err == nil {
		t.Fatal("a refused request wrote the settings file")
	}
}

// A settings file written before the job switches existed keeps its master switch and gives every job its default (on); a switch that was
// written off stays off.
func TestAnOldSettingsFileGivesEveryJobItsDefault(t *testing.T) {
	s := newTestServer(t)
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(s.SettingsPath, []byte(`{"autoRefillOnOpen": true, "trackNumbers": {"Combat": 3}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, m := settingsCall(t, s, "GET", "")
	if m["autoRefillOnOpen"] != true {
		t.Fatalf("%v", m)
	}
	for k, j := range jobsOf(t, m) {
		if j["on"] != true {
			t.Errorf("%s must default to on", k)
		}
	}
	if code, _ := settingsCall(t, s, "POST", `{"autoRepairVehicles":false}`); code != 200 {
		t.Fatal(code)
	}
	st := settings.Load(s.SettingsPath)
	if !st.AutoRefillOnOpen || st.TrackNumbers["Combat"] != 3 || st.JobOn("autoRepairVehicles", true) || !st.JobOn("autoRepairGear", true) {
		t.Fatalf("%+v", st)
	}
}

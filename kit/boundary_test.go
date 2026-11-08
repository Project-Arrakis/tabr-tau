package kit

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const modulePath = "github.com/Project-Arrakis/tabr-tau/kit"

// goList runs `go list` in this module with the given arguments. GOFLAGS=-mod=readonly and GOWORK=off keep a developer's go.work
// (which may name other modules) out of the answer.
func goList(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"list"}, args...)...)
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=readonly", "GOWORK=off")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list %v: %v\n%s", args, err, errb.String())
	}
	return out.String()
}

// The module imports the standard library and itself, nothing else, in its code and in its tests. A third-party dependency would
// force every program that uses the module to move to the same version of it.
func TestOnlyTheStandardLibraryIsImported(t *testing.T) {
	out := goList(t, "-deps", "-test", "-f", "{{.ImportPath}}|{{.Standard}}", "./...")
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Split(line, "|")
		if len(f) != 2 {
			t.Fatalf("unexpected go list line %q", line)
		}
		// `go list -test` also names the test binaries ("p [p.test]", "p.test"): they belong to this module too.
		path, std := strings.TrimSuffix(strings.Fields(f[0])[0], ".test"), f[1] == "true"
		n++
		if std || path == modulePath || strings.HasPrefix(path, modulePath+"/") {
			continue
		}
		t.Errorf("import outside the standard library and this module: %s", path)
	}
	if n == 0 {
		t.Fatal("go list returned nothing")
	}
}

// No package imports an internal package of the editor, or any package of the editor's root module: nothing in the editor's tree may be
// needed to build this module.
func TestNothingFromTheEditorIsImported(t *testing.T) {
	out := goList(t, "-test", "-f", `{{.ImportPath}}: {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}`, "./...")
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		for _, imp := range strings.Fields(line)[1:] {
			if strings.HasPrefix(imp, "github.com/Project-Arrakis/") && !strings.HasPrefix(imp, modulePath) {
				t.Errorf("%s imports %s, a package outside this module", strings.SplitN(line, ":", 2)[0], imp)
			}
			if first, _, _ := strings.Cut(imp, "/"); strings.Contains(first, ".") && (strings.Contains(imp, "/internal/") || strings.HasSuffix(imp, "/internal")) {
				t.Errorf("%s imports %s, an internal package", strings.SplitN(line, ":", 2)[0], imp)
			}
		}
	}
}

// go.mod lists no requirement: the first line of a dependency would be the end of "standard library only".
func TestGoModRequiresNothing(t *testing.T) {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		w := strings.Fields(line)
		if len(w) > 0 && (w[0] == "require" || w[0] == "replace") {
			t.Errorf("go.mod must not carry %q", w[0])
		}
	}
	if _, err := os.Stat("go.sum"); err == nil {
		t.Error("go.sum must not exist in a module with no dependency")
	}
}

// The module's directory holds no Go package named internal.
func TestNoInternalDirectory(t *testing.T) {
	out := goList(t, "-f", "{{.Dir}}", "./...")
	for _, d := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.Contains(strings.ReplaceAll(d, "\\", "/"), "/internal") {
			t.Errorf("an internal package directory inside the module: %s", d)
		}
	}
}

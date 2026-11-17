package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// These tests prove the version stamp end to end: scripts/buildinfo.ps1 works the version out of git, and a program built with its flags says that version. The second half
// matters on its own: the names of the variables live in two places (the script and internal/buildinfo), and a renamed variable would stamp nothing, silently.

const buildinfoScript = "../../scripts/buildinfo.ps1"

func needTool(t *testing.T, names ...string) string {
	t.Helper()
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	t.Skipf("none of %v is installed", names)
	return ""
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false", "-C", dir}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-11-16T12:00:00+0000", "GIT_COMMITTER_DATE=2026-11-16T12:00:00+0000", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// stampRepo makes a small repository with a go.mod and returns its folder.
func stampRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/stamped\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "go.mod")
	gitIn(t, dir, "commit", "-q", "-m", "first")
	return dir
}

func commitAgain(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", name)
	gitIn(t, dir, "commit", "-q", "-m", name)
}

// runScript runs scripts/buildinfo.ps1 against a repository and returns the line it prints.
func runScript(t *testing.T, repo string, extra ...string) string {
	t.Helper()
	ps := needTool(t, "pwsh", "powershell")
	script, err := filepath.Abs(buildinfoScript)
	if err != nil {
		t.Fatal(err)
	}
	args := append([]string{"-NoProfile", "-NonInteractive", "-File", script, "-Repo", repo, "-AllowShallow"}, extra...)
	out, err := exec.Command(ps, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("buildinfo.ps1: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func stampOf(t *testing.T, flags, name string) string {
	t.Helper()
	m := regexp.MustCompile(`-X example\.com/stamped/internal/buildinfo\.` + name + `=(\S+)`).FindStringSubmatch(flags)
	if m == nil {
		t.Fatalf("no %s in %q", name, flags)
	}
	return m[1]
}

func TestBuildinfoScriptWorksOutTheVersion(t *testing.T) {
	needTool(t, "git")
	dir := stampRepo(t)
	version := func(extra ...string) string {
		return stampOf(t, runScript(t, dir, extra...), "Version")
	}

	// no tag yet: 0.0.0 plus the build number
	f := runScript(t, dir)
	if !strings.HasPrefix(f, "-H=windowsgui -s -w -X example.com/stamped/internal/buildinfo.Version=0.0.0+1 ") {
		t.Errorf("no tag: %q", f)
	}
	if stampOf(t, f, "Build") != "1" || stampOf(t, f, "Date") != "2026-11-16" || stampOf(t, f, "Commit") != gitIn(t, dir, "rev-parse", "--short=7", "HEAD") {
		t.Errorf("no tag: build, date or commit wrong in %q", f)
	}

	// exactly on a tag; tags of other modules are not release tags
	gitIn(t, dir, "tag", "v1.2.3")
	gitIn(t, dir, "tag", "design/v9.9.9")
	if v := version(); v != "1.2.3" {
		t.Errorf("on the tag: %q", v)
	}
	// commits after the tag
	commitAgain(t, dir, "a.txt")
	commitAgain(t, dir, "b.txt")
	f = runScript(t, dir)
	if v := stampOf(t, f, "Version"); v != "1.2.3+2" {
		t.Errorf("two commits after the tag: %q", v)
	}
	if stampOf(t, f, "Build") != "3" {
		t.Errorf("build number: %q", f)
	}
	// a tracked file changed: dirty
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := version(); v != "1.2.3+2.dirty" {
		t.Errorf("dirty after commits: %q", v)
	}
	gitIn(t, dir, "checkout", "--", "a.txt")
	// an untracked file does not make a tree dirty
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := version(); v != "1.2.3+2" {
		t.Errorf("untracked file: %q", v)
	}
	// dirty on the tag itself; a pre-release tag keeps its part
	gitIn(t, dir, "tag", "v1.3.0-rc.1")
	if v := version(); v != "1.3.0-rc.1" {
		t.Errorf("on a pre-release tag: %q", v)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := version(); v != "1.3.0-rc.1+dirty" {
		t.Errorf("dirty on a tag: %q", v)
	}
}

func TestBuildinfoScriptOptions(t *testing.T) {
	needTool(t, "git")
	dir := stampRepo(t)
	// the module comes from go.mod unless given, and the base flags can be changed or left out
	if f := runScript(t, dir, "-BaseFlags", ""); !strings.HasPrefix(f, "-X example.com/stamped/internal/buildinfo.Version=") {
		t.Errorf("no base flags: %q", f)
	}
	if f := runScript(t, dir, "-Module", "example.org/other"); !strings.Contains(f, "-X example.org/other/internal/buildinfo.Build=1 ") {
		t.Errorf("module given: %q", f)
	}
}

func repoModule(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^module\s+(\S+)\s*$`).FindSubmatch(b)
	if m == nil {
		t.Fatal("no module line in go.mod")
	}
	return string(m[1])
}

// TestStampReachesTheProgram builds the real program with the flags the script prints and runs it with --version.
func TestStampReachesTheProgram(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the program")
	}
	needTool(t, "git")
	goTool := needTool(t, "go")
	dir := stampRepo(t)
	gitIn(t, dir, "tag", "v1.2.3")
	commitAgain(t, dir, "a.txt")
	flags := runScript(t, dir, "-Module", repoModule(t), "-BaseFlags", "-s -w")
	commit := gitIn(t, dir, "rev-parse", "--short=7", "HEAD")

	exe := filepath.Join(t.TempDir(), "tabr-tau-stamped.exe")
	build := exec.Command(goTool, "build", "-trimpath", "-ldflags", flags, "-o", exe, "./cmd/tabr-tau")
	build.Dir = "../.."
	build.Env = append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	out, err := exec.Command(exe, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("--version: %v\n%s", err, out)
	}
	if want := "tabr-tau v1.2.3+1 (build 2, " + commit + ")"; strings.TrimSpace(string(out)) != want {
		t.Errorf("--version printed %q, want %q", strings.TrimSpace(string(out)), want)
	}
}

func TestVersionLineOfAnUnstampedBuild(t *testing.T) {
	if got := versionLine(); !strings.HasPrefix(got, "tabr-tau dev") {
		t.Errorf("versionLine() = %q, want it to start with tabr-tau dev", got)
	}
}

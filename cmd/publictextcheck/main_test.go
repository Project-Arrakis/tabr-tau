package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/hygiene"
)

// word is a word of this test's own; the check is given its digest through the extra-digest variable, as CI gives its private list.
const word = "zorblax"

// prof is a profile path as it sits inside a JSON string (backslashes doubled); the marker keeps this file from containing one itself.
var prof = strings.ReplaceAll(`C:\\Users@\\pat.smith\\x`, "@", "")

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

// runEvent runs the check over an event payload written to a temp file, and returns the exit code and both outputs.
func runEvent(t *testing.T, name, payload string, extra map[string]string) (int, string, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(p, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	e := map[string]string{"GITHUB_EVENT_NAME": name, "GITHUB_EVENT_PATH": p, "PUBLIC_TEXT_EXTRA_DIGESTS": hygiene.Digest(word)}
	for k, v := range extra {
		e[k] = v
	}
	var out, errb bytes.Buffer
	code := run(nil, env(e), &out, &errb)
	return code, out.String(), errb.String()
}

func TestAnIssueWithCleanTextPasses(t *testing.T) {
	code, out, errs := runEvent(t, "issues", `{"issue":{"number":7,"title":"The grid is wrong","body":"Steps:\n1. open the page\n2. read %USERPROFILE%\\x"}}`, nil)
	if code != 0 || !strings.Contains(out, "public text OK (2 text(s) checked)") || errs != "" {
		t.Fatalf("code=%d out=%q err=%q", code, out, errs)
	}
}

func TestAnIssueBodyWithAReservedWordFailsAndTheLogDoesNotRepeatIt(t *testing.T) {
	code, out, errs := runEvent(t, "issues", `{"issue":{"number":7,"title":"fine","body":"line one\nsee the `+strings.ToUpper(word)+` repo"}}`, nil)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(out, "issue #7 body, line 2: reserved word") {
		t.Errorf("the report does not name the place and the rule: %q", out)
	}
	if !strings.Contains(errs, "public text rule") || !strings.Contains(errs, "Edit the text") {
		t.Errorf("the failure message is not clear: %q", errs)
	}
	if strings.Contains(strings.ToLower(out+errs), word) {
		t.Errorf("the log repeats the reserved word: %q %q", out, errs)
	}
}

func TestAPullRequestTitleBodyBranchAndAttributionAreChecked(t *testing.T) {
	for name, payload := range map[string]string{
		"title":       `{"pull_request":{"number":9,"title":"add ` + word + ` support","body":"ok","head":{"ref":"fix/x"}}}`,
		"body":        `{"pull_request":{"number":9,"title":"ok","body":"mentions ` + word + `","head":{"ref":"fix/x"}}}`,
		"branch":      `{"pull_request":{"number":9,"title":"ok","body":"ok","head":{"ref":"fix/` + word + `"}}}`,
		"attribution": `{"pull_request":{"number":9,"title":"ok","body":"text\n\nGenerated with [Claude Code](https://x)","head":{"ref":"fix/x"}}}`,
		"trailer":     `{"pull_request":{"number":9,"title":"ok","body":"Co-Authored-By: A Bot <noreply@anthropic.com>","head":{"ref":"fix/x"}}}`,
		"profile":     `{"pull_request":{"number":9,"title":"ok","body":"logs in ` + prof + `","head":{"ref":"fix/x"}}}`,
	} {
		want := map[string]string{
			"title": "pull request #9 title, line 1: reserved word", "body": "pull request #9 body, line 1: reserved word",
			"branch": "pull request #9 branch name, line 1: reserved word", "attribution": "pull request #9 body, line 3: AI attribution line",
			"trailer": "pull request #9 body, line 1: AI attribution line", "profile": "pull request #9 body, line 1: path under a personal profile",
		}[name]
		code, out, _ := runEvent(t, "pull_request", payload, nil)
		if code != 1 || !strings.Contains(out, want) {
			t.Errorf("%s: code=%d out=%q, want %q", name, code, out, want)
		}
	}
	// the attribution rule is for a pull request only; an issue may talk about it
	if code, out, _ := runEvent(t, "issues", `{"issue":{"number":1,"title":"t","body":"Generated with a tool"}}`, nil); code != 0 {
		t.Errorf("an issue body about attribution failed: %q", out)
	}
}

func TestCommentsReviewsAndCommentsOnAPullRequestAreChecked(t *testing.T) {
	cases := map[string]string{
		"issue_comment":               `{"issue":{"number":3,"title":"t","body":"b"},"comment":{"body":"look at ` + word + `"}}`,
		"pull_request_review":         `{"pull_request":{"number":3,"title":"t","body":"b","head":{"ref":"x"}},"review":{"body":"see ` + word + `"}}`,
		"pull_request_review_comment": `{"pull_request":{"number":3,"title":"t","body":"b","head":{"ref":"x"}},"comment":{"body":"` + word + `"}}`,
	}
	for name, payload := range cases {
		code, out, _ := runEvent(t, name, payload, nil)
		if code != 1 || !strings.Contains(out, name) {
			t.Errorf("%s: code=%d out=%q", name, code, out)
		}
	}
	// a comment on a pull request carries the pull_request key of the issue object
	code, out, _ := runEvent(t, "issue_comment", `{"issue":{"number":3,"title":"t","body":"b","pull_request":{"url":"u"}},"comment":{"body":"fine"}}`, nil)
	if code != 0 {
		t.Errorf("a clean comment on a pull request failed: %q", out)
	}
}

func TestWithoutTheExtraListOnlyTheBuiltInDigestsApply(t *testing.T) {
	code, out, _ := runEvent(t, "issues", `{"issue":{"number":1,"title":"`+word+`","body":"b"}}`, map[string]string{"PUBLIC_TEXT_EXTRA_DIGESTS": ""})
	if code != 0 {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestBadInputsAreRefusedNotPassed(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(nil, env(nil), &out, &errb); code != 2 {
		t.Errorf("no event and no mode: code = %d, want 2 (a check that read nothing must not pass)", code)
	}
	if code, _, _ := runEvent(t, "issues", `not json`, nil); code != 2 {
		t.Errorf("a payload that is not JSON: code = %d, want 2", code)
	}
	if code, _, _ := runEvent(t, "push", `{}`, nil); code != 2 {
		t.Errorf("an event with no text: code = %d, want 2", code)
	}
	if code := run([]string{"-nope"}, env(nil), &out, &errb); code != 2 {
		t.Errorf("an unknown argument: code = %d, want 2", code)
	}
	if code := run([]string{"-commits", filepath.Join(t.TempDir(), "missing.log")}, env(nil), &out, &errb); code != 2 {
		t.Errorf("a commit log that cannot be read: code = %d, want 2 (a check that read nothing must not pass)", code)
	}
}

func TestShowIsRefusedInCI(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"-show", "-items", "x"}, env(map[string]string{"GITHUB_ACTIONS": "true"}), &out, &errb); code != 2 || !strings.Contains(errb.String(), "refused in CI") {
		t.Errorf("code=%d err=%q", code, errb.String())
	}
}

func TestDigestMode(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"-digest", "Zorblax"}, env(nil), &out, &errb); code != 0 || strings.TrimSpace(out.String()) != hygiene.Digest(word) {
		t.Errorf("code=%d out=%q", code, out.String())
	}
	if code := run([]string{"-digest"}, env(nil), &out, &errb); code != 2 {
		t.Errorf("-digest with no word: code = %d, want 2", code)
	}
}

func TestItemsFileScansIssuesPullRequestsAndComments(t *testing.T) {
	items := `[
 {"number":1,"title":"clean","body":"clean","comments":[{"body":"clean"}]},
 {"number":2,"title":"clean","body":"a ` + word + ` here","comments":[]},
 {"number":3,"title":"clean","body":"clean","comments":[{"body":"fine"},{"body":"and ` + word + `"}]},
 {"number":4,"title":"clean","body":"clean","headRefName":"x/` + word + `","reviews":[{"body":"` + word + `"}]},
 {"number":5,"title":"` + prof + `","body":null,"comments":[]}
]`
	p := filepath.Join(t.TempDir(), "items.json")
	if err := os.WriteFile(p, []byte(items), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	code := run([]string{"-items", p}, env(map[string]string{"PUBLIC_TEXT_EXTRA_DIGESTS": hygiene.Digest(word)}), &out, &errb)
	got := out.String()
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	for _, want := range []string{"#2 body, line 1: reserved word", "#3 comment 2, line 1: reserved word", "#4 branch name, line 1: reserved word", "#4 review 1, line 1: reserved word", "#5 title, line 1: path under a personal profile"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "#1 ") {
		t.Errorf("the clean item #1 was reported: %q", got)
	}
	if strings.Contains(strings.ToLower(got), word) {
		t.Errorf("the report repeats the word without -show: %q", got)
	}
	out.Reset()
	run([]string{"-show", "-items", p}, env(map[string]string{"PUBLIC_TEXT_EXTRA_DIGESTS": hygiene.Digest(word)}), &out, &errb)
	if !strings.Contains(out.String(), word) {
		t.Errorf("-show did not print the text: %q", out.String())
	}
}

func TestCommitMessagesOfAGitLog(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	git := func(args ...string) []byte {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		b, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, b)
		}
		return b
	}
	git("init", "-q")
	git("commit", "-q", "--allow-empty", "-m", "base")
	git("commit", "-q", "--allow-empty", "-m", "a clean message refs #1")
	git("commit", "-q", "--allow-empty", "-m", "mentions "+word)
	logFile := func(rng string) string {
		p := filepath.Join(t.TempDir(), "commits.log")
		if err := os.WriteFile(p, git("log", "--format=%H%x00%B%x01", rng), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	var out, errb bytes.Buffer
	code := run([]string{"-commits", logFile("HEAD~2..HEAD")}, env(map[string]string{"PUBLIC_TEXT_EXTRA_DIGESTS": hygiene.Digest(word)}), &out, &errb)
	if code != 1 || strings.Count(out.String(), "reserved word") != 1 || !strings.Contains(out.String(), "message, line 1: reserved word") || strings.Contains(strings.ToLower(out.String()), word) {
		t.Errorf("code=%d out=%q", code, out.String())
	}
	out.Reset()
	if code := run([]string{"-commits", logFile("HEAD~2..HEAD~1")}, env(nil), &out, &errb); code != 0 || !strings.Contains(out.String(), "1 text(s) checked") {
		t.Errorf("code=%d out=%q", code, out.String())
	}
}

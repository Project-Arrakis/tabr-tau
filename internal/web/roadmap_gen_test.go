package web

import (
	"os/exec"
	"strings"
	"testing"
)

// CR-11: scripts/roadmap-gen.sh writes issue titles into docs/roadmap.md, a committed page. Anyone who can open an issue writes a title, so the script must pass it on as plain
// text: no link, no image, no HTML tag, no emphasis, no bare web address, and the pipe does not end the cell.
func TestRoadmapGeneratorPassesATitleOnAsPlainText(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil || strings.Contains(strings.ToLower(bash), "system32") { // on Windows that one is the Linux subsystem's launcher, not the shell the script is written for
		t.Skip("no bash to run the script with")
	}
	cases := map[string]string{
		"[click here](https://evil.example/x)": `\[click here\](https&#58;//evil.example/x)`,
		`<img src=x onerror=alert(1)>`:         `&lt;img src=x onerror=alert(1)&gt;`,
		"![pixel](https://evil.example/p.png)": `\!\[pixel\](https&#58;//evil.example/p.png)`,
		"a | b":                                `a \| b`,
		"`code` *em* _em_ ~~s~~":               "\\`code\\` \\*em\\* \\_em\\_ \\~\\~s\\~\\~",
		`back\slash & more`:                    `back\\slash &amp; more`,
		"see www.example.com":                  `see www&#46;example.com`,
		"Bug: the automatic pass wrote the save (game running); six items": "Bug: the automatic pass wrote the save (game running); six items",
	}
	for in, want := range cases {
		cmd := exec.Command(bash, "-c", `eval "$(sed -n '/^cell_text()/,/^}/p' ../../scripts/roadmap-gen.sh)"; cell_text "$1"`, "_", in)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got := strings.TrimRight(string(out), "\r\n"); got != want {
			t.Errorf("%q\n got  %s\n want %s", in, got, want)
		}
	}
}

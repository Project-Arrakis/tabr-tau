package save

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"time"
)

// A pin of the session tracker over a seeded corpus of 60000 log lines, taken from the editor's own state machine before it moved to kit/sessionlog (#565): after every line,
// whether a session is active, when it ended and when the game last wrote its database. Lines without a time stamp count as "now", which is written as N.

func spPinCorpus() []string {
	rng := rand.New(rand.NewPCG(1, 2))
	prefixes := []string{"", "[x]", "[2026.10.08-17.57.43:218][ 14][1][0]", "[2026.10.08-17.57.44:218]", "[2026.10.08-17.58.01:218][1]", "[a][b]", "[2026.10.09-01.02.03:004][9][9][9]", " [x]", "\xef\xbb\xbf", "[" + strings.Repeat("q", 41) + "]"}
	cats := []string{"", "LogNet: Display: ", "LogChat: Display: Player: ", "LogOther: Display: ", "LogWindows: Log: ", "Log: ", "LogLowLevelPersistence: DB(game.db:0): "}
	bodies := []string{
		"Browse (net mode 0): /Game/Dune/Maps/Arrakis/SOC_1/Survival_1?listen",
		"Browse (net mode 2): /Game/Dune/Maps/SocialHubs/Arrakeen/SH_Arrakeen?listen?flowtype=Travel",
		"Browse (net mode 0): /Game/Dune/Maps/CharacterSelectionCave/CB_CS_Cave?Name=Player",
		"Browse (net mode 2): /Game/Dune/Maps/CharacterSelectionCave/CB_CS_Cave?closed",
		"Browse (net mode 0): 172.240.109.140:7807/Game/Dune/Maps/CB_CS_Cave?flowtype=Login",
		"Browse (net mode 0): 1.2.3.4:7807/Game/X?listen",
		"Browse (net mode 0): /Game/X?listen?closed",
		"Browse (net mode 0): ", "Browse (net mode 0", "Browse (net mode 0): /Game/X): 1.2.3.4/Game?listen",
		"Delaying Travel (net mode Listen Server): ?closed?delayed_travel",
		"Delaying Travel (net mode Listen Server): /Game/Dune/Maps/ChallengeRoom/X?delayed_travel",
		"FPlatformMisc::RequestExit(0, <NoCallSiteInfo>)", "Log file closed",
		"SaveDatabase - Database saved to disk. Saving again in 60 seconds.",
		"OtherClient: Initialized", "nothing of interest", "",
	}
	lines := make([]string, 60000)
	for i := range lines {
		l := prefixes[rng.IntN(len(prefixes))] + cats[rng.IntN(len(cats))] + bodies[rng.IntN(len(bodies))]
		if rng.IntN(6) == 0 {
			l += " " + cats[rng.IntN(len(cats))] + bodies[rng.IntN(len(bodies))]
		}
		lines[i] = l
	}
	return lines
}

func spPinHash(feed func(string) (bool, time.Time, time.Time)) string {
	h := sha256.New()
	stamp := func(t time.Time) string {
		switch {
		case t.IsZero():
			return "0"
		case time.Since(t) < 5*time.Second && time.Since(t) > -5*time.Second:
			return "N"
		}
		return fmt.Sprint(t.Unix())
	}
	for _, l := range spPinCorpus() {
		active, ended, saved := feed(l)
		fmt.Fprintf(h, "%v|%s|%s\n", active, stamp(ended), stamp(saved))
	}
	return hex.EncodeToString(h.Sum(nil))
}

const spPinHashWant = "675938c6c84d822167c3013ef8f5598dfe264b43ed93e86501fcc77aff694c0b"

func TestPinTrackerOverASeededCorpus(t *testing.T) {
	var tr spTracker
	got := spPinHash(func(l string) (bool, time.Time, time.Time) {
		tr.feed(l)
		return tr.SPActive, tr.EndedAt, tr.LastSaveAt
	})
	if got != spPinHashWant {
		t.Fatalf("the tracker answers differently for some line of the corpus: hash %s, pinned %s", got, spPinHashWant)
	}
}

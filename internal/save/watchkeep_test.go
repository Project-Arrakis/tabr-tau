package save

import (
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// #821 (and #779, LOW 2, with the real watch): the preparation of the write (the hash of the file, the integrity check, the encode and its round trip) takes longer than the
// watch's gap, as it can on a large save. Without anything asking the gate meanwhile, the one question after it is the first evaluation after a gap: the watch starts over and the
// write is refused, before any backup is taken, and again on every attempt. With keepWatching the gate is evaluated every few seconds while the write runs, the watch stays
// unbroken, and the write goes through.
//
// The clock of the test is its own: it moves only by the steps the test makes by hand, never with the real time, so how fast the runner is cannot change the outcome (#898: with
// a clock that ran 300 times faster than the real one, a tick that came 100 ms late on a busy runner was a gap in the watch). The stalled preparation steps the clock 10 s at a
// time, and only after the ticker has evaluated the gate once more; so with the ticker, no two evaluations are ever more than one step apart, however late a tick comes, and
// without it nothing evaluates the gate while the clock moves.
func TestAWriteWhosePreparationOutlastsTheWatchKeepsTheWatch(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ticker bool
	}{{"keepWatching asks the gate meanwhile", true}, {"nothing asks meanwhile (the refusal #779 keeps from taking a backup)", false}} {
		t.Run(tc.name, func(t *testing.T) {
			p := makeSave(t, `create table t(a integer); insert into t values (1);`)
			setAge(t, p, time.Hour)
			s, err := Open(p)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			orig, _ := os.ReadFile(p)
			gateStubs(t)
			oldObs, oldEvery, oldPrepared := observeQuietFor, watchEvery, prepared
			observeQuietFor = menuQuietAfterWrite
			t.Cleanup(func() {
				observeQuietFor, watchEvery, prepared = oldObs, oldEvery, oldPrepared
				forgetObserved(p)
				forgetOwnWrite(p)
			})
			forgetObserved(p)
			watchEvery = time.Millisecond
			if !tc.ticker {
				watchEvery = time.Hour
			}
			base := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
			var stepped atomic.Int64 // the steps the test makes by hand, in nanoseconds of the test's clock
			var asked atomic.Int64   // how many times the gate has been evaluated far enough to read the log's status
			clock := func() time.Time {
				return base.Add(time.Duration(stepped.Load()))
			}
			nowFunc = clock
			mod := base.Add(-48 * time.Hour) // one time for the whole test: a changing time would restart the watch itself
			fileTimeFor = func(string) (time.Time, bool, error) { return mod, false, nil }
			statusFor = func(string) (spStatus, error) {
				asked.Add(1)
				now := clock()
				return spStatus{EndedAt: now.Add(-time.Hour), LastSaveAt: now.Add(-time.Hour + 5*time.Second)}, nil
			}
			for i := 0; i < 8; i++ { // watched for 70 s without a gap
				stepped.Add(int64(10 * time.Second))
				processState(p, true)
			}
			if g := processState(p, true); g.Blocked {
				t.Fatalf("the warm-up should have passed the watch: %+v", g)
			}
			setA(t, s, 2)
			// The preparation takes 120 s of the test's clock, four gaps. With the ticker each step waits for one more evaluation of the gate (the real time it takes does not matter;
			// the limit only keeps a ticker that never comes from hanging the test); without it the clock moves at once.
			prepared = func() {
				for i := 0; i < 12; i++ {
					if tc.ticker {
						want := asked.Load() + 1
						for deadline := time.Now().Add(30 * time.Second); asked.Load() < want; time.Sleep(time.Millisecond) {
							if time.Now().After(deadline) {
								t.Error("keepWatching did not evaluate the gate while the write was prepared")
								return
							}
						}
					}
					stepped.Add(int64(10 * time.Second))
				}
			}
			f := s.Flush()
			if tc.ticker {
				if !f.Saved {
					t.Fatalf("a slow preparation broke the watch, and the write was refused: %+v", f)
				}
				return
			}
			if f.Saved || !strings.Contains(f.Why, "only just started watching") {
				t.Fatalf("without the ticker the write after a slow preparation must be refused by the watch: %+v", f)
			}
			if ents, _ := os.ReadDir(s.backupDir()); len(ents) != 0 {
				t.Fatalf("a refused write took %d backup(s)", len(ents))
			}
			if now, _ := os.ReadFile(p); string(now) != string(orig) {
				t.Fatal("the file changed")
			}
			if !s.Dirty() {
				t.Fatal("the edit must stay queued")
			}
		})
	}
}

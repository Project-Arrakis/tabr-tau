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
// a clock that ran 300 times faster than the real one, a tick that came 100 ms late on a busy runner was a gap in the watch). The ticks are the test's own too (#875): the stalled
// preparation steps the clock 10 s, then sends one tick, which is taken only when the evaluation before it has finished. So with the ticker the evaluations run one after the
// other, each reads the clock as it is when it starts, none is late enough to read a time before the one recorded, and no two are more than one step apart; without it nothing
// evaluates the gate while the clock moves.
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
			oldObs, oldEvery, oldTicks, oldPrepared := observeQuietFor, watchEvery, watchTicks, prepared
			observeQuietFor = menuQuietAfterWrite
			t.Cleanup(func() {
				observeQuietFor, watchEvery, watchTicks, prepared = oldObs, oldEvery, oldTicks, oldPrepared
				forgetObserved(p)
				forgetOwnWrite(p)
			})
			forgetObserved(p)
			// The ticks are the test's own (#875): an unbuffered channel, so a send is taken only when the keepWatching goroutine is back at its select, that is, when the evaluation
			// before has finished. With a ticker that ran on the real time, an evaluation could read the clock, be held up before it recorded the observation (a busy runner), and
			// arrive after the write's own evaluation at the later time: a reading 10 s before the last one is a clock that went backward, the watch started over and the write was refused.
			ticks := make(chan time.Time)
			watchTicks = func(time.Duration) (<-chan time.Time, func()) { return ticks, func() {} }
			if !tc.ticker {
				watchEvery = time.Hour
				watchTicks = oldTicks // nothing is sent; the hour is never reached
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
			// The preparation takes 120 s of the test's clock, four gaps. With the ticker each step is followed by one tick, taken when the evaluation before it is done; without it
			// the clock moves at once.
			before := asked.Load()
			prepared = func() {
				for i := 0; i < 12; i++ {
					stepped.Add(int64(10 * time.Second))
					if tc.ticker {
						// The clock has moved; the tick is taken only after the evaluation before it has finished, and the evaluation it starts reads the clock as it now is, because the
						// clock moves again only after the next tick is taken. So the evaluations are one after the other, none of them reads a time before the one recorded, and no
						// two are more than one step apart. (The real time limit only keeps a goroutine that never comes from hanging the test.)
						select {
						case ticks <- time.Time{}:
						case <-time.After(30 * time.Second):
							t.Error("keepWatching did not take a tick while the write was prepared")
							return
						}
					}
				}
			}
			f := s.Flush()
			if tc.ticker {
				if !f.Saved {
					t.Fatalf("a slow preparation broke the watch, and the write was refused: %+v", f)
				}
				if n := asked.Load() - before; n < 12 {
					t.Fatalf("keepWatching evaluated the gate %d times for 12 ticks", n)
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

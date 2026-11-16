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
// The clock of the test runs 300 times faster than the real one (100 ms is the watch's gap), plus the steps the test makes by hand, so the evaluations of the background ticker
// see the time pass while the preparation is stalled.
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
			watchEvery = 10 * time.Millisecond // 3 s of the test's clock
			if !tc.ticker {
				watchEvery = time.Hour
			}
			base := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
			start := time.Now()
			var stepped atomic.Int64 // the steps the test makes by hand, in nanoseconds of the test's clock
			clock := func() time.Time {
				return base.Add(time.Duration(stepped.Load()) + 300*time.Since(start))
			}
			nowFunc = clock
			mod := base.Add(-48 * time.Hour) // one time for the whole test: a changing time would restart the watch itself
			fileTimeFor = func(string) (time.Time, bool, error) { return mod, false, nil }
			statusFor = func(string) (spStatus, error) {
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
			prepared = func() { time.Sleep(350 * time.Millisecond) } // 105 s of the test's clock, more than three gaps
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

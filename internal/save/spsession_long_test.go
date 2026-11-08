package save

import (
	"os"
	"strings"
	"testing"
)

// A long session makes a long log (about 70 MB an hour, #530). The status is read from what the game added since the last call, in chunks, whatever the size of the file.
// These tests use small chunk sizes and small "caps"; no large fixture is needed because nothing in the code depends on the size of the file any more.

// wholeParse is the reference: the tracker fed with every complete line of text at once, as a parse of the whole file.
func wholeParse(text string) spStatus {
	var tr spTracker
	if nl := strings.LastIndexByte(text, '\n'); nl >= 0 {
		for _, line := range strings.Split(text[:nl], "\n") {
			tr.feed(strings.TrimRight(line, "\r"))
		}
	}
	return spStatus{Active: tr.SPActive, EndedAt: tr.EndedAt, LastSaveAt: tr.LastSaveAt}
}

func setChunks(t *testing.T, chunk, maxLine int) {
	t.Helper()
	oc, ol := logChunk, maxLogLine
	t.Cleanup(func() { logChunk, maxLogLine = oc, ol })
	logChunk, maxLogLine = chunk, maxLine
}

// script is a game session in lines: worlds entered and left, writes of the database, noise.
func script(n int) []string {
	cycle := []string{lnBOMOpen, lnNoise, lnStart, lnSave0, lnNoise, lnSave1, lnLeave1, lnSave1, lnLeave2, lnNoise, lnMP, lnNoise}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, cycle[i%len(cycle)])
	}
	return out
}

func sizeOf(t *testing.T, p string) int64 {
	t.Helper()
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return st.Size()
}

// A log followed up to a small capBytes keeps answering when it grows past three times the capBytes, equal to a parse of the whole file at every step.
func TestLongLogIsFollowedPastAnyCap(t *testing.T) {
	const capBytes = 2000
	for _, chunk := range []int{1, 64, 4096} {
		setChunks(t, chunk, 1<<20)
		resetSPLog()
		save, lp := writeLog(t, "")
		var all strings.Builder
		crossedCap, crossedThree := false, false
		steps := 0
		for i, line := range script(100) {
			piece := line + "\r\n"
			if i%5 == 4 { // every fifth line arrives in two parts, so a read can end inside a line
				cut := len(piece) / 2
				appendLog(t, lp, piece[:cut])
				all.WriteString(piece[:cut])
				st, err := singlePlayerStatus(save)
				if err != nil || st != wholeParse(all.String()) {
					t.Fatalf("chunk %d, half line %d: %+v %v, whole parse %+v", chunk, i, st, err, wholeParse(all.String()))
				}
				piece = piece[cut:]
			}
			appendLog(t, lp, piece)
			all.WriteString(piece)
			st, err := singlePlayerStatus(save)
			if err != nil {
				t.Fatalf("chunk %d, line %d: a long log must not be an error: %v", chunk, i, err)
			}
			if want := wholeParse(all.String()); st != want {
				t.Fatalf("chunk %d, line %d (%d bytes): followed %+v, whole parse %+v", chunk, i, all.Len(), st, want)
			}
			steps++
			if size := sizeOf(t, lp); size > capBytes {
				crossedCap = true
				if size > 3*capBytes {
					crossedThree = true
				}
			}
			spLog.Lock()
			off := spLog.offset
			spLog.Unlock()
			if off != int64(all.Len()) {
				t.Fatalf("chunk %d: the offset %d must be at the end of the file (%d)", chunk, off, all.Len())
			}
		}
		if !crossedCap || !crossedThree || steps != 100 {
			t.Fatalf("test setup: the log must pass the capBytes and three times the capBytes (%v %v %d)", crossedCap, crossedThree, steps)
		}
	}
}

// A log that is already long on the first read is read to the end in chunks, also with a chunk smaller than a line, and a last line without its end waits until it is complete.
func TestLongLogFirstReadInChunks(t *testing.T) {
	line := len(lnStart) + 2
	for _, chunk := range []int{1, 7, line - 1, line, line + 1, 1000, 1 << 20} {
		setChunks(t, chunk, 1<<20)
		resetSPLog()
		body := strings.Join(script(600), "\r\n") + "\r\n"
		// the last line is cut: it is the game's start marker, which must not count until it is complete
		tail := lnStart
		full := body + tail
		save, lp := writeLog(t, full[:len(full)-9])
		if int64(len(full)) <= 3*int64(chunk) && chunk < 1000 {
			t.Fatalf("test setup: chunk %d, file %d", chunk, len(full))
		}
		before := wholeParse(full[:len(full)-9])
		st, err := singlePlayerStatus(save)
		if err != nil || st != before {
			t.Fatalf("chunk %d: first read %+v %v, whole parse %+v", chunk, st, err, before)
		}
		spLog.Lock()
		rest := spLog.rest
		spLog.Unlock()
		if rest != tail[:len(tail)-9] {
			t.Fatalf("chunk %d: the unfinished line must be kept, got %q", chunk, rest)
		}
		appendLog(t, lp, tail[len(tail)-9:]+"\r\n")
		want := wholeParse(full + "\r\n")
		if st, err = singlePlayerStatus(save); err != nil || st != want || !st.Active {
			t.Fatalf("chunk %d: the completed line: %+v %v, whole parse %+v", chunk, st, err, want)
		}
		// nothing new: nothing applied twice
		if again, _ := singlePlayerStatus(save); again != st {
			t.Fatalf("chunk %d: a read with nothing new changed the state", chunk)
		}
	}
}

// A file that was replaced (a new game run, a shorter log, or a longer one with another start) is read again from the top, in chunks.
func TestLongLogRestartsWhenReplaced(t *testing.T) {
	setChunks(t, 50, 1<<20)
	resetSPLog()
	first := "\xef\xbb\xbfLog: Log file open, 10/08/26 09:20:16\r\n" + strings.Join(script(200), "\r\n") + "\r\n"
	save, lp := writeLog(t, first)
	if st, err := singlePlayerStatus(save); err != nil || st != wholeParse(first) {
		t.Fatalf("%+v %v", st, err)
	}
	// shorter
	shorter := "\xef\xbb\xbfLog: Log file open, 10/08/26 11:00:00\r\n" + lnStart + "\r\n"
	if err := os.WriteFile(lp, []byte(shorter), 0o644); err != nil {
		t.Fatal(err)
	}
	if st, err := singlePlayerStatus(save); err != nil || st != wholeParse(shorter) || !st.Active {
		t.Fatalf("a shorter file starts from the top: %+v %v", st, err)
	}
	// longer than the old one, another run: told apart by its first bytes
	longer := "\xef\xbb\xbfLog: Log file open, 10/08/26 12:00:00\r\n" + lnStart + "\r\n" + lnLeave1 + "\r\n" + strings.Join(script(300), "\r\n") + "\r\n"
	if int64(len(longer)) <= sizeOf(t, lp) {
		t.Fatal("test setup: the replacement must be longer")
	}
	if err := os.WriteFile(lp, []byte(longer), 0o644); err != nil {
		t.Fatal(err)
	}
	if st, err := singlePlayerStatus(save); err != nil || st != wholeParse(longer) {
		t.Fatalf("a replaced, longer file starts from the top: %+v %v, whole parse %+v", st, err, wholeParse(longer))
	}
	// an unfinished line of the old file does not leak into the new one
	if err := os.WriteFile(lp, []byte(first+lnStart[:40]), 0o644); err != nil {
		t.Fatal(err)
	}
	singlePlayerStatus(save)
	if err := os.WriteFile(lp, []byte("\xef\xbb\xbfLog: Log file open, 10/08/26 13:00:00\r\n"+lnNoise+"\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if st, err := singlePlayerStatus(save); err != nil || st.Active || !st.EndedAt.IsZero() {
		t.Fatalf("a new file starts clean: %+v %v", st, err)
	}
}

// A line far longer than maxLogLine is not held whole: its head is read as the line, the rest is dropped to its end, and the lines after it are read as usual.
func TestVeryLongLineIsBounded(t *testing.T) {
	setChunks(t, 16, 200)
	resetSPLog()
	huge := lnNoise + strings.Repeat("x", 5000)
	save, lp := writeLog(t, lnBOMOpen+"\r\n"+huge[:2500])
	for i := 0; i < 2; i++ {
		st, err := singlePlayerStatus(save)
		if err != nil || st.Active {
			t.Fatalf("%+v %v", st, err)
		}
		spLog.Lock()
		n := len(spLog.rest)
		spLog.Unlock()
		if n > 200 {
			t.Fatalf("a line without an end is kept up to maxLogLine, not %d bytes", n)
		}
		appendLog(t, lp, huge[2500:])
	}
	appendLog(t, lp, "\r\n"+lnStart+"\r\n")
	if st, err := singlePlayerStatus(save); err != nil || !st.Active {
		t.Fatalf("the line after the long one must be read: %+v %v", st, err)
	}
}

// A log that cannot be read at all still fails closed: an error, and the edit is queued with a reason that says so (the other branches of the rule are in spsession_test.go).
func TestUnreadableLogQueuesTheEdit(t *testing.T) {
	oldList, oldStatus, oldHeld := listProcs, statusFor, fileHeld
	t.Cleanup(func() { listProcs, statusFor, fileHeld = oldList, oldStatus, oldHeld })
	listProcs = func(bool) ([]string, string) { return []string{"DuneSandbox-Win64-Shipping.exe"}, "" }
	statusFor = singlePlayerStatus
	fileHeld = func(string) (bool, error) { return false, nil }
	setChunks(t, 64, 1<<20)
	resetSPLog()
	save, lp := writeLog(t, strings.Join(script(300), "\r\n")+"\r\n")
	if _, err := singlePlayerStatus(save); err != nil {
		t.Fatalf("a long log is not an error: %v", err)
	}
	if err := os.Remove(lp); err != nil {
		t.Fatal(err)
	}
	_, err := singlePlayerStatus(save)
	if err == nil || !strings.Contains(err.Error(), "cannot read the game log") {
		t.Fatalf("a removed log must be an error: %v", err)
	}
	g := processState(save, true)
	if !g.Blocked || g.Mode != "game_running" || !strings.Contains(g.Reason, "its log cannot be read") || !strings.HasSuffix(g.Reason, QueuedUntilClears) {
		t.Fatalf("the edit must be queued with the reason: %+v", g)
	}
	if strings.Contains(g.Reason, "256") || strings.Contains(g.Reason, "larger") {
		t.Fatalf("no size in the reason: %q", g.Reason)
	}
}

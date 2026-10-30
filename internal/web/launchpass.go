package web

import (
	"math"
	"sync"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
)

// The automatic pass at launch used to start the moment the editor opened, before the page was there to turn the switch off (#460). It now waits: the clock starts when the
// first page asks about it (so a person has the banner on screen for the whole wait), and the pass runs only if nobody pressed Skip, turned the setting off, or started the
// editor with -no-auto. A page that never asks, such as a script that only calls the API, never starts the pass.

// LaunchDelay is how long the banner counts down before the pass at launch starts.
const LaunchDelay = 15 * time.Second

type launchPhase int

const (
	launchNone      launchPhase = iota // no pass is wanted at launch, or none is scheduled
	launchScheduled                    // wanted; waits for the first page to show the countdown
	launchCounting                     // the countdown is on screen; the timer is running
	launchRan                          // the pass was started (it may still be working, or have been held back by the game)
	launchSkipped                      // Skip, the setting turned off, or -no-auto: no pass runs
)

type launchPass struct {
	mu       sync.Mutex
	phase    launchPhase
	delay    time.Duration
	deadline time.Time
	timer    *time.Timer
	busy     bool // the pass that the countdown started is working (it can take seconds on a large save)
}

// ScheduleLaunchPass decides what happens at launch. With nothing switched on there is nothing to wait for. With skip (the -no-auto flag) the pass does not run in this run and
// the page says so. With a delay of zero or less the pass runs at once (for tests and callers that have their own wait); otherwise it waits for the countdown.
func (s *Server) ScheduleLaunchPass(delay time.Duration, skip bool) {
	if !autoPassWanted(settings.Load(s.SettingsPath)) {
		return
	}
	if !save.IsGamesOwnSave(s.Save.Path) { // a copy is never refilled by itself (#508): no countdown, and the page says why
		s.launch.mu.Lock()
		s.launch.phase = launchSkipped
		s.launch.mu.Unlock()
		s.NoteStartup(passNotRunOnCopy, false)
		return
	}
	if skip {
		s.launch.mu.Lock()
		s.launch.phase = launchSkipped
		s.launch.mu.Unlock()
		s.NoteStartup("Automatic refill skipped for this run: the editor was started with -no-auto.", false)
		return
	}
	s.launch.mu.Lock()
	if delay <= 0 {
		s.launch.phase = launchRan
		s.launch.mu.Unlock()
		s.RunStartupTasks()
		return
	}
	s.launch.phase, s.launch.delay = launchScheduled, delay
	s.launch.mu.Unlock()
}

// launchStatus is what the page's banner shows: whether a pass is counting down and for how many more seconds. The first call after the launch starts the clock. A countdown whose
// switches were turned off meanwhile ends here, so the banner disappears at once and the pass does not run.
func (s *Server) launchStatus() map[string]any {
	wanted := autoPassWanted(settings.Load(s.SettingsPath)) // read before the lock: it reads a file
	l := &s.launch
	l.mu.Lock()
	defer l.mu.Unlock()
	switch l.phase {
	case launchScheduled:
		if !wanted {
			l.phase = launchSkipped
			break
		}
		l.phase = launchCounting
		l.deadline = s.now().Add(l.delay)
		l.timer = time.AfterFunc(l.delay, s.fireLaunch)
	case launchCounting:
		if !wanted {
			l.stopLocked()
			l.phase = launchSkipped
		}
	}
	return l.statusLocked(s.now())
}

func (l *launchPass) statusLocked(now time.Time) map[string]any {
	out := map[string]any{"pending": l.phase == launchCounting, "running": l.busy, "secondsLeft": 0, "seconds": int(math.Ceil(l.delay.Seconds()))}
	if l.phase == launchCounting {
		out["secondsLeft"] = max(0, int(math.Ceil(l.deadline.Sub(now).Seconds())))
	}
	return out
}

func (l *launchPass) stopLocked() {
	if l.timer != nil {
		l.timer.Stop()
		l.timer = nil
	}
}

// fireLaunch is the timer's end: the pass starts unless Skip or a switch got there first. RunStartupTasks reads the switches again, so a setting changed in the last moment is honoured too.
func (s *Server) fireLaunch() {
	l := &s.launch
	l.mu.Lock()
	if l.phase != launchCounting {
		l.mu.Unlock()
		return
	}
	l.phase = launchRan
	l.timer = nil
	l.busy = true
	l.mu.Unlock()
	defer func() {
		l.mu.Lock()
		l.busy = false
		l.mu.Unlock()
	}()
	s.RunStartupTasks()
}

// SkipLaunchPass is the Skip button: no pass runs at launch. If the pass has already started, that is said and nothing is undone.
func (s *Server) SkipLaunchPass() map[string]any {
	l := &s.launch
	l.mu.Lock()
	defer l.mu.Unlock()
	skipped := l.phase == launchScheduled || l.phase == launchCounting
	if skipped {
		l.stopLocked()
		l.phase = launchSkipped
	}
	out := l.statusLocked(s.now())
	out["skipped"] = skipped
	out["started"] = l.phase == launchRan
	return out
}

// RunPassNow is the "Run the automatic pass now" button: it ends any countdown (the pass must not run twice) and runs the jobs that are switched on, whether or not the master
// switch for the launch is on. The write rules are those of an edit made by hand (#499): nothing is written while the game may still hold the save's rows, and unlike the pass at launch it is not queued for later.
func (s *Server) RunPassNow() {
	l := &s.launch
	l.mu.Lock()
	if l.phase == launchScheduled || l.phase == launchCounting {
		l.stopLocked()
		l.phase = launchSkipped
	}
	l.mu.Unlock()
	s.runPass(true)
}

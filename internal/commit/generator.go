// Package commit handles commit count generation and backdated git commit execution.
package commit

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"commitforge/internal/contribution"
)

// ── Commit-job types ─────────────────────────────────────────────────────────

// CommitJob describes a single backdated commit that should be created.
type CommitJob struct {
	Timestamp time.Time // exact UTC timestamp for this commit
	Message   string    // commit message
}

// defaultMessages is the random pool used when no fixed message is configured.
var defaultMessages = []string{
	"update",
	"fix",
	"refactor",
	"chore: maintenance",
	"docs: update",
	"style: cleanup",
	"perf: improve performance",
	"test: add tests",
	"build: update dependencies",
	"ci: update pipeline",
	"feat: minor improvements",
	"fix: small bug fix",
}

const (
	dayStartSec  = 9 * 60 * 60  // 09:00 UTC in seconds from midnight
	dayEndSec    = 17 * 60 * 60 // 17:00 UTC in seconds from midnight
	dayWindowSec = dayEndSec - dayStartSec
)

// StaggerJobs produces count CommitJobs for the given calendar date.
// Timestamps are evenly distributed across the 09:00–17:00 UTC window so that
// multiple same-day commits have distinct seconds.  A single commit is placed
// at a random offset within the window.
// Pass nil messages to use the built-in default pool.
// Pass nil rng to use a new time-seeded source.
func StaggerJobs(date time.Time, count int, messages []string, rng *rand.Rand) []CommitJob {
	if count <= 0 {
		return nil
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	if len(messages) == 0 {
		messages = defaultMessages
	}

	base := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	jobs := make([]CommitJob, count)

	for i := 0; i < count; i++ {
		var secOffset int
		if count == 1 {
			// Single commit: random position in the window.
			secOffset = dayStartSec + rng.Intn(dayWindowSec)
		} else {
			// Multiple commits: even spacing, step = window / count (≥ 1 s).
			step := dayWindowSec / count
			if step < 1 {
				step = 1
			}
			secOffset = dayStartSec + i*step
			// Clamp in case of overflow beyond 17:00.
			if secOffset >= dayEndSec {
				secOffset = dayEndSec - (count - i)
			}
		}
		jobs[i] = CommitJob{
			Timestamp: base.Add(time.Duration(secOffset) * time.Second),
			Message:   messages[rng.Intn(len(messages))],
		}
	}
	return jobs
}

// CountSpec describes how commit counts are assigned to selected days.
// Exactly one mode is active:
//   - Fixed > 0: every day receives exactly Fixed commits.
//   - Fixed == 0: every day receives an independently sampled value in [Min, Max].
type CountSpec struct {
	Fixed int // > 0 for fixed mode
	Min   int // inclusive minimum for random mode
	Max   int // inclusive maximum for random mode
}

// IsRandom reports whether the spec uses per-day random sampling.
func (cs CountSpec) IsRandom() bool {
	return cs.Fixed == 0
}

// SampleCount returns the commit count for one day.
// For fixed mode it returns cs.Fixed.
// For random mode it draws uniformly from [cs.Min, cs.Max] using rng.
func (cs CountSpec) SampleCount(rng *rand.Rand) int {
	if !cs.IsRandom() {
		return cs.Fixed
	}
	return cs.Min + rng.Intn(cs.Max-cs.Min+1)
}

// ParseCountSpec parses a user-supplied string into a CountSpec.
//
// Accepted formats:
//
//	"5"    → fixed, every day gets 5 commits
//	"1-8"  → random, each day gets an independent draw from [1, 8]
//
// The count (or both bounds) must be ≥ 1. For ranges, min must be ≤ max.
func ParseCountSpec(s string) (CountSpec, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return CountSpec{}, fmt.Errorf("enter a count (e.g. 5) or range (e.g. 1-8)")
	}

	// Detect "min-max" format: hyphen must not be the first character.
	if idx := strings.Index(s, "-"); idx > 0 {
		minStr := s[:idx]
		maxStr := s[idx+1:]

		minVal, err := strconv.Atoi(minStr)
		if err != nil || minVal < 1 {
			return CountSpec{}, fmt.Errorf("min must be a positive integer")
		}
		maxVal, err := strconv.Atoi(maxStr)
		if err != nil || maxVal < 1 {
			return CountSpec{}, fmt.Errorf("max must be a positive integer")
		}
		if minVal > maxVal {
			return CountSpec{}, fmt.Errorf("min (%d) must be ≤ max (%d)", minVal, maxVal)
		}
		return CountSpec{Min: minVal, Max: maxVal}, nil
	}

	// Fixed count.
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return CountSpec{}, fmt.Errorf("count must be a positive integer")
	}
	return CountSpec{Fixed: n}, nil
}

// ApplyToCalendar writes the commit count computed by spec into the Day.Count field
// of every date listed in dates within cal.
// rng is used for random-mode specs; pass nil to use a new time-seeded source.
// Dates that are not present in cal (padding or out-of-range) are silently skipped.
func ApplyToCalendar(spec CountSpec, dates []time.Time, cal *contribution.Calendar, rng *rand.Rand) {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	// Build a fast date→grid-position index so Apply is O(n) not O(n×weeks×7).
	type pos struct{ w, d int }
	index := make(map[string]pos, len(cal.Weeks)*7)
	for wi, week := range cal.Weeks {
		for wd := range week {
			if !week[wd].Date.IsZero() {
				key := week[wd].Date.UTC().Format("2006-01-02")
				index[key] = pos{wi, wd}
			}
		}
	}

	for _, d := range dates {
		key := d.UTC().Format("2006-01-02")
		if p, ok := index[key]; ok {
			cal.Weeks[p.w][p.d].Count = spec.SampleCount(rng)
		}
	}
}

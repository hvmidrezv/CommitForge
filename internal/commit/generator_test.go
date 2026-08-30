package commit

import (
	"math/rand"
	"testing"
	"time"

	"commitforge/internal/contribution"
)

// ---- helpers ----------------------------------------------------------------

func testDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// ---- ParseCountSpec tests ---------------------------------------------------

func TestParseCountSpec_Fixed(t *testing.T) {
	spec, err := ParseCountSpec("5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Fixed != 5 || spec.IsRandom() {
		t.Errorf("got %+v, want Fixed=5 random=false", spec)
	}
}

func TestParseCountSpec_Random(t *testing.T) {
	spec, err := ParseCountSpec("1-8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spec.IsRandom() || spec.Min != 1 || spec.Max != 8 {
		t.Errorf("got %+v, want random Min=1 Max=8", spec)
	}
}

func TestParseCountSpec_RangeSameMinMax(t *testing.T) {
	spec, err := ParseCountSpec("3-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spec.IsRandom() || spec.Min != 3 || spec.Max != 3 {
		t.Errorf("got %+v, want Min=3 Max=3", spec)
	}
}

func TestParseCountSpec_WhitespaceStripped(t *testing.T) {
	_, err := ParseCountSpec("  7  ")
	if err != nil {
		t.Errorf("leading/trailing spaces should be accepted, got error: %v", err)
	}
}

func TestParseCountSpec_Invalid(t *testing.T) {
	cases := []struct {
		input string
		desc  string
	}{
		{"", "empty"},
		{"0", "zero count"},
		{"-1", "negative fixed"},
		{"abc", "non-numeric"},
		{"-", "just hyphen"},
		{"-8", "leading hyphen (no min)"},
		{"1-", "no max"},
		{"5-3", "min > max"},
		{"0-5", "zero min"},
		{"3-0", "zero max"},
		{"1.5", "decimal"},
	}
	for _, tc := range cases {
		_, err := ParseCountSpec(tc.input)
		if err == nil {
			t.Errorf("ParseCountSpec(%q) [%s] expected error, got nil", tc.input, tc.desc)
		}
	}
}

// ---- SampleCount tests ------------------------------------------------------

func TestSampleCount_Fixed_AlwaysSame(t *testing.T) {
	spec := CountSpec{Fixed: 7}
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		if got := spec.SampleCount(rng); got != 7 {
			t.Errorf("iteration %d: got %d, want 7", i, got)
		}
	}
}

func TestSampleCount_Random_InBounds(t *testing.T) {
	spec := CountSpec{Min: 3, Max: 9}
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 1000; i++ {
		v := spec.SampleCount(rng)
		if v < 3 || v > 9 {
			t.Errorf("iteration %d: got %d, want in [3, 9]", i, v)
		}
	}
}

func TestSampleCount_Random_SingleValueRange(t *testing.T) {
	spec := CountSpec{Min: 5, Max: 5}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20; i++ {
		if v := spec.SampleCount(rng); v != 5 {
			t.Errorf("iteration %d: got %d, want 5", i, v)
		}
	}
}

// ---- ApplyToCalendar tests --------------------------------------------------

func TestApplyToCalendar_Fixed_AllDatesSet(t *testing.T) {
	cal := contribution.Build(1, testDate(2024, 6, 15))
	var sel contribution.Selection
	sel.SelectRange(testDate(2024, 6, 3), testDate(2024, 6, 7), cal)

	spec := CountSpec{Fixed: 5}
	ApplyToCalendar(spec, sel.Dates(), &cal, nil)

	for _, d := range sel.Dates() {
		found := false
		for _, week := range cal.Weeks {
			for _, day := range week {
				if day.Date.Equal(d) {
					found = true
					if day.Count != 5 {
						t.Errorf("date %v: Count = %d, want 5", d, day.Count)
					}
				}
			}
		}
		if !found {
			t.Errorf("date %v not found in calendar", d)
		}
	}
}

func TestApplyToCalendar_Random_CountsInRange(t *testing.T) {
	cal := contribution.Build(1, testDate(2024, 6, 15))
	var sel contribution.Selection
	sel.SelectRange(testDate(2024, 6, 3), testDate(2024, 6, 7), cal)

	spec := CountSpec{Min: 2, Max: 6}
	rng := rand.New(rand.NewSource(42))
	ApplyToCalendar(spec, sel.Dates(), &cal, rng)

	for _, d := range sel.Dates() {
		for _, week := range cal.Weeks {
			for _, day := range week {
				if day.Date.Equal(d) {
					if day.Count < 2 || day.Count > 6 {
						t.Errorf("date %v: Count = %d, want in [2, 6]", d, day.Count)
					}
				}
			}
		}
	}
}

func TestApplyToCalendar_OnlyAffectsSelectedDates(t *testing.T) {
	cal := contribution.Build(1, testDate(2024, 6, 15))
	var sel contribution.Selection
	// Select only June 10.
	sel.Add(testDate(2024, 6, 10))

	spec := CountSpec{Fixed: 3}
	ApplyToCalendar(spec, sel.Dates(), &cal, nil)

	// All other days must remain at count 0.
	for _, week := range cal.Weeks {
		for _, day := range week {
			if day.Date.IsZero() || day.Date.Equal(testDate(2024, 6, 10)) {
				continue
			}
			if day.Count != 0 {
				t.Errorf("unselected date %v has Count = %d, want 0", day.Date, day.Count)
			}
		}
	}
}

func TestApplyToCalendar_Random_PerDayIndependence(t *testing.T) {
	// Select 10 days; with range [1,100] and any decent RNG, multiple distinct
	// values must appear — confirming each day is sampled independently.
	cal := contribution.Build(1, testDate(2024, 6, 15))
	var sel contribution.Selection
	sel.SelectRange(testDate(2024, 6, 1), testDate(2024, 6, 10), cal)

	spec := CountSpec{Min: 1, Max: 100}
	rng := rand.New(rand.NewSource(99)) // seed chosen to yield variety
	ApplyToCalendar(spec, sel.Dates(), &cal, rng)

	seen := make(map[int]bool)
	for _, d := range sel.Dates() {
		for _, week := range cal.Weeks {
			for _, day := range week {
				if day.Date.Equal(d) {
					seen[day.Count] = true
				}
			}
		}
	}
	if len(seen) < 2 {
		t.Errorf("expected multiple distinct counts (per-day independence), got only: %v", seen)
	}
}

// ── StaggerJobs tests ─────────────────────────────────────────────────────────

func TestStaggerJobs_ZeroCount_ReturnsNil(t *testing.T) {
	jobs := StaggerJobs(testDate(2024, 6, 15), 0, nil, nil)
	if len(jobs) != 0 {
		t.Errorf("got %d jobs for count=0, want 0", len(jobs))
	}
}

func TestStaggerJobs_SingleCommit_InWindow(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	jobs := StaggerJobs(testDate(2024, 6, 15), 1, nil, rng)
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(jobs))
	}
	h := jobs[0].Timestamp.Hour()
	if h < 9 || h >= 17 {
		t.Errorf("timestamp hour = %d, want in [9, 16]", h)
	}
}

func TestStaggerJobs_MultipleCommits_CountMatches(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	jobs := StaggerJobs(testDate(2024, 6, 15), 5, nil, rng)
	if len(jobs) != 5 {
		t.Fatalf("got %d jobs, want 5", len(jobs))
	}
}

func TestStaggerJobs_MultipleCommits_AllInWindow(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	jobs := StaggerJobs(testDate(2024, 6, 15), 5, nil, rng)
	for i, j := range jobs {
		h := j.Timestamp.Hour()
		if h < 9 || h >= 17 {
			t.Errorf("job[%d] hour = %d, want in [9, 16]", i, h)
		}
	}
}

func TestStaggerJobs_MultipleCommits_DistinctTimestamps(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	jobs := StaggerJobs(testDate(2024, 6, 15), 8, nil, rng)
	seen := map[time.Time]bool{}
	for _, j := range jobs {
		if seen[j.Timestamp] {
			t.Errorf("duplicate timestamp: %v", j.Timestamp)
		}
		seen[j.Timestamp] = true
	}
}

func TestStaggerJobs_CorrectDate(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	jobs := StaggerJobs(testDate(2024, 6, 15), 3, nil, rng)
	for _, j := range jobs {
		if j.Timestamp.Year() != 2024 || j.Timestamp.Month() != 6 || j.Timestamp.Day() != 15 {
			t.Errorf("job date = %v, want 2024-06-15", j.Timestamp)
		}
	}
}

func TestStaggerJobs_CustomMessages(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	msgs := []string{"my commit"}
	jobs := StaggerJobs(testDate(2024, 6, 15), 4, msgs, rng)
	for _, j := range jobs {
		if j.Message != "my commit" {
			t.Errorf("message = %q, want %q", j.Message, "my commit")
		}
	}
}

func TestStaggerJobs_DefaultMessagesUsedWhenNil(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	jobs := StaggerJobs(testDate(2024, 6, 15), 20, nil, rng)
	for _, j := range jobs {
		if j.Message == "" {
			t.Error("message should not be empty when using default pool")
		}
	}
}

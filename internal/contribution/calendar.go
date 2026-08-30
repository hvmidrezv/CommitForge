// Package contribution provides calendar generation and day-selection logic.
package contribution

import "time"

// Level is the contribution intensity rendered in the GitHub-style grid (0–4).
type Level int

const (
	// LevelNone renders the "no contributions" color.
	LevelNone Level = 0 // no commits
	// LevelLight renders the first non-zero intensity color.
	LevelLight Level = 1 // 1–3 commits
	// LevelMedium renders the medium intensity color.
	LevelMedium Level = 2 // 4–6 commits
	// LevelStrong renders the strong intensity color.
	LevelStrong Level = 3 // 7–9 commits
	// LevelFull renders the highest intensity color.
	LevelFull Level = 4 // 10+ commits
)

// CountToLevel maps a non-negative commit count to a display Level.
func CountToLevel(count int) Level {
	switch {
	case count == 0:
		return LevelNone
	case count <= 3:
		return LevelLight
	case count <= 6:
		return LevelMedium
	case count <= 9:
		return LevelStrong
	default:
		return LevelFull
	}
}

// Day is a single cell in the contribution grid.
// A Day whose Date is the zero value is a padding cell (future or out-of-range).
type Day struct {
	Date    time.Time // calendar date, UTC midnight; zero = padding
	Count   int       // number of commits for this day (staged in memory)
	WeekIdx int       // column index in the grid (0 = oldest week)
	Weekday int       // row index: 0 = Sunday … 6 = Saturday
}

// Calendar holds the full week/day grid for N years back from a reference date.
// Weeks[c][r] is the Day for column c (0 = oldest), row r (0 = Sunday … 6 = Saturday).
// Cells where Date.IsZero() == true are padding (future days in the last partial week).
type Calendar struct {
	Weeks     [][]Day   // [weekIdx][weekday 0..6]
	StartDate time.Time // Sunday of the first (leftmost) column
	EndDate   time.Time // today (last rendered day)
	Years     int
}

// Build constructs a Calendar for the given number of years back from today.
// Pass time.Now() for today in production; pass a fixed date in tests for determinism.
func Build(years int, today time.Time) Calendar {
	if years < 1 {
		years = 1
	}
	today = truncateToDay(today)

	// Sunday of today's week.
	todaySunday := today.AddDate(0, 0, -int(today.Weekday()))
	// Go back 52*years weeks to set the leftmost column's Sunday.
	startDate := todaySunday.AddDate(0, 0, -52*years*7)

	// Build day-by-day to guarantee exactly one tile per day in [startDate, today]
	// without month-boundary or partial-week gaps.
	totalDays := int(today.Sub(startDate).Hours()/24) + 1
	weekCount := ((totalDays - 1) / 7) + 1
	weeks := make([][]Day, weekCount)
	for i := range weeks {
		weeks[i] = make([]Day, 7)
	}
	for d := startDate; !d.After(today); d = d.AddDate(0, 0, 1) {
		deltaDays := int(d.Sub(startDate).Hours() / 24)
		weekIdx := deltaDays / 7
		weekday := int(d.Weekday())
		weeks[weekIdx][weekday] = Day{
			Date:    d,
			Count:   0,
			WeekIdx: weekIdx,
			Weekday: weekday,
		}
	}

	return Calendar{
		Weeks:     weeks,
		StartDate: startDate,
		EndDate:   today,
		Years:     years,
	}
}

// truncateToDay returns t at UTC midnight, stripping the time-of-day component.
func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

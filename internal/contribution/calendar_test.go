package contribution

import (
	"testing"
	"time"
)

// testDate is a convenience constructor for UTC dates in tests.
func testDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func TestBuild_StartIsSunday(t *testing.T) {
	cal := Build(1, testDate(2024, 6, 15)) // Saturday
	if cal.StartDate.Weekday() != time.Sunday {
		t.Errorf("StartDate weekday = %v, want Sunday", cal.StartDate.Weekday())
	}
}

func TestBuild_EndIsToday(t *testing.T) {
	today := testDate(2024, 6, 15)
	cal := Build(1, today)
	if !cal.EndDate.Equal(today) {
		t.Errorf("EndDate = %v, want %v", cal.EndDate, today)
	}
}

func TestBuild_WeekCount_OneYear(t *testing.T) {
	cal := Build(1, testDate(2024, 6, 15))
	// 52 full past weeks + 1 current partial week = 53 columns.
	if got := len(cal.Weeks); got != 53 {
		t.Errorf("len(Weeks) = %d, want 53 (52 full + 1 partial)", got)
	}
}

func TestBuild_WeekCount_TwoYears(t *testing.T) {
	cal := Build(2, testDate(2024, 6, 15))
	// 52*2 full past weeks + 1 current partial week = 105 columns.
	if got := len(cal.Weeks); got != 105 {
		t.Errorf("len(Weeks) = %d, want 105 (52*2 + 1 partial)", got)
	}
}

func TestBuild_WeekdayAlignment(t *testing.T) {
	cal := Build(1, testDate(2024, 6, 15))
	for wi, week := range cal.Weeks {
		for wd, day := range week {
			if day.Date.IsZero() {
				continue
			}
			if got := int(day.Date.Weekday()); got != wd {
				t.Errorf("Weeks[%d][%d].Date.Weekday() = %d, want %d", wi, wd, got, wd)
			}
		}
	}
}

func TestBuild_LeapYear_Feb29Appears(t *testing.T) {
	// 2024 is a leap year; a 1-year calendar ending 2024-12-31 must include Feb 29.
	cal := Build(1, testDate(2024, 12, 31))
	target := testDate(2024, 2, 29)
	for _, week := range cal.Weeks {
		for _, d := range week {
			if d.Date.Equal(target) {
				return // found — test passes
			}
		}
	}
	t.Errorf("Feb 29 2024 not found in calendar ending 2024-12-31")
}

func TestBuild_NoDayBeyondToday(t *testing.T) {
	today := testDate(2024, 6, 15)
	cal := Build(1, today)
	for wi, week := range cal.Weeks {
		for wd, day := range week {
			if day.Date.IsZero() {
				continue
			}
			if day.Date.After(today) {
				t.Errorf("Weeks[%d][%d] = %v is after today %v", wi, wd, day.Date, today)
			}
		}
	}
}

func TestBuild_LastWeekPadding(t *testing.T) {
	// today = Wednesday (weekday 3); indices 4, 5, 6 of the last week must be padding.
	today := testDate(2024, 6, 12) // Wednesday
	cal := Build(1, today)
	lastWeek := cal.Weeks[len(cal.Weeks)-1]
	todayWD := int(today.Weekday()) // 3
	for wd := todayWD + 1; wd < 7; wd++ {
		if !lastWeek[wd].Date.IsZero() {
			t.Errorf("lastWeek[%d] should be padding, got %v", wd, lastWeek[wd].Date)
		}
	}
}

func TestBuild_StartDateExact(t *testing.T) {
	today := testDate(2024, 6, 15) // Saturday
	cal := Build(1, today)
	todaySunday := today.AddDate(0, 0, -int(today.Weekday()))
	want := todaySunday.AddDate(0, 0, -52*7)
	if !cal.StartDate.Equal(want) {
		t.Errorf("StartDate = %v, want %v", cal.StartDate, want)
	}
}

func TestBuild_TodayIsSunday(t *testing.T) {
	// When today is a Sunday the last week has exactly one non-padding day.
	today := testDate(2024, 12, 29) // Sunday
	cal := Build(1, today)
	lastWeek := cal.Weeks[len(cal.Weeks)-1]
	if lastWeek[0].Date.IsZero() {
		t.Error("lastWeek[0] (Sunday = today) must not be padding")
	}
	for wd := 1; wd < 7; wd++ {
		if !lastWeek[wd].Date.IsZero() {
			t.Errorf("lastWeek[%d] should be padding when today is Sunday", wd)
		}
	}
}

func TestCountToLevel(t *testing.T) {
	cases := []struct {
		count int
		want  Level
	}{
		{0, LevelNone},
		{1, LevelLight},
		{3, LevelLight},
		{4, LevelMedium},
		{6, LevelMedium},
		{7, LevelStrong},
		{9, LevelStrong},
		{10, LevelFull},
		{100, LevelFull},
	}
	for _, tc := range cases {
		if got := CountToLevel(tc.count); got != tc.want {
			t.Errorf("CountToLevel(%d) = %v, want %v", tc.count, got, tc.want)
		}
	}
}

func TestCalendarType_StoresBuildMetadata(t *testing.T) {
	var cal Calendar = Build(2, testDate(2024, 6, 15))
	if cal.Years != 2 {
		t.Fatalf("Years = %d, want 2", cal.Years)
	}
	if len(cal.Weeks) == 0 {
		t.Fatal("Weeks should not be empty")
	}
}

func TestDayType_ZeroValueIsPadding(t *testing.T) {
	var d Day
	if !d.Date.IsZero() {
		t.Fatal("zero-value Day should be padding (zero date)")
	}
}

func TestBuild_DayCoverage_HasNoGapsOrDuplicates(t *testing.T) {
	today := testDate(2024, 12, 31)
	cal := Build(1, today)

	seen := map[string]int{}
	var dates []time.Time
	for _, week := range cal.Weeks {
		for _, day := range week {
			if day.Date.IsZero() {
				continue
			}
			key := day.Date.Format("2006-01-02")
			seen[key]++
			dates = append(dates, day.Date)
		}
	}

	expected := int(cal.EndDate.Sub(cal.StartDate).Hours()/24) + 1
	if len(dates) != expected {
		t.Fatalf("non-padding day count = %d, want %d", len(dates), expected)
	}
	for key, n := range seen {
		if n != 1 {
			t.Fatalf("date %s appears %d times, want exactly 1", key, n)
		}
	}

	for d := cal.StartDate; !d.After(cal.EndDate); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		if seen[key] != 1 {
			t.Fatalf("date %s missing or duplicated; count=%d", key, seen[key])
		}
	}
}

func TestBuild_JunToAugWindow_HasAllDaysExactlyOnce(t *testing.T) {
	// Ends in late August so Jun-Aug are definitely inside the rendered range.
	today := testDate(2024, 8, 31)
	cal := Build(1, today)

	seen := map[string]int{}
	for _, week := range cal.Weeks {
		for _, day := range week {
			if day.Date.IsZero() {
				continue
			}
			seen[day.Date.Format("2006-01-02")]++
		}
	}

	jun1 := testDate(2024, 6, 1)
	aug31 := testDate(2024, 8, 31)
	windowDays := int(aug31.Sub(jun1).Hours()/24) + 1
	found := 0
	for d := jun1; !d.After(aug31); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		if seen[key] != 1 {
			t.Fatalf("Jun-Aug date %s appears %d times, want exactly 1", key, seen[key])
		}
		found++
	}
	if found != windowDays {
		t.Fatalf("checked %d days in Jun-Aug window, want %d", found, windowDays)
	}
}

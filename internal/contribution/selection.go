// Package contribution provides calendar generation and day-selection logic.
package contribution

import (
	"sort"
	"time"
)

// Selection holds the set of currently selected dates for the contribution grid.
// The zero value is an empty, ready-to-use selection.
type Selection struct {
	selected map[string]struct{} // keyed by "2006-01-02" UTC
}

// Toggle adds date to the selection if absent, or removes it if present.
func (s *Selection) Toggle(date time.Time) {
	s.ensureInit()
	k := dateKey(date)
	if _, ok := s.selected[k]; ok {
		delete(s.selected, k)
	} else {
		s.selected[k] = struct{}{}
	}
}

// Add marks date as selected. No-op if already selected.
func (s *Selection) Add(date time.Time) {
	s.ensureInit()
	s.selected[dateKey(date)] = struct{}{}
}

// Remove deselects date. No-op if not selected.
func (s *Selection) Remove(date time.Time) {
	if s.selected == nil {
		return
	}
	delete(s.selected, dateKey(date))
}

// IsSelected reports whether date is currently selected.
func (s *Selection) IsSelected(date time.Time) bool {
	if s.selected == nil {
		return false
	}
	_, ok := s.selected[dateKey(date)]
	return ok
}

// Count returns the number of selected dates.
func (s *Selection) Count() int {
	return len(s.selected)
}

// Clear deselects all dates.
func (s *Selection) Clear() {
	s.selected = nil
}

// SelectRange adds all non-padding calendar days in the closed interval [from, to]
// to the selection. If from is after to, they are swapped automatically.
func (s *Selection) SelectRange(from, to time.Time, cal Calendar) {
	s.ensureInit()
	if from.After(to) {
		from, to = to, from
	}
	// Walk real calendar days and pick the closed date interval so range select
	// stays chronological (not a visual rectangle across week columns).
	for _, week := range cal.Weeks {
		for _, day := range week {
			if day.Date.IsZero() {
				continue
			}
			if !day.Date.Before(from) && !day.Date.After(to) {
				s.selected[dateKey(day.Date)] = struct{}{}
			}
		}
	}
}

// SelectAll adds every non-padding day in cal to the selection.
func (s *Selection) SelectAll(cal Calendar) {
	s.ensureInit()
	for _, week := range cal.Weeks {
		for _, day := range week {
			if !day.Date.IsZero() {
				s.selected[dateKey(day.Date)] = struct{}{}
			}
		}
	}
}

// Dates returns all selected dates sorted in ascending chronological order.
func (s *Selection) Dates() []time.Time {
	if len(s.selected) == 0 {
		return nil
	}
	dates := make([]time.Time, 0, len(s.selected))
	for k := range s.selected {
		t, err := time.ParseInLocation("2006-01-02", k, time.UTC)
		if err == nil {
			dates = append(dates, t)
		}
	}
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].Before(dates[j])
	})
	return dates
}

// Clone returns an independent deep copy of the selection.
func (s *Selection) Clone() Selection {
	var c Selection
	if s.selected == nil {
		return c
	}
	c.selected = make(map[string]struct{}, len(s.selected))
	for k := range s.selected {
		c.selected[k] = struct{}{}
	}
	return c
}

// dateKey returns the canonical UTC string key for a date ("2006-01-02").
func dateKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func (s *Selection) ensureInit() {
	if s.selected == nil {
		s.selected = make(map[string]struct{})
	}
}

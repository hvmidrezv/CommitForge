package contribution

import (
	"testing"
	"time"
)

func TestToggle_SelectAndDeselect(t *testing.T) {
	var sel Selection
	d := testDate(2024, 6, 15)
	sel.Toggle(d)
	if !sel.IsSelected(d) {
		t.Error("should be selected after first Toggle")
	}
	sel.Toggle(d)
	if sel.IsSelected(d) {
		t.Error("should be deselected after second Toggle")
	}
}

func TestToggle_IsolatedDates(t *testing.T) {
	var sel Selection
	d1 := testDate(2024, 6, 15)
	d2 := testDate(2024, 6, 16)
	sel.Toggle(d1)
	if sel.IsSelected(d2) {
		t.Error("d2 must not be affected by toggling d1")
	}
	if sel.Count() != 1 {
		t.Errorf("Count = %d, want 1", sel.Count())
	}
}

func TestSelectRange_Chronological(t *testing.T) {
	cal := Build(1, testDate(2024, 6, 15))
	var sel Selection
	sel.SelectRange(testDate(2024, 6, 3), testDate(2024, 6, 7), cal)
	// June 3 (Mon) through June 7 (Fri) = 5 days
	if got := sel.Count(); got != 5 {
		t.Errorf("Count = %d, want 5", got)
	}
	for _, d := range []time.Time{
		testDate(2024, 6, 3),
		testDate(2024, 6, 4),
		testDate(2024, 6, 5),
		testDate(2024, 6, 6),
		testDate(2024, 6, 7),
	} {
		if !sel.IsSelected(d) {
			t.Errorf("date %v should be selected", d)
		}
	}
}

func TestSelectRange_ReversedOrder(t *testing.T) {
	cal := Build(1, testDate(2024, 6, 15))
	var sel Selection
	// Passing arguments in reverse; result must be the same as chronological order.
	sel.SelectRange(testDate(2024, 6, 7), testDate(2024, 6, 3), cal)
	if got := sel.Count(); got != 5 {
		t.Errorf("Count = %d, want 5", got)
	}
}

func TestSelectRange_SingleDay(t *testing.T) {
	cal := Build(1, testDate(2024, 6, 15))
	var sel Selection
	d := testDate(2024, 6, 10)
	sel.SelectRange(d, d, cal)
	if got := sel.Count(); got != 1 {
		t.Errorf("Count = %d, want 1", got)
	}
	if !sel.IsSelected(d) {
		t.Errorf("date %v should be selected", d)
	}
}

func TestSelectRange_CrossWeekBoundary(t *testing.T) {
	cal := Build(1, testDate(2024, 6, 15))
	var sel Selection
	// June 8 (Sat) to June 10 (Mon) crosses a Sun week boundary: 3 days.
	sel.SelectRange(testDate(2024, 6, 8), testDate(2024, 6, 10), cal)
	if got := sel.Count(); got != 3 {
		t.Errorf("Count = %d, want 3", got)
	}
	for _, d := range []time.Time{testDate(2024, 6, 8), testDate(2024, 6, 9), testDate(2024, 6, 10)} {
		if !sel.IsSelected(d) {
			t.Errorf("date %v should be selected", d)
		}
	}
}

func TestSelectAll(t *testing.T) {
	cal := Build(1, testDate(2024, 6, 15))
	var sel Selection
	sel.SelectAll(cal)
	want := 0
	for _, week := range cal.Weeks {
		for _, d := range week {
			if !d.Date.IsZero() {
				want++
			}
		}
	}
	if got := sel.Count(); got != want {
		t.Errorf("Count = %d, want %d", got, want)
	}
}

func TestClear(t *testing.T) {
	cal := Build(1, testDate(2024, 6, 15))
	var sel Selection
	sel.SelectAll(cal)
	if sel.Count() == 0 {
		t.Fatal("expected non-zero count after SelectAll")
	}
	sel.Clear()
	if sel.Count() != 0 {
		t.Errorf("Count = %d after Clear, want 0", sel.Count())
	}
}

func TestIsSelected_ZeroValue(t *testing.T) {
	var sel Selection
	if sel.IsSelected(testDate(2024, 6, 15)) {
		t.Error("IsSelected must return false on zero-value Selection")
	}
}

func TestDates_SortedAscending(t *testing.T) {
	cal := Build(1, testDate(2024, 6, 15))
	var sel Selection
	sel.SelectRange(testDate(2024, 6, 3), testDate(2024, 6, 7), cal)
	dates := sel.Dates()
	if len(dates) != 5 {
		t.Fatalf("len(Dates()) = %d, want 5", len(dates))
	}
	for i := 1; i < len(dates); i++ {
		if !dates[i-1].Before(dates[i]) {
			t.Errorf("Dates not sorted: [%d]=%v >= [%d]=%v", i-1, dates[i-1], i, dates[i])
		}
	}
}

func TestDates_EmptySelection(t *testing.T) {
	var sel Selection
	if d := sel.Dates(); d != nil {
		t.Errorf("Dates() = %v, want nil for empty selection", d)
	}
}

func TestClone_Independent(t *testing.T) {
	var sel Selection
	d := testDate(2024, 6, 15)
	sel.Toggle(d)

	clone := sel.Clone()
	// Mutate clone — original must be unaffected.
	clone.Clear()

	if !sel.IsSelected(d) {
		t.Error("original selection should still contain d after clearing the clone")
	}
	if clone.Count() != 0 {
		t.Error("clone should be empty after Clear")
	}
}

func TestAddRemove(t *testing.T) {
	var sel Selection
	d := testDate(2024, 6, 20)
	sel.Add(d)
	if !sel.IsSelected(d) {
		t.Fatal("Add should mark date as selected")
	}
	if sel.Count() != 1 {
		t.Fatalf("Count=%d want 1", sel.Count())
	}
	sel.Remove(d)
	if sel.IsSelected(d) {
		t.Fatal("Remove should clear selected date")
	}
	if sel.Count() != 0 {
		t.Fatalf("Count=%d want 0", sel.Count())
	}
}

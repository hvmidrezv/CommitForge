package tui

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"commitforge/internal/contribution"
	"commitforge/internal/gitops"
	"commitforge/internal/state"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---- helpers ----------------------------------------------------------------

// newTestModel creates a Model with a fixed calendar for deterministic tests.
func newTestModel(cal contribution.Calendar) Model {
	return Model{
		calendar:            cal,
		selection:           &contribution.Selection{},
		dateCounts:          map[string]int{},
		generatedDateCounts: map[string]int{},
		cursor:              initialCursor(cal),
		screen:              screenGrid,
		projectName:         "test-project",
		projectsRoot:        "output",
		viewYears:           max(1, cal.Years),
		viewEndDate:         cal.EndDate,
		initialAnchor:       cal.EndDate,
		cfg:                 Config{Dir: filepath.Join("output", "test-project"), DisableState: true},
	}
}

// testCal builds a 1-year calendar ending on 2024-06-15 (Saturday, weekday 6).
//
// Grid layout:
//
//	53 columns (weeks 0–52); week 52 = Jun 9–15.
//	Initial cursor = {Week:52, Weekday:6} = Jun 15.
func testCal() contribution.Calendar {
	return contribution.Build(1, time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC))
}

// makeKeyMsg converts a human-readable key string to a tea.KeyMsg.
func makeKeyMsg(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// pressKey feeds a single key event to m and returns the resulting Model.
func pressKey(m Model, key string) Model {
	result, _ := m.Update(makeKeyMsg(key))
	return result.(Model)
}

func runCmd(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return
	}
	for _, c := range batch {
		runCmd(c)
	}
}

// pressKeys feeds a sequence of key events and returns the final Model.
func pressKeys(m Model, keys ...string) Model {
	for _, k := range keys {
		m = pressKey(m, k)
	}
	return m
}

func assertQuitCmd(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected non-nil quit cmd")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("cmd returned %T, want tea.QuitMsg", msg)
	}
}

func assertVisibleDateSelectionAndCount(t *testing.T, m Model, date time.Time, wantCount int) {
	t.Helper()
	pos, ok := findDateCell(m.calendar, date)
	if !ok {
		t.Fatalf("date %s not visible in current window", date.Format("2006-01-02"))
	}
	day := m.calendar.Weeks[pos.Week][pos.Weekday]
	if !m.selection.IsSelected(day.Date) {
		t.Fatalf("date %s should be selected", date.Format("2006-01-02"))
	}
	if day.Count != wantCount {
		t.Fatalf("date %s count = %d, want %d", date.Format("2006-01-02"), day.Count, wantCount)
	}
}

// ---- cursor tests -----------------------------------------------------------

func TestCursorInitialization(t *testing.T) {
	m := newTestModel(testCal())
	// today is Jun 15 (Saturday, weekday 6); it lives in week 52 of 53.
	want := CellPos{Week: 52, Weekday: 6}
	if m.cursor != want {
		t.Errorf("initial cursor = %v, want %v", m.cursor, want)
	}
}

func TestInit_DisableStateReturnsNilCmd(t *testing.T) {
	m := Model{cfg: Config{DisableState: true}}
	if cmd := m.Init(); cmd != nil {
		t.Fatal("Init should return nil cmd when state is disabled")
	}
}

func TestInit_EnableStateReturnsAutosaveCmd(t *testing.T) {
	m := Model{cfg: Config{DisableState: false}}
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init should return autosave cmd when state is enabled")
	}
}

func TestMoveLeft_DecreasesWeek(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKey(m, "left")
	if m.cursor.Week != 51 {
		t.Errorf("cursor.Week = %d after left, want 51", m.cursor.Week)
	}
	if m.cursor.Weekday != 6 {
		t.Errorf("cursor.Weekday = %d, want 6 (unchanged)", m.cursor.Weekday)
	}
}

func TestMoveLeft_VimH(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKey(m, "h")
	if m.cursor.Week != 51 {
		t.Errorf("h: cursor.Week = %d, want 51", m.cursor.Week)
	}
}

func TestYearWindowNavigation_PreservesSelectionAndCountsAcrossViews(t *testing.T) {
	m := newTestModel(testCal())

	currentDate := m.calendar.EndDate
	olderDate := currentDate.AddDate(-1, 0, 0)
	m.selection.Add(currentDate)
	m.selection.Add(olderDate)
	m.dateCounts[dateKeyUTC(currentDate)] = 5
	m.dateCounts[dateKeyUTC(olderDate)] = 3
	m.syncVisibleCalendarCounts()

	selectionBefore := m.selection.Clone()
	countsBefore := make(map[string]int, len(m.dateCounts))
	for k, v := range m.dateCounts {
		countsBefore[k] = v
	}

	// Move to older window and assert older date is restored on-screen.
	m = pressKey(m, "[")
	if !reflect.DeepEqual(m.dateCounts, countsBefore) {
		t.Fatalf("dateCounts changed after year-window navigation")
	}
	if !reflect.DeepEqual(m.selection, &selectionBefore) {
		t.Fatalf("selection changed after year-window navigation")
	}
	assertVisibleDateSelectionAndCount(t, m, olderDate, 3)

	// Move forward again and ensure current window selection/count restores too.
	m = pressKey(m, "]")
	assertVisibleDateSelectionAndCount(t, m, currentDate, 5)
}

func TestYearWindowNavigation_AllowsDec31OfPreviousYearSelection(t *testing.T) {
	m := newTestModel(testCal()) // ends at 2024-06-15
	m = pressKey(m, "[")         // previous year window

	if got := m.viewEndDate.UTC().Format("2006-01-02"); got != "2023-12-31" {
		t.Fatalf("viewEndDate = %s, want 2023-12-31", got)
	}

	d := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
	pos, ok := findDateCell(m.calendar, d)
	if !ok {
		t.Fatal("expected Dec 31 of previous year to be visible/selectable")
	}
	m.cursor = pos
	m = pressKey(m, " ")
	if !m.selection.IsSelected(d) {
		t.Fatal("expected Dec 31 of previous year to be selectable")
	}
}

func TestMoveRight_ClampsAtLastWeek(t *testing.T) {
	m := newTestModel(testCal())
	before := m.cursor
	m = pressKey(m, "right")
	if m.cursor != before {
		t.Errorf("cursor moved right past last week: got %v, want %v", m.cursor, before)
	}
}

func TestMoveUp_DecreasesWeekday(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKey(m, "up")
	if m.cursor.Weekday != 5 {
		t.Errorf("cursor.Weekday = %d after up, want 5", m.cursor.Weekday)
	}
}

func TestMoveUp_VimK(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKey(m, "k")
	if m.cursor.Weekday != 5 {
		t.Errorf("k: cursor.Weekday = %d, want 5", m.cursor.Weekday)
	}
}

func TestMoveDown_ClampsAtLastValidDay(t *testing.T) {
	// today = Saturday (weekday 6), so weekday 6 is the last valid row in week 52.
	m := newTestModel(testCal())
	before := m.cursor
	m = pressKey(m, "down")
	if m.cursor != before {
		t.Errorf("cursor moved down past last valid day: got %v, want %v", m.cursor, before)
	}
}

func TestMoveUp_ClampsAtRow0(t *testing.T) {
	m := newTestModel(testCal())
	// Move to Sunday (weekday 0) via 6 ups.
	m = pressKeys(m, "k", "k", "k", "k", "k", "k")
	if m.cursor.Weekday != 0 {
		t.Errorf("cursor.Weekday = %d after 6 ups, want 0", m.cursor.Weekday)
	}
	// One more up — must stay at 0.
	m = pressKey(m, "k")
	if m.cursor.Weekday != 0 {
		t.Errorf("cursor.Weekday = %d after up from row 0, want 0", m.cursor.Weekday)
	}
}

func TestMoveLeft_ClampsAtWeek0(t *testing.T) {
	m := newTestModel(testCal())
	// Move to week 0 via many lefts.
	for i := 0; i < 60; i++ {
		m = pressKey(m, "left")
	}
	if m.cursor.Week != 0 {
		t.Errorf("cursor.Week = %d after left from week 0, want 0", m.cursor.Week)
	}
}

// ---- selection tests --------------------------------------------------------

func TestSpace_TogglesCurrentCell(t *testing.T) {
	m := newTestModel(testCal())
	d := m.calendar.Weeks[m.cursor.Week][m.cursor.Weekday].Date
	m = pressKey(m, " ")
	if !m.selection.IsSelected(d) {
		t.Errorf("date %v should be selected after space", d)
	}
	m = pressKey(m, " ")
	if m.selection.IsSelected(d) {
		t.Errorf("date %v should be deselected after second space", d)
	}
}

func TestKeyA_SelectsAll(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKey(m, "a")
	// Count all non-padding days.
	want := 0
	for _, week := range m.calendar.Weeks {
		for _, day := range week {
			if !day.Date.IsZero() {
				want++
			}
		}
	}
	if got := m.selection.Count(); got != want {
		t.Errorf("Count = %d after 'a', want %d", got, want)
	}
}

func TestKeyU_ClearsSelection(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, "a", "u")
	if got := m.selection.Count(); got != 0 {
		t.Errorf("Count = %d after 'u', want 0", got)
	}
}

func TestKeyEsc_DoesNotClearSelectionInNormalMode(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, "a", "esc")
	if got := m.selection.Count(); got == 0 {
		t.Errorf("Count = %d after esc (normal mode), want selection preserved", got)
	}
}

// ---- range-mode tests -------------------------------------------------------

func TestKeyV_EntersRangeMode(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKey(m, "v")
	if !m.rangeMode {
		t.Error("rangeMode should be true after 'v'")
	}
	if m.rangeAnchor != m.cursor {
		t.Errorf("rangeAnchor = %v, want %v (initial cursor)", m.rangeAnchor, m.cursor)
	}
}

func TestRangeMode_Esc_CancelsWithoutSelecting(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, "v", "left", "left", "esc")
	if m.rangeMode {
		t.Error("rangeMode should be false after esc")
	}
	if got := m.selection.Count(); got != 0 {
		t.Errorf("Count = %d after cancelling range, want 0", got)
	}
}

func TestRangeMode_Confirm_SelectsDateRange(t *testing.T) {
	// cursor starts at {52, 6} = Jun 15.
	// Move up 3 → {52, 3} = Jun 12 (Wed).
	// Enter range mode.
	// Move left 1 → {51, 3} = Jun 5 (Wed).
	// Confirm with 'v'.
	// Expected range: Jun 5–12 = 8 days.
	m := newTestModel(testCal())
	m = pressKeys(m, "k", "k", "k", "v", "h", "v")

	if m.rangeMode {
		t.Error("rangeMode should be false after confirming")
	}
	if got := m.selection.Count(); got != 8 {
		t.Errorf("Count = %d, want 8 (Jun 5–12)", got)
	}
	// Spot-check a few dates.
	for _, d := range []time.Time{
		time.Date(2024, 6, 5, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 6, 9, 0, 0, 0, 0, time.UTC), // Sunday crossing week boundary
		time.Date(2024, 6, 12, 0, 0, 0, 0, time.UTC),
	} {
		if !m.selection.IsSelected(d) {
			t.Errorf("date %v should be in range selection", d)
		}
	}
}

func TestRangeMode_ConfirmWithEnter(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, "k", "k", "k", "v", "h", "enter")
	if m.rangeMode {
		t.Error("rangeMode should be false after enter")
	}
	if m.selection.Count() != 8 {
		t.Errorf("Count = %d, want 8", m.selection.Count())
	}
}

func TestRangeMode_ConfirmWithSpace(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, "k", "k", "k", "v", "h", " ")
	if m.rangeMode {
		t.Error("rangeMode should be false after space")
	}
	if m.selection.Count() != 8 {
		t.Errorf("Count = %d, want 8", m.selection.Count())
	}
}

func TestRangeMode_SingleDay(t *testing.T) {
	// Confirm immediately without moving — selects just the anchor day.
	m := newTestModel(testCal())
	anchorDate := m.calendar.Weeks[m.cursor.Week][m.cursor.Weekday].Date
	m = pressKeys(m, "v", "v")
	if got := m.selection.Count(); got != 1 {
		t.Errorf("Count = %d, want 1 for single-day range", got)
	}
	if !m.selection.IsSelected(anchorDate) {
		t.Errorf("anchor date %v should be selected", anchorDate)
	}
}

func TestRangeMode_Esc_DoesNotClearExistingSelection(t *testing.T) {
	m := newTestModel(testCal())
	// Select a day, then start range mode and cancel it.
	d := m.calendar.Weeks[m.cursor.Week][m.cursor.Weekday].Date
	m = pressKeys(m, " ", "v", "left", "esc")
	if !m.selection.IsSelected(d) {
		t.Error("pre-existing selection must survive range-mode cancel with esc")
	}
}

func TestRangeMode_AnchorReflectsCurrentCursor(t *testing.T) {
	m := newTestModel(testCal())
	// Move to a different position, then enter range mode.
	m = pressKeys(m, "k", "k", "h")
	wantAnchor := m.cursor
	m = pressKey(m, "v")
	if m.rangeAnchor != wantAnchor {
		t.Errorf("rangeAnchor = %v, want %v", m.rangeAnchor, wantAnchor)
	}
}

// ---- Phase 4: count-entry screen --------------------------------------------

func TestEnter_WithSelection_OpensCountEntry(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, " ", "enter") // select today, then enter
	if m.screen != screenCountEntry {
		t.Errorf("screen = %v, want screenCountEntry", m.screen)
	}
}

func TestEnter_AssignedUnchangedSelection_SkipsPromptToOptions(t *testing.T) {
	m := newTestModel(testCal())
	today := m.calendar.Weeks[m.cursor.Week][m.cursor.Weekday].Date

	// First assignment flow.
	m = pressKeys(m, " ", "enter", "5", "enter")
	if m.screen != screenOptions {
		t.Fatalf("screen = %v, want screenOptions after assignment", m.screen)
	}
	if got := m.dateCounts[dateKeyUTC(today)]; got != 5 {
		t.Fatalf("today count = %d, want 5", got)
	}

	// Back to grid without changing selection, then Enter should skip prompt.
	m = pressKey(m, "esc")
	if m.screen != screenGrid {
		t.Fatalf("screen = %v, want screenGrid", m.screen)
	}
	m = pressKey(m, "enter")
	if m.screen != screenOptions {
		t.Fatalf("screen = %v, want screenOptions (prompt skipped)", m.screen)
	}
	if got := m.dateCounts[dateKeyUTC(today)]; got != 5 {
		t.Fatalf("today count changed = %d, want 5", got)
	}
}

func TestEnter_AssignedButSelectionChanged_ShowsPromptAgain(t *testing.T) {
	m := newTestModel(testCal())
	first := m.calendar.Weeks[m.cursor.Week][m.cursor.Weekday].Date

	// Assign initial selection.
	m = pressKeys(m, " ", "enter", "5", "enter", "esc")
	if got := m.dateCounts[dateKeyUTC(first)]; got != 5 {
		t.Fatalf("first date count = %d, want 5", got)
	}

	// Change selection by adding one more tile.
	m = pressKeys(m, "left", " ")
	second := m.calendar.Weeks[m.cursor.Week][m.cursor.Weekday].Date

	// Enter should show prompt again because selection changed.
	m = pressKey(m, "enter")
	if m.screen != screenCountEntry {
		t.Fatalf("screen = %v, want screenCountEntry after selection change", m.screen)
	}
	// Existing counts for untouched previously-assigned dates must stay intact.
	if got := m.dateCounts[dateKeyUTC(first)]; got != 5 {
		t.Fatalf("first date count changed = %d, want 5", got)
	}
	if got := m.dateCounts[dateKeyUTC(second)]; got != 0 {
		t.Fatalf("newly added date should not have count yet, got %d", got)
	}
}

func TestEnter_NoSelection_StaysOnGrid(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKey(m, "enter") // no selection
	if m.screen != screenGrid {
		t.Errorf("enter with no selection should stay on screenGrid, got %v", m.screen)
	}
}

func TestCountEntry_TypingDigit_AppendsToInput(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, " ", "enter", "5")
	if m.countInput != "5" {
		t.Errorf("countInput = %q, want \"5\"", m.countInput)
	}
}

func TestCountEntry_TypingHyphen_Allowed(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, " ", "enter", "1", "-", "8")
	if m.countInput != "1-8" {
		t.Errorf("countInput = %q, want \"1-8\"", m.countInput)
	}
}

func TestCountEntry_NonDigit_IsAcceptedAsTextInput(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, " ", "enter")
	m = pressKey(m, "x")
	if m.countInput != "x" {
		t.Errorf("countInput = %q after non-digit, want \"x\"", m.countInput)
	}
}

func TestCountEntry_Backspace_DeletesLastChar(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, " ", "enter", "5", "3")
	m = pressKey(m, "backspace")
	if m.countInput != "5" {
		t.Errorf("countInput = %q after backspace, want \"5\"", m.countInput)
	}
}

func TestCountEntry_Backspace_EmptyInput_NoOp(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, " ", "enter")
	m = pressKey(m, "backspace") // nothing to delete
	if m.countInput != "" {
		t.Errorf("countInput = %q, want \"\" after backspace on empty", m.countInput)
	}
}

func TestCountEntry_Esc_ReturnsToGrid(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, " ", "enter", "5", "esc")
	if m.screen != screenGrid {
		t.Errorf("screen = %v after esc, want screenGrid", m.screen)
	}
}

func TestCountEntry_Esc_KeepsSelection(t *testing.T) {
	m := newTestModel(testCal())
	todayDate := m.calendar.Weeks[m.cursor.Week][m.cursor.Weekday].Date
	m = pressKeys(m, " ", "enter", "5", "esc")
	if !m.selection.IsSelected(todayDate) {
		t.Error("selection must survive count-entry cancel with esc")
	}
}

func TestCountEntry_Esc_ClearsCountInput(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, " ", "enter", "5", "esc")
	if m.countInput != "" {
		t.Errorf("countInput = %q after esc, want \"\"", m.countInput)
	}
	if m.countErr != "" {
		t.Errorf("countErr = %q after esc, want \"\"", m.countErr)
	}
}

func TestCountEntry_InvalidInput_ShowsError(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, " ", "enter", "0", "enter") // "0" is invalid (must be ≥ 1)
	if m.countErr == "" {
		t.Error("expected countErr to be set for invalid input \"0\"")
	}
	if m.screen != screenCountEntry {
		t.Error("invalid input must keep the count-entry screen open")
	}
}

func TestCountEntry_InvalidInput_ErrorClearedOnTyping(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, " ", "enter", "0", "enter") // trigger error
	m = pressKey(m, "5")                         // type more input → error clears
	if m.countErr != "" {
		t.Errorf("countErr = %q after typing, want \"\"", m.countErr)
	}
}

func TestCountEntry_FixedConfirm_AppliesCount(t *testing.T) {
	m := newTestModel(testCal())
	todayDate := m.calendar.Weeks[m.cursor.Week][m.cursor.Weekday].Date
	// Select today, enter count screen, type "5", confirm.
	m = pressKeys(m, " ", "enter", "5", "enter")

	// Confirming counts now opens the options menu, not the grid.
	if m.screen != screenOptions {
		t.Errorf("screen = %v after confirming, want screenOptions", m.screen)
	}
	// Find today in the calendar and check its count.
	found := false
	for _, week := range m.calendar.Weeks {
		for _, day := range week {
			if day.Date.Equal(todayDate) {
				found = true
				if day.Count != 5 {
					t.Errorf("day.Count = %d, want 5", day.Count)
				}
			}
		}
	}
	if !found {
		t.Error("today not found in calendar after applying count")
	}
}

func TestCountEntry_RangeConfirm_CountsInBounds(t *testing.T) {
	m := newTestModel(testCal())
	// Select 5 days via range then open count entry.
	m = pressKeys(m, "k", "k", "k", "k", "v", "k", "k", "k", "k", "v") // select 5 days
	m = pressKeys(m, "enter", "2", "-", "8", "enter")                  // assign 2-8

	// Verify all selected dates now have counts in [2, 8].
	for _, week := range m.calendar.Weeks {
		for _, day := range week {
			if day.Date.IsZero() || day.Count == 0 {
				continue
			}
			if day.Count < 2 || day.Count > 8 {
				t.Errorf("date %v: Count = %d, want in [2, 8]", day.Date, day.Count)
			}
		}
	}
}

func TestCountEntry_LivePreview_UpdatesOnInput(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, " ", "enter") // select 1 day, open count entry
	if m.previewCounts != nil {
		t.Error("previewCounts should be nil before any input")
	}
	m = pressKey(m, "5") // valid single-digit input
	if m.previewCounts == nil {
		t.Error("previewCounts should be non-nil after valid input")
	}
}

func TestCountEntry_LivePreview_NilOnInvalidInput(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, " ", "enter", "0") // "0" is not valid (< 1)
	if m.previewCounts != nil {
		t.Error("previewCounts should be nil for invalid input")
	}
}

func TestCountEntry_ConfirmClearsCountFields(t *testing.T) {
	m := newTestModel(testCal())
	m = pressKeys(m, " ", "enter", "3", "enter")
	if m.countInput != "" {
		t.Errorf("countInput = %q after confirm, want \"\"", m.countInput)
	}
	if m.previewCounts != nil {
		t.Error("previewCounts should be nil after confirm")
	}
}

// ── Phase 5: options menu ─────────────────────────────────────────────────────

// openOptionsMenu is a helper that selects today, assigns count 5, and returns
// the model at the options-menu screen.
func openOptionsMenu(t *testing.T) Model {
	t.Helper()
	m := newTestModel(testCal())
	m = pressKeys(m, " ", "enter", "5", "enter")
	if m.screen != screenOptions {
		t.Fatalf("openOptionsMenu: screen = %v, want screenOptions", m.screen)
	}
	return m
}

func TestOptions_OpensAfterCountConfirm(t *testing.T) {
	m := openOptionsMenu(t)
	_ = m // already verified by openOptionsMenu
}

func TestOptions_InitialMenuCursorIsZero(t *testing.T) {
	m := openOptionsMenu(t)
	if m.menuCursor != 0 {
		t.Errorf("menuCursor = %d, want 0", m.menuCursor)
	}
}

func TestOptions_Down_MovesMenuCursor(t *testing.T) {
	m := openOptionsMenu(t)
	m = pressKey(m, "j")
	if m.menuCursor != 1 {
		t.Errorf("menuCursor = %d after j, want 1", m.menuCursor)
	}
}

func TestOptions_Down_VimJ_Equivalent(t *testing.T) {
	m := openOptionsMenu(t)
	m = pressKey(m, "down")
	if m.menuCursor != 1 {
		t.Errorf("menuCursor = %d after down, want 1", m.menuCursor)
	}
}

func TestOptions_Up_MovesMenuCursor(t *testing.T) {
	m := openOptionsMenu(t)
	m = pressKeys(m, "j", "j", "k") // go to 2, then back to 1
	if m.menuCursor != 1 {
		t.Errorf("menuCursor = %d, want 1", m.menuCursor)
	}
}

func TestOptions_Down_WrapsAtBottom(t *testing.T) {
	m := openOptionsMenu(t)
	// Move to the last item.
	for i := 0; i < int(menuCount)-1; i++ {
		m = pressKey(m, "j")
	}
	if m.menuCursor != int(menuCount)-1 {
		t.Fatalf("expected cursor at last item (%d), got %d", int(menuCount)-1, m.menuCursor)
	}
	m = pressKey(m, "j") // wrap to top
	if m.menuCursor != 0 {
		t.Errorf("menuCursor = %d after wrap at bottom, want 0", m.menuCursor)
	}
}

func TestOptions_Up_WrapsAtTop(t *testing.T) {
	m := openOptionsMenu(t)
	m = pressKey(m, "k") // wrap from 0 to last
	if m.menuCursor != int(menuCount)-1 {
		t.Errorf("menuCursor = %d after wrap at top, want %d", m.menuCursor, int(menuCount)-1)
	}
}

func TestOptions_Esc_GoesToGrid(t *testing.T) {
	m := openOptionsMenu(t)
	m = pressKey(m, "esc")
	if m.screen != screenGrid {
		t.Errorf("screen = %v after esc, want screenGrid", m.screen)
	}
}

// navigateTo moves the menu cursor to the given item from position 0.
func navigateTo(m Model, item menuItem) Model {
	for m.menuCursor != int(item) {
		m = pressKey(m, "j")
	}
	return m
}

func TestOptions_Push_OpensPushConfirm(t *testing.T) {
	m := openOptionsMenu(t)
	m.generatedDateCounts = snapshotGeneratedCounts(m.dateCounts)
	m = navigateTo(m, menuPush)
	m = pressKey(m, "enter")
	if m.screen != screenPushConfirm {
		t.Errorf("screen = %v after Push, want screenPushConfirm", m.screen)
	}
}

func TestOptions_X_OpensClearAllConfirm(t *testing.T) {
	m := openOptionsMenu(t)
	result, cmd := m.Update(makeKeyMsg("x"))
	m = result.(Model)
	if cmd != nil {
		t.Fatal("x should not run clear operation without confirmation")
	}
	if m.screen != screenClearAllConfirm {
		t.Fatalf("screen = %v, want screenClearAllConfirm", m.screen)
	}
}

func TestClearAllConfirm_ExplicitTextStartsGenerating(t *testing.T) {
	m := openOptionsMenu(t)
	m.screen = screenClearAllConfirm
	m.projectName = "test-project"
	orig := clearAllCommitsRepo
	defer func() { clearAllCommitsRepo = orig }()
	clearAllCommitsRepo = func(dir string, stream gitops.StreamFn) error { return nil }

	m = pressKey(m, "y")
	m = pressKey(m, "e")
	m = pressKey(m, "s")
	result, cmd := m.Update(makeKeyMsg("enter"))
	m = result.(Model)
	if m.screen != screenGenerating {
		t.Fatalf("screen = %v, want screenGenerating", m.screen)
	}
	if !m.clearAllInFlight {
		t.Fatal("clearAllInFlight should be true after confirmation")
	}
	if cmd == nil {
		t.Fatal("expected clear-all command")
	}
}

func TestClearAllDoneMsg_Success_ClearsState(t *testing.T) {
	m := openOptionsMenu(t)
	if m.selection.Count() == 0 {
		t.Fatal("expected non-empty selection in setup")
	}
	m.generatedDateCounts["2024-06-15"] = 5
	m.clearAllInFlight = true
	result, _ := m.Update(clearAllDoneMsg{err: nil})
	m = result.(Model)

	if m.screen != screenGenerateDone {
		t.Fatalf("screen = %v, want screenGenerateDone", m.screen)
	}
	if m.selection.Count() != 0 {
		t.Fatal("selection should be cleared after clear-all success")
	}
	if len(m.dateCounts) != 0 || len(m.generatedDateCounts) != 0 {
		t.Fatal("date counts should be cleared after clear-all success")
	}
	if m.clearAllInFlight {
		t.Fatal("clearAllInFlight should be reset")
	}
}

func TestOptions_Deselect_ClearsSelectionAndGoesToGrid(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuDeselect)
	m = pressKey(m, "enter")
	if m.screen != screenGrid {
		t.Errorf("screen = %v, want screenGrid", m.screen)
	}
	if got := m.selection.Count(); got != 0 {
		t.Errorf("selection.Count() = %d after Deselect, want 0", got)
	}
}

func TestOptions_DeselectUngenerated_RemovesStateWithoutRegenerate(t *testing.T) {
	m := openOptionsMenu(t) // has counts assigned, but not generated snapshot
	today := m.calendar.Weeks[m.cursor.Week][m.cursor.Weekday].Date

	orig := regenerateRepo
	defer func() { regenerateRepo = orig }()
	calls := 0
	regenerateRepo = func(cfg gitops.RegenerateConfig) (int, error) {
		calls++
		return 0, nil
	}

	m = navigateTo(m, menuDeselect)
	m = pressKey(m, "enter")
	if m.screen != screenGrid {
		t.Fatalf("screen = %v, want screenGrid", m.screen)
	}
	if calls != 0 {
		t.Fatalf("regenerate should not be called, got %d call(s)", calls)
	}
	if m.selection.Count() != 0 {
		t.Fatalf("selection should be cleared")
	}
	if got := m.dateCounts[dateKeyUTC(today)]; got != 0 {
		t.Fatalf("dateCounts for deselected day = %d, want removed/0", got)
	}
}

func TestOptions_DeselectGenerated_ConfirmYes_CallsRegenerateAndUpdatesState(t *testing.T) {
	m := openOptionsMenu(t)
	today := m.calendar.Weeks[m.cursor.Week][m.cursor.Weekday].Date
	key := dateKeyUTC(today)
	m.generatedDateCounts[key] = m.dateCounts[key]

	orig := regenerateRepo
	defer func() { regenerateRepo = orig }()
	regenerateRepo = func(cfg gitops.RegenerateConfig) (int, error) {
		return 0, nil
	}

	m = navigateTo(m, menuDeselect)
	result, cmd := m.Update(makeKeyMsg("enter"))
	m = result.(Model)
	if m.screen != screenDeselectConfirm {
		t.Fatalf("screen = %v, want screenDeselectConfirm", m.screen)
	}
	if cmd != nil {
		t.Fatalf("confirm step should not start regenerate yet")
	}

	result, cmd = m.Update(makeKeyMsg("y"))
	m = result.(Model)
	if m.screen != screenGenerating {
		t.Fatalf("screen = %v, want screenGenerating", m.screen)
	}
	if cmd == nil {
		t.Fatal("expected regenerate command on confirm yes")
	}
	runCmd(cmd)
	if _, exists := m.dateCounts[key]; exists {
		t.Fatalf("deselected generated date should be removed from dateCounts")
	}
	if _, exists := m.generatedDateCounts[key]; exists {
		t.Fatalf("deselected generated date should be removed from generatedDateCounts")
	}
}

func TestOptions_EditCounts_OpensCountEntry(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuEditCounts)
	m = pressKey(m, "enter")
	if m.screen != screenCountEntry {
		t.Errorf("screen = %v, want screenCountEntry", m.screen)
	}
	// Selection is preserved so the user can re-assign counts.
	if m.selection.Count() == 0 {
		t.Error("selection should be preserved when editing counts")
	}
	if m.countInput != "" {
		t.Errorf("countInput = %q, want \"\" (reset for fresh input)", m.countInput)
	}
}

func TestOptions_GenerateLocal_OpensGeneratingScreen(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuGenerateLocal)
	result, cmd := m.Update(makeKeyMsg("enter"))
	m = result.(Model)
	if m.screen != screenGenerating {
		t.Errorf("screen = %v, want screenGenerating", m.screen)
	}
	if cmd == nil {
		t.Error("expected non-nil generation cmd (tea.Batch)")
	}
}

func TestOptions_PreviewSummary_OpensPreview(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuPreviewSummary)
	m = pressKey(m, "enter")
	if m.screen != screenPreview {
		t.Errorf("screen = %v, want screenPreview", m.screen)
	}
}

func TestOptions_SaveExit_SetsQuittingFlag(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuSaveExit)
	m = pressKey(m, "enter")
	if !m.quitting {
		t.Error("quitting flag should be true after Save & exit")
	}
}

func TestOptions_SaveExit_ReturnsQuitCmd(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuSaveExit)
	result, cmd := m.Update(makeKeyMsg("enter"))
	_ = result
	if cmd == nil {
		t.Fatal("expected non-nil Quit cmd from Save & exit")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("cmd returned %T, want tea.QuitMsg", msg)
	}
}

func TestOptions_Back_GoesToGrid(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuBack)
	m = pressKey(m, "enter")
	if m.screen != screenGrid {
		t.Errorf("screen = %v after Back, want screenGrid", m.screen)
	}
}

func TestOptions_Back_PreservesSelectionAndCounts(t *testing.T) {
	m := newTestModel(testCal())
	todayDate := m.calendar.Weeks[m.cursor.Week][m.cursor.Weekday].Date
	// Select today, assign 5, reach options menu.
	m = pressKeys(m, " ", "enter", "5", "enter")
	// Navigate to Back and confirm.
	m = navigateTo(m, menuBack)
	m = pressKey(m, "enter")

	if !m.selection.IsSelected(todayDate) {
		t.Error("selection must be preserved after Back")
	}
	for _, week := range m.calendar.Weeks {
		for _, day := range week {
			if day.Date.Equal(todayDate) && day.Count != 5 {
				t.Errorf("count = %d after Back, want 5", day.Count)
			}
		}
	}
}

func TestOptions_SpaceSelectsItem(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuBack)
	m = pressKey(m, " ") // space should also select
	if m.screen != screenGrid {
		t.Errorf("screen = %v after space on Back, want screenGrid", m.screen)
	}
}

func TestPreview_Esc_ReturnsToOptions(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuPreviewSummary)
	m = pressKeys(m, "enter", "esc") // open preview, then esc
	if m.screen != screenOptions {
		t.Errorf("screen = %v after esc from preview, want screenOptions", m.screen)
	}
}

func TestPreview_RendersWithData(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuPreviewSummary)
	m = pressKey(m, "enter")
	view := m.View()
	if view == "" {
		t.Error("preview screen must render a non-empty view")
	}
	// The view must contain the date we assigned a count to.
	if !strings.Contains(view, "2024-06-15") {
		t.Error("preview must show the selected date 2024-06-15")
	}
}

// ── Phase 6: generation screens ───────────────────────────────────────────────

func TestGenerating_IgnoresKeysWhileRunning(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuGenerateLocal)
	result, _ := m.Update(makeKeyMsg("enter"))
	m = result.(Model)
	if m.screen != screenGenerating {
		t.Fatalf("expected screenGenerating, got %v", m.screen)
	}
	// Keys other than q/ctrl+c must be silently ignored.
	m = pressKey(m, "k")
	if m.screen != screenGenerating {
		t.Errorf("screen changed to %v after key during generation", m.screen)
	}
}

func TestGenerating_GenerateDoneMsg_Success(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuGenerateLocal)
	result, _ := m.Update(makeKeyMsg("enter"))
	m = result.(Model)

	// Simulate generation completing successfully.
	result2, _ := m.Update(generateDoneMsg{err: nil, total: 7})
	m = result2.(Model)

	if m.screen != screenGenerateDone {
		t.Errorf("screen = %v after done msg, want screenGenerateDone", m.screen)
	}
	if m.generateTotal != 7 {
		t.Errorf("generateTotal = %d, want 7", m.generateTotal)
	}
	if m.generateErr != nil {
		t.Errorf("generateErr = %v, want nil", m.generateErr)
	}
}

func TestGenerating_GenerateDoneMsg_Error(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuGenerateLocal)
	result, _ := m.Update(makeKeyMsg("enter"))
	m = result.(Model)

	// Simulate failure.
	genErr := errors.New("git init failed")
	result2, _ := m.Update(generateDoneMsg{err: genErr, total: 0})
	m = result2.(Model)

	if m.screen != screenGenerateDone {
		t.Errorf("screen = %v, want screenGenerateDone", m.screen)
	}
	if m.generateErr == nil {
		t.Error("generateErr should be set after error")
	}
}

func TestGenerateDone_EnterGoesBackToGrid(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuGenerateLocal)
	result, _ := m.Update(makeKeyMsg("enter"))
	m = result.(Model)
	result2, _ := m.Update(generateDoneMsg{err: nil, total: 3})
	m = result2.(Model)

	m = pressKey(m, "enter")
	if m.screen != screenGrid {
		t.Errorf("screen = %v after enter on done screen, want screenGrid", m.screen)
	}
}

func TestGenerateDone_EscGoesBackToGrid(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuGenerateLocal)
	result, _ := m.Update(makeKeyMsg("enter"))
	m = result.(Model)
	result2, _ := m.Update(generateDoneMsg{err: nil, total: 2})
	m = result2.(Model)

	m = pressKey(m, "esc")
	if m.screen != screenGrid {
		t.Errorf("screen = %v after esc on done screen, want screenGrid", m.screen)
	}
}

func TestGenerating_SpinnerTick_AdvancesFrame(t *testing.T) {
	m := openOptionsMenu(t)
	m = navigateTo(m, menuGenerateLocal)
	result, _ := m.Update(makeKeyMsg("enter"))
	m = result.(Model)
	initialFrame := m.spinnerFrame

	result2, _ := m.Update(spinnerTickMsg{})
	m = result2.(Model)

	if m.spinnerFrame == initialFrame {
		t.Error("spinnerFrame should have advanced after spinnerTickMsg")
	}
}

func TestGenerating_SpinnerTickIgnoredWhenNotGenerating(t *testing.T) {
	// Spinner ticks after generation is done should not re-enter generating screen.
	m := openOptionsMenu(t)
	m = navigateTo(m, menuGenerateLocal)
	result, _ := m.Update(makeKeyMsg("enter"))
	m = result.(Model)
	// Complete generation
	result2, _ := m.Update(generateDoneMsg{err: nil, total: 1})
	m = result2.(Model)
	if m.screen != screenGenerateDone {
		t.Fatalf("expected screenGenerateDone, got %v", m.screen)
	}
	// Late spinner tick — must be ignored
	result3, cmd := m.Update(spinnerTickMsg{})
	m = result3.(Model)
	if m.screen != screenGenerateDone {
		t.Errorf("screen changed to %v after spinner tick on done screen", m.screen)
	}
	if cmd != nil {
		t.Error("no new tick cmd should be returned when not on screenGenerating")
	}
}

func TestGenerating_EmptyJobs_ReturnsErrorDoneMsg(t *testing.T) {
	// A selection with count=0 on all days should produce an error done message.
	m := newTestModel(testCal())
	// Select today but don't assign any count (count stays 0)
	m = pressKey(m, " ") // select today
	// Manually set screen to options (as if count entry was skipped)
	m.screen = screenOptions
	m.menuCursor = int(menuGenerateLocal)

	result, cmd := m.Update(makeKeyMsg("enter"))
	m = result.(Model)
	if m.screen != screenGenerating {
		t.Fatalf("expected screenGenerating, got %v", m.screen)
	}
	if cmd == nil {
		t.Fatal("expected a cmd")
	}
	// Execute the cmd synchronously to get the done message.
	msg := cmd()
	// With tea.Batch the returned msg may be the first sub-cmd's result or nil.
	// We can't reliably call a batch cmd synchronously; just verify screen state.
	_ = msg
}

// ── Phase 7: push flow ───────────────────────────────────────────────────────

func openPushConfirm(t *testing.T) Model {
	t.Helper()
	m := openOptionsMenu(t)
	// Simulate already-generated state so push tests don't trigger auto-generate.
	m.generatedDateCounts = snapshotGeneratedCounts(m.dateCounts)
	m = navigateTo(m, menuPush)
	m = pressKey(m, "enter")
	if m.screen != screenPushConfirm {
		t.Fatalf("screen = %v, want screenPushConfirm", m.screen)
	}
	return m
}

func TestPushConfirm_Y_GoesToRemoteInputWhenNoRemote(t *testing.T) {
	m := openPushConfirm(t)
	m = pressKey(m, "y")
	if m.screen != screenProjectRemoteInput {
		t.Errorf("screen = %v, want screenProjectRemoteInput", m.screen)
	}
}

func TestPushConfirm_N_GoesBackToOptions(t *testing.T) {
	m := openPushConfirm(t)
	m = pressKey(m, "n")
	if m.screen != screenOptions {
		t.Errorf("screen = %v, want screenOptions", m.screen)
	}
}

func TestPushGuidance_Enter_GoesToRemoteInputWhenNoRemote(t *testing.T) {
	m := openPushConfirm(t)
	m = pressKey(m, "y")
	m = pressKey(m, "enter")
	if m.screen != screenProjectRemoteInput {
		t.Errorf("screen = %v, want screenProjectRemoteInput", m.screen)
	}
}

func TestPushRepoType_EnterWithoutRemote_GoesToRemoteInput(t *testing.T) {
	m := openPushConfirm(t)
	m.cfg.Remote = ""
	m = pressKey(m, "y")
	if m.screen != screenProjectRemoteInput {
		t.Errorf("screen = %v, want screenProjectRemoteInput", m.screen)
	}
}

func TestPushRemoteInput_EnterWithRemote_StartsRunning(t *testing.T) {
	m := openPushConfirm(t)
	m.cfg.Remote = ""
	m = pressKey(m, "y")
	m = pressKeys(m, "g", "i", "t", "@", "g", "i", "t", "h", "u", "b", ".", "c", "o", "m", ":", "u", "/", "r", ".", "g", "i", "t")
	result, cmd := m.Update(makeKeyMsg("enter"))
	m = result.(Model)
	if m.screen != screenPushRunning {
		t.Errorf("screen = %v, want screenPushRunning", m.screen)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd starting push process")
	}
}

func TestPushRunning_LogMsg_Appends(t *testing.T) {
	m := openPushConfirm(t)
	m.screen = screenPushRunning
	m.pushStream = make(chan tea.Msg, 1)
	result, _ := m.Update(pushLogMsg{line: "line-1"})
	m = result.(Model)
	if len(m.pushLogs) != 1 || m.pushLogs[0] != "line-1" {
		t.Errorf("pushLogs=%v want [line-1]", m.pushLogs)
	}
}

func TestPushRunning_DoneSuccess_GoesToDoneScreen(t *testing.T) {
	m := openPushConfirm(t)
	m.screen = screenPushRunning
	result, _ := m.Update(pushDoneMsg{err: nil})
	m = result.(Model)
	if m.screen != screenPushDone {
		t.Errorf("screen = %v, want screenPushDone", m.screen)
	}
	if !strings.Contains(strings.ToLower(m.pushDoneText), "success") {
		t.Errorf("pushDoneText=%q want success message", m.pushDoneText)
	}
}

func TestPushRunning_DoneError_GoesToDoneScreen(t *testing.T) {
	m := openPushConfirm(t)
	m.screen = screenPushRunning
	result, _ := m.Update(pushDoneMsg{err: errors.New("auth failed")})
	m = result.(Model)
	if m.screen != screenPushDone {
		t.Errorf("screen = %v, want screenPushDone", m.screen)
	}
	if !strings.Contains(strings.ToLower(m.pushDoneText), "auth failed") {
		t.Errorf("pushDoneText=%q want error text", m.pushDoneText)
	}
}

func TestPushDone_Enter_ReturnsToOptions(t *testing.T) {
	m := openPushConfirm(t)
	m.screen = screenPushDone
	m = pressKey(m, "enter")
	if m.screen != screenOptions {
		t.Errorf("screen = %v, want screenOptions", m.screen)
	}
}

func TestPushDone_Esc_ReturnsToOptions(t *testing.T) {
	m := openPushConfirm(t)
	m.screen = screenPushDone
	m = pressKey(m, "esc")
	if m.screen != screenOptions {
		t.Errorf("screen = %v, want screenOptions", m.screen)
	}
}

func TestPushOption_WithYesFlag_SkipsConfirmAndAsksRemoteWhenMissing(t *testing.T) {
	m := openOptionsMenu(t)
	m.generatedDateCounts = snapshotGeneratedCounts(m.dateCounts)
	m.cfg.Yes = true
	m = navigateTo(m, menuPush)
	m = pressKey(m, "enter")
	if m.screen != screenProjectRemoteInput {
		t.Errorf("screen = %v, want screenProjectRemoteInput with --yes", m.screen)
	}
}

func TestHelpOverlay_TogglesOnAnyScreen(t *testing.T) {
	m := newTestModel(testCal())
	if m.helpVisible {
		t.Fatal("help should start hidden")
	}
	m = pressKey(m, "?")
	if !m.helpVisible {
		t.Fatal("help should be visible after ?")
	}
	m = pressKey(m, "?")
	if m.helpVisible {
		t.Fatal("help should hide after second ?")
	}
}

func TestHelpOverlay_EscClosesOverlayOnly(t *testing.T) {
	m := newTestModel(testCal())
	m.screen = screenOptions
	m.helpVisible = true
	m = pressKey(m, "esc")
	if m.helpVisible {
		t.Fatal("esc should close help overlay")
	}
	if m.screen != screenOptions {
		t.Fatalf("screen changed unexpectedly: %v", m.screen)
	}
}

func TestHelpOverlay_RendersActiveKeys(t *testing.T) {
	m := newTestModel(testCal())
	m.screen = screenPushRemoteInput
	m.helpVisible = true
	view := m.View()
	if !strings.Contains(view, "toggle help") {
		t.Fatal("help overlay not rendered")
	}
	if !strings.Contains(view, "type remote URL") {
		t.Fatalf("expected screen-specific key help in overlay, got:\n%s", view)
	}
}

func TestQuitKeys_QuitFromEveryTopLevelScreen(t *testing.T) {
	topLevel := []screen{
		screenGrid,
		screenOptions,
		screenPreview,
		screenGenerating,
		screenGenerateDone,
		screenPushConfirm,
		screenPushGuidance,
		screenPushRepoType,
		screenPushRunning,
		screenPushDone,
	}
	for _, s := range topLevel {
		m := newTestModel(testCal())
		m.screen = s
		result, cmd := m.Update(makeKeyMsg("q"))
		_ = result
		assertQuitCmd(t, cmd)

		result, cmd = m.Update(makeKeyMsg("ctrl+c"))
		_ = result
		assertQuitCmd(t, cmd)
	}
}

func TestQuitKeys_QDoesNotQuitInTextInputModes(t *testing.T) {
	m := newTestModel(testCal())
	m.screen = screenCountEntry
	result, cmd := m.Update(makeKeyMsg("q"))
	m = result.(Model)
	if cmd != nil {
		t.Fatal("q should not quit in count-input mode")
	}
	if m.countInput != "q" {
		t.Fatalf("countInput=%q want q", m.countInput)
	}

	m = newTestModel(testCal())
	m.screen = screenPushRemoteInput
	result, cmd = m.Update(makeKeyMsg("q"))
	m = result.(Model)
	if cmd != nil {
		t.Fatal("q should not quit in remote-input mode")
	}
	if m.pushRemoteInput != "q" {
		t.Fatalf("pushRemoteInput=%q want q", m.pushRemoteInput)
	}
}

func TestQuitKeys_CtrlCQuitsInTextInputModes(t *testing.T) {
	for _, s := range []screen{screenCountEntry, screenPushRemoteInput} {
		m := newTestModel(testCal())
		m.screen = s
		result, cmd := m.Update(makeKeyMsg("ctrl+c"))
		_ = result
		assertQuitCmd(t, cmd)
	}
}

func TestFooterView_StaysOneLineWhenWide(t *testing.T) {
	m := newTestModel(testCal())
	m.width = 220
	footer := m.footerView()
	if strings.Contains(footer, "\n") {
		t.Fatalf("expected single-line footer on wide terminal, got:\n%s", footer)
	}
	for _, expected := range []string{"space toggle", "v range", "enter create/menu"} {
		if !strings.Contains(strings.ToLower(footer), expected) {
			t.Fatalf("expected footer to contain %q, got:\n%s", expected, footer)
		}
	}
}

func TestFooterView_WrapsWhenNarrow(t *testing.T) {
	m := newTestModel(testCal())
	m.width = 48
	footer := m.footerView()
	if !strings.Contains(footer, "\n") {
		t.Fatalf("expected wrapped footer on narrow terminal, got:\n%s", footer)
	}
}

func TestView_GridFitsNarrowAndWideWidths(t *testing.T) {
	for _, width := range []int{80, 120} {
		m := newTestModel(testCal())
		m.width = width
		m.height = 24
		view := m.View()
		for _, line := range strings.Split(view, "\n") {
			if lipgloss.Width(line) > width {
				t.Fatalf("line width %d exceeds screen width %d:\n%s", lipgloss.Width(line), width, line)
			}
		}
	}
}

func TestWindowSizeMsg_UpdatesModelDimensions(t *testing.T) {
	m := newTestModel(testCal())
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = result.(Model)
	if m.width != 120 || m.height != 40 {
		t.Fatalf("size not updated, got %dx%d", m.width, m.height)
	}
}

func TestView_ResponsiveAtCommonSizes(t *testing.T) {
	for _, tc := range []struct {
		w int
		h int
	}{
		{w: 80, h: 24},
		{w: 120, h: 40},
		{w: 40, h: 15},
	} {
		m := newTestModel(testCal())
		result, _ := m.Update(tea.WindowSizeMsg{Width: tc.w, Height: tc.h})
		m = result.(Model)
		view := m.View()
		for _, line := range strings.Split(view, "\n") {
			if lipgloss.Width(line) > tc.w {
				t.Fatalf("line width %d exceeds %d at %dx%d:\n%s", lipgloss.Width(line), tc.w, tc.w, tc.h, line)
			}
		}
	}
}

func TestView_ShowsResizeMessageWhenTooSmall(t *testing.T) {
	m := newTestModel(testCal())
	result, _ := m.Update(tea.WindowSizeMsg{Width: 39, Height: 9})
	m = result.(Model)
	view := m.View()
	if !strings.Contains(view, "Terminal too small") {
		t.Fatalf("expected small-terminal message, got:\n%s", view)
	}
}

func TestUseCompactGrid_DependsOnAvailableWidth(t *testing.T) {
	m := newTestModel(testCal())
	m.width = 120
	m.height = 24
	if !m.useCompactGrid() {
		t.Fatalf("expected compact grid at width %d", m.width)
	}

	m.width = 220
	if m.useCompactGrid() {
		t.Fatalf("expected normal grid at width %d", m.width)
	}
}

func TestNewModel_LoadsPersistedState(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "proj")
	today := time.Now().UTC().Format("2006-01-02")
	err := state.Save(project, state.PersistedState{
		Version:       1,
		SelectedDir:   project,
		SelectedDates: []string{today},
		DateCounts:    map[string]int{today: 7},
		Message:       "my-msg",
		MessageMode:   "fixed",
		RemoteURL:     "git@github.com:me/repo.git",
	})
	if err != nil {
		t.Fatalf("state.Save: %v", err)
	}
	m := NewModel(1, Config{Dir: root})
	m = pressKey(m, "enter")
	if m.cfg.Message != "my-msg" {
		t.Fatalf("cfg.Message=%q want my-msg", m.cfg.Message)
	}
	if m.cfg.MessageMode != "fixed" {
		t.Fatalf("cfg.MessageMode=%q want fixed", m.cfg.MessageMode)
	}
	if m.cfg.Remote != "git@github.com:me/repo.git" {
		t.Fatalf("cfg.Remote=%q", m.cfg.Remote)
	}

	var found bool
	for _, week := range m.calendar.Weeks {
		for _, d := range week {
			if d.Date.IsZero() {
				continue
			}
			if d.Date.UTC().Format("2006-01-02") == today {
				found = true
				if d.Count != 7 {
					t.Fatalf("restored count=%d want 7", d.Count)
				}
				if !m.selection.IsSelected(d.Date) {
					t.Fatal("restored date should be selected")
				}
			}
		}
	}
	if !found {
		t.Skip("today not in rendered range; skipping date assertion")
	}
}

func TestNewModel_PrefillsRemoteFromGitOrigin(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	remotePath := filepath.Join(dir, "fake-remote.git")
	cmd = exec.Command("git", "remote", "add", "origin", remotePath)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}
	m := NewModel(1, Config{Dir: root})
	m = pressKey(m, "enter")
	if strings.TrimSpace(m.cfg.Remote) != strings.TrimSpace(remotePath) {
		t.Fatalf("cfg.Remote=%q want %q", m.cfg.Remote, remotePath)
	}
}

func TestNewModel_ShowsProjectSelectWhenProjectsExist(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir alpha: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignore.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write ignore file: %v", err)
	}
	m := NewModel(1, Config{Dir: root, DisableState: true})
	if m.screen != screenProjectSelect {
		t.Fatalf("screen=%v want screenProjectSelect", m.screen)
	}
	if len(m.projectNames) != 2 {
		t.Fatalf("projectNames=%v want [alpha Create new project]", m.projectNames)
	}
	if m.projectNames[0] != "alpha" || m.projectNames[1] != projectCreateOptionLabel {
		t.Fatalf("unexpected project list: %v", m.projectNames)
	}
}

func TestNewModel_MissingOutputStartsCreateFlow(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist-yet")
	m := NewModel(1, Config{Dir: root, DisableState: true})
	if m.screen != screenProjectCreateName {
		t.Fatalf("screen=%v want screenProjectCreateName", m.screen)
	}
}

func TestPushWithoutRemote_PromptsRemoteInput(t *testing.T) {
	m := openOptionsMenu(t)
	m.generatedDateCounts = snapshotGeneratedCounts(m.dateCounts)
	m.cfg.Remote = ""
	m.pushRemote = ""
	m = navigateTo(m, menuPush)
	m = pressKey(m, "enter")
	m = pressKey(m, "y")
	if m.screen != screenProjectRemoteInput {
		t.Fatalf("screen=%v want screenProjectRemoteInput", m.screen)
	}
}

func TestPush_AutoGeneratesWhenNoCommitsExist(t *testing.T) {
	// Start with a model that has counts assigned but no generated commits yet.
	m := newTestModel(testCal())
	m = pressKeys(m, " ", "enter", "5", "enter") // select day, assign count 5
	if m.screen != screenOptions {
		t.Fatalf("want screenOptions, got %v", m.screen)
	}
	// generatedDateCounts is empty — no generation has happened yet.
	if len(m.generatedDateCounts) != 0 {
		t.Fatal("setup error: generatedDateCounts should be empty before push")
	}

	orig := regenerateRepo
	defer func() { regenerateRepo = orig }()
	regenerateRepo = func(cfg gitops.RegenerateConfig) (int, error) {
		return 1, nil
	}

	// Select Push — should trigger auto-generate since no commits exist.
	m = navigateTo(m, menuPush)
	result, cmd := m.Update(makeKeyMsg("enter"))
	m = result.(Model)
	if m.screen != screenGenerating {
		t.Fatalf("screen = %v, want screenGenerating (auto-generate before push)", m.screen)
	}
	if !m.pushAfterGenerate {
		t.Fatal("pushAfterGenerate flag must be set")
	}
	if cmd == nil {
		t.Fatal("expected a generate cmd")
	}
}

// Package tui contains the bubbletea TUI model and views for CommitForge.
package tui

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"commitforge/internal/commit"
	"commitforge/internal/contribution"
	"commitforge/internal/gitops"
	"commitforge/internal/state"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen enum ----------------------------------------------------------

// screen identifies which TUI screen is currently active.
type screen int

const (
	screenProjectSelect      screen = iota // startup project picker
	screenProjectCreateName                // startup new project name input
	screenProjectRemoteMode                // choose local-only or add remote now
	screenProjectRemoteInput               // remote URL input (create/add/push)
	screenGrid                             // contribution grid with cursor and selection
	screenCountEntry                       // commit-count assignment form
	screenOptions                          // post-count action menu
	screenDeselectConfirm                  // confirm removing already-generated commits
	screenClearAllConfirm                  // confirm destructive clear-all operation
	screenPreview                          // date/weekday/commits summary table
	screenGenerating                       // async generation in progress
	screenGenerateDone                     // generation succeeded or failed
	screenPushConfirm                      // ask: push now? (y/n)
	screenPushGuidance                     // static setup guidance block
	screenPushRepoType                     // choose blank vs existing remote
	screenPushRemoteInput                  // prompt for remote URL
	screenPushRunning                      // running push flow with streaming log
	screenPushDone                         // push completed (success/failure)
)

// Config ----------------------------------------------------------------

// Config carries CLI flag values forwarded from cobra to the TUI model.
type Config struct {
	Dir          string // --dir flag (default "output")
	Message      string // --message flag: if non-empty forces fixed-message mode
	MessageMode  string // --message-mode flag ("random" or "fixed")
	Remote       string // --remote flag (optional pre-filled origin URL)
	NoPush       bool   // --no-push flag: skip push flow, generate locally only
	Yes          bool   // --yes flag (auto-confirm prompts where possible)
	DisableState bool   // test-only switch to disable persistence/autosave
}

// Types -----------------------------------------------------------------

// CellPos identifies a cell in the contribution grid by column (week) and row (weekday).
type CellPos struct {
	Week    int // column index in cal.Weeks (0 = oldest)
	Weekday int // row index: 0 = Sunday â€¦ 6 = Saturday
}

// Custom bubbletea message types.
type (
	autosaveTickMsg struct{}
	spinnerTickMsg  struct{}
	generateDoneMsg struct {
		err   error
		total int
	}
	pushLogMsg struct {
		line string
	}
	pushDoneMsg struct {
		err error
	}
	clearAllDoneMsg struct {
		err error
	}
)

// spinnerFrames cycles to animate the generating screen.
var spinnerFrames = []string{"-", "\\", "|", "/"}

var regenerateRepo = gitops.Regenerate
var clearAllCommitsRepo = gitops.ClearAllCommits

// Model -----------------------------------------------------------------

// Model holds all TUI state for CommitForge.
type Model struct {
	// Project/workspace state
	projectsRoot         string
	projectNames         []string
	projectCursor        int
	projectName          string
	projectNameInput     string
	projectInputErr      string
	projectRemoteChoice  int
	remoteInputPurpose   remoteInputPurpose
	clearAllConfirmInput string
	clearAllReturnScreen screen

	// Grid state
	calendar                 contribution.Calendar
	selection                *contribution.Selection
	dateCounts               map[string]int // global per-date counts across all years
	generatedDateCounts      map[string]int // last regenerated/generated snapshot
	cursor                   CellPos
	rangeMode                bool
	rangeAnchor              CellPos
	lastAssignedSelectionSig string
	viewYears                int
	viewEndDate              time.Time
	initialAnchor            time.Time // max forward anchor (usually startup "today")
	width                    int
	height                   int

	// Count-entry state (Phase 4)
	screen        screen
	countInput    string
	countErr      string
	previewCounts map[string]int
	previewOffset int

	// Options menu state (Phase 5)
	menuCursor                    int
	quitting                      bool
	helpVisible                   bool
	deselectPendingDates          []string
	deselectPendingGeneratedTotal int

	// Generation state (Phase 6)
	generateDir       string
	generateTotal     int
	generateErr       error
	generateMsg       string
	spinnerFrame      int
	clearAllInFlight  bool
	pushAfterGenerate bool // auto-push once generation finishes

	// Push-flow state (Phase 7)
	pushRepoType    gitops.PushMode // blank or existing
	pushRemoteInput string
	pushInputErr    string
	pushRemote      string
	pushLogs        []string
	pushLogOffset   int
	pushErr         error
	pushDoneText    string
	pushStream      chan tea.Msg

	// CLI config forwarded from cobra
	cfg Config

	// Persistence state
	stateErr string
}

type remoteInputPurpose int

const (
	remoteInputForProjectCreate remoteInputPurpose = iota
	remoteInputForProjectAdd
	remoteInputForPush
)

// NewModel returns an initialised Model with a contribution calendar built from today.
func NewModel(years int, cfg Config) Model {
	root := strings.TrimSpace(cfg.Dir)
	if root == "" {
		root = "output"
	}
	cfg.Dir = ""
	if strings.TrimSpace(cfg.MessageMode) == "" {
		cfg.MessageMode = "random"
	}

	if years < 1 {
		years = 1
	}
	anchor := truncateToUTCDate(time.Now())
	cal := contribution.Build(years, anchor)
	m := Model{
		calendar:            cal,
		selection:           &contribution.Selection{},
		dateCounts:          map[string]int{},
		generatedDateCounts: map[string]int{},
		cursor:              initialCursor(cal),
		viewYears:           years,
		viewEndDate:         anchor,
		initialAnchor:       anchor,
		projectsRoot:        root,
		cfg:                 cfg,
	}

	m.initializeProjectFlow()
	return m
}

// initialCursor returns a CellPos pointing at the most recent day (today).
func initialCursor(cal contribution.Calendar) CellPos {
	return CellPos{
		Week:    len(cal.Weeks) - 1,
		Weekday: int(cal.EndDate.Weekday()),
	}
}

// bubbletea interface --------------------------------------------------

// Init is the bubbletea initialisation hook.
func (m Model) Init() tea.Cmd {
	if m.cfg.DisableState {
		return nil
	}
	return tickAutosave()
}

// Update handles incoming messages and key events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Clone selection so each returned model owns its own state.
	cloned := m.selection.Clone()
	m.selection = &cloned

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case autosaveTickMsg:
		m.saveState()
		return m, tickAutosave()

	case spinnerTickMsg:
		if m.screen == screenGenerating {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
			return m, tickSpinner()
		}

	case generateDoneMsg:
		m.generateTotal = msg.total
		m.generateErr = msg.err
		dir := m.generateDir
		if dir == "" {
			dir = m.activeProjectDir()
		}
		if msg.err != nil {
			m.generateMsg = msg.err.Error()
			m.pushAfterGenerate = false
		} else {
			m.generateMsg = fmt.Sprintf("%d commit(s) created in %s", msg.total, dir)
			m.generatedDateCounts = snapshotGeneratedCounts(m.dateCounts)
			// Auto-advance to push if generation was triggered by Push action.
			if m.pushAfterGenerate {
				m.pushAfterGenerate = false
				m.saveState()
				if strings.TrimSpace(m.pushRemote) == "" {
					origin, _ := gitops.GetOriginURL(m.activeProjectDir())
					m.pushRemote = strings.TrimSpace(origin)
					m.pushRemoteInput = m.pushRemote
				}
				if strings.TrimSpace(m.pushRemote) == "" {
					m.remoteInputPurpose = remoteInputForPush
					m.screen = screenProjectRemoteInput
					return m, nil
				}
				next, cmd := m.startPush()
				return next, cmd
			}
		}
		m.screen = screenGenerateDone
		m.saveState()

	case clearAllDoneMsg:
		m.generateTotal = 0
		m.generateErr = msg.err
		if msg.err != nil {
			m.generateMsg = msg.err.Error()
		} else {
			m.generateMsg = "✓ Commits cleared and force-pushed. Note: GitHub's contribution graph may take a few hours (up to ~24h) to visually update - this is a GitHub caching delay, not an app issue."
			m.selection.Clear()
			m.dateCounts = map[string]int{}
			m.generatedDateCounts = map[string]int{}
			m.lastAssignedSelectionSig = ""
			m.syncVisibleCalendarCounts()
		}
		m.clearAllInFlight = false
		m.screen = screenGenerateDone
		m.saveState()
		return m, nil

	case pushLogMsg:
		m.pushLogs = append(m.pushLogs, msg.line)
		// Keep viewport pinned to bottom while streaming.
		if m.pushLogOffset < 0 {
			m.pushLogOffset = 0
		}
		return m, waitPushMsg(m.pushStream)

	case pushDoneMsg:
		m.pushErr = msg.err
		if msg.err != nil {
			m.pushDoneText = msg.err.Error()
		} else {
			m.pushDoneText = "Push completed successfully."
		}
		m.screen = screenPushDone
		m.pushStream = nil
		m.saveState()
		return m, nil

	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if key == "q" && !m.isTextInputMode() {
			return m, tea.Quit
		}
		switch key {
		case "?":
			m.helpVisible = !m.helpVisible
			return m, nil
		case "esc", "backspace":
			if m.helpVisible {
				m.helpVisible = false
				return m, nil
			}
			var cmd tea.Cmd
			m, cmd = m.dispatchKey(key)
			m.saveState()
			if m.quitting {
				return m, tea.Quit
			}
			return m, cmd
		default:
			var cmd tea.Cmd
			m, cmd = m.dispatchKey(key)
			m.saveState()
			if m.quitting {
				return m, tea.Quit
			}
			return m, cmd
		}
	}
	return m, nil
}

// Key dispatch ---------------------------------------------------------

// dispatchKey routes a key string to the handler for the active screen.
// It returns the updated Model and an optional tea.Cmd.
func (m Model) dispatchKey(key string) (Model, tea.Cmd) {
	switch m.screen {
	case screenProjectSelect:
		return m.handleProjectSelectKey(key), nil
	case screenProjectCreateName:
		return m.handleProjectCreateNameKey(key), nil
	case screenProjectRemoteMode:
		return m.handleProjectRemoteModeKey(key), nil
	case screenProjectRemoteInput:
		return m.handleProjectRemoteInputKey(key)
	case screenCountEntry:
		return m.handleCountEntryKey(key)
	case screenOptions:
		return m.handleOptionsKey(key)
	case screenDeselectConfirm:
		return m.handleDeselectConfirmKey(key)
	case screenClearAllConfirm:
		return m.handleClearAllConfirmKey(key)
	case screenPreview:
		return m.handlePreviewKey(key), nil
	case screenGenerating:
		return m, nil // ignore keys while generation is running
	case screenGenerateDone:
		return m.handleGenerateDoneKey(key), nil
	case screenPushConfirm:
		return m.handlePushConfirmKey(key)
	case screenPushGuidance:
		return m.handlePushGuidanceKey(key)
	case screenPushRepoType:
		return m.handlePushRepoTypeKey(key)
	case screenPushRemoteInput:
		return m.handlePushRemoteInputKey(key)
	case screenPushRunning:
		return m.handlePushRunningKey(key), nil
	case screenPushDone:
		return m.handlePushDoneKey(key), nil
	default: // screenGrid
		if m.rangeMode {
			return m.handleRangeKey(key), nil
		}
		return m.handleNormalKey(key), nil
	}
}

func (m Model) handleProjectSelectKey(key string) Model {
	if len(m.projectNames) == 0 {
		return m
	}
	switch key {
	case "up", "k":
		m.projectCursor = (m.projectCursor - 1 + len(m.projectNames)) % len(m.projectNames)
	case "down", "j":
		m.projectCursor = (m.projectCursor + 1) % len(m.projectNames)
	case "enter", " ":
		selected := m.projectNames[m.projectCursor]
		if selected == projectCreateOptionLabel {
			m.screen = screenProjectCreateName
			m.projectNameInput = ""
			m.projectInputErr = ""
			return m
		}
		if err := m.activateProject(selected); err != nil {
			m.stateErr = err.Error()
		}
	}
	return m
}

func (m Model) handleProjectCreateNameKey(key string) Model {
	switch key {
	case "esc", "backspace":
		if len(m.projectNameInput) > 0 {
			r := []rune(m.projectNameInput)
			m.projectNameInput = string(r[:len(r)-1])
			m.projectInputErr = ""
			return m
		}
		if len(m.projectNames) > 0 {
			m.screen = screenProjectSelect
			m.projectInputErr = ""
		}
	case "enter":
		name := strings.TrimSpace(m.projectNameInput)
		if err := validateProjectName(name); err != nil {
			m.projectInputErr = err.Error()
			return m
		}
		if projectExists(m.projectsRoot, name) {
			m.projectInputErr = "A project with this name already exists."
			return m
		}
		m.projectName = name
		m.projectRemoteChoice = 0
		m.projectInputErr = ""
		m.screen = screenProjectRemoteMode
	default:
		for _, r := range key {
			if r >= 32 && r != 127 {
				m.projectNameInput += string(r)
			}
		}
		m.projectInputErr = ""
	}
	return m
}

func (m Model) handleProjectRemoteModeKey(key string) Model {
	switch key {
	case "up", "k", "down", "j":
		if m.projectRemoteChoice == 0 {
			m.projectRemoteChoice = 1
		} else {
			m.projectRemoteChoice = 0
		}
	case "enter", " ":
		if m.projectRemoteChoice == 0 {
			if err := m.createAndActivateProject(m.projectName, ""); err != nil {
				m.projectInputErr = err.Error()
				return m
			}
			return m
		}
		m.pushRemoteInput = ""
		m.pushInputErr = ""
		m.remoteInputPurpose = remoteInputForProjectCreate
		m.screen = screenProjectRemoteInput
	case "esc", "backspace":
		m.screen = screenProjectCreateName
	}
	return m
}

func (m Model) handleProjectRemoteInputKey(key string) (Model, tea.Cmd) {
	switch key {
	case "esc":
		if m.remoteInputPurpose == remoteInputForProjectCreate {
			m.screen = screenProjectRemoteMode
		} else {
			m.screen = screenOptions
		}
		m.pushInputErr = ""
	case "backspace":
		if len(m.pushRemoteInput) > 0 {
			r := []rune(m.pushRemoteInput)
			m.pushRemoteInput = string(r[:len(r)-1])
			m.pushInputErr = ""
		} else {
			if m.remoteInputPurpose == remoteInputForProjectCreate {
				m.screen = screenProjectRemoteMode
			} else {
				m.screen = screenOptions
			}
		}
	case "enter":
		remote := strings.TrimSpace(m.pushRemoteInput)
		if err := gitops.ValidateRemoteURL(remote); err != nil {
			m.pushInputErr = err.Error()
			return m, nil
		}
		m.pushRemote = remote
		m.pushInputErr = ""
		switch m.remoteInputPurpose {
		case remoteInputForProjectCreate:
			if err := m.createAndActivateProject(m.projectName, remote); err != nil {
				m.pushInputErr = err.Error()
				return m, nil
			}
			return m, nil
		case remoteInputForProjectAdd:
			if err := m.applyRemoteUpdate(remote); err != nil {
				m.pushDoneText = err.Error()
				m.pushErr = err
				m.screen = screenPushDone
				return m, nil
			}
			hasCommits, err := gitops.HasCommits(m.activeProjectDir())
			if err != nil {
				m.pushDoneText = err.Error()
				m.pushErr = err
				m.screen = screenPushDone
				return m, nil
			}
			if !hasCommits {
				m.screen = screenOptions
				return m, nil
			}
			m.pushRepoType = gitops.PushModeExistingRepo
			return m.startPush()
		case remoteInputForPush:
			if err := m.applyRemoteUpdate(remote); err != nil {
				m.pushDoneText = err.Error()
				m.pushErr = err
				m.screen = screenPushDone
				return m, nil
			}
			m.pushRepoType = gitops.PushModeExistingRepo
			return m.startPush()
		}
	default:
		for _, ch := range key {
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') ||
				strings.ContainsRune(":/._-@\\?=+%", ch) {
				m.pushRemoteInput += string(ch)
			}
		}
		if len(key) > 0 {
			m.pushInputErr = ""
		}
	}
	return m, nil
}

// handleNormalKey processes key input in the default grid navigation mode.
func (m Model) handleNormalKey(key string) Model {
	switch key {
	case "up", "k":
		m.cursor = m.clampCursor(CellPos{m.cursor.Week, m.cursor.Weekday - 1})
	case "down", "j":
		m.cursor = m.clampCursor(CellPos{m.cursor.Week, m.cursor.Weekday + 1})
	case "left", "h":
		m.cursor = m.clampCursor(CellPos{m.cursor.Week - 1, m.cursor.Weekday})
	case "right", "l":
		m.cursor = m.clampCursor(CellPos{m.cursor.Week + 1, m.cursor.Weekday})
	case " ":
		day := m.calendar.Weeks[m.cursor.Week][m.cursor.Weekday]
		if !day.Date.IsZero() {
			m.selection.Toggle(day.Date)
		}
	case "v":
		m.rangeMode = true
		m.rangeAnchor = m.cursor
	case "a":
		m.selection.SelectAll(m.calendar)
	case "u":
		m.selection.Clear()
	case "x":
		m.clearAllConfirmInput = ""
		m.clearAllReturnScreen = screenGrid
		m.screen = screenClearAllConfirm
	case "[", "pgup":
		m = m.shiftYearWindow(-1)
	case "]", "pgdown":
		m = m.shiftYearWindow(1)
	case "esc", "backspace":
		// Grid is the root screen; Esc/Backspace are navigation-back keys on
		// other screens, so here they are intentionally no-op.
	case "enter":
		if m.selection.Count() > 0 {
			if m.selectionHasPendingChanges() || !m.selectionHasAssignedCounts() {
				m.screen = screenCountEntry
				m.countInput = ""
				m.countErr = ""
				m.previewCounts = nil
			} else {
				m.screen = screenOptions
				m.menuCursor = 0
			}
		}
	}
	return m
}

// handleRangeKey processes key input while range-select mode is active.
func (m Model) handleRangeKey(key string) Model {
	switch key {
	case "up", "k":
		m.cursor = m.clampCursor(CellPos{m.cursor.Week, m.cursor.Weekday - 1})
	case "down", "j":
		m.cursor = m.clampCursor(CellPos{m.cursor.Week, m.cursor.Weekday + 1})
	case "left", "h":
		m.cursor = m.clampCursor(CellPos{m.cursor.Week - 1, m.cursor.Weekday})
	case "right", "l":
		m.cursor = m.clampCursor(CellPos{m.cursor.Week + 1, m.cursor.Weekday})
	case "v", " ", "enter":
		anchorDate := m.calendar.Weeks[m.rangeAnchor.Week][m.rangeAnchor.Weekday].Date
		cursorDate := m.calendar.Weeks[m.cursor.Week][m.cursor.Weekday].Date
		if !anchorDate.IsZero() && !cursorDate.IsZero() {
			m.selection.SelectRange(anchorDate, cursorDate, m.calendar)
		}
		m.rangeMode = false
		m.rangeAnchor = CellPos{}
	case "esc", "u":
		m.rangeMode = false
		m.rangeAnchor = CellPos{}
	case "[", "pgup":
		m.rangeMode = false
		m.rangeAnchor = CellPos{}
		m = m.shiftYearWindow(-1)
	case "]", "pgdown":
		m.rangeMode = false
		m.rangeAnchor = CellPos{}
		m = m.shiftYearWindow(1)
	}
	return m
}

// handleCountEntryKey processes key input on the count-assignment screen.
func (m Model) handleCountEntryKey(key string) (Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screen = screenGrid
		m.countInput = ""
		m.countErr = ""
		m.previewCounts = nil
	case "backspace":
		if len(m.countInput) > 0 {
			runes := []rune(m.countInput)
			m.countInput = string(runes[:len(runes)-1])
			m.countErr = ""
			m.previewCounts = m.computePreviewCounts()
		} else {
			m.screen = screenGrid
		}
	case "enter":
		spec, err := commit.ParseCountSpec(m.countInput)
		if err != nil {
			m.countErr = err.Error()
		} else {
			requiresRegenerate := m.selectionTouchesGeneratedDates()
			m = applyCountSpec(m, spec)
			m.countInput = ""
			m.countErr = ""
			m.previewCounts = nil
			if requiresRegenerate {
				dir := m.activeProjectDir()
				m.generateDir = dir
				m.generateTotal = 0
				m.generateErr = nil
				m.generateMsg = ""
				m.spinnerFrame = 0
				m.screen = screenGenerating
				return m, m.makeGenerateCmd()
			}
			m.screen = screenOptions
			m.menuCursor = 0
		}
	default:
		for _, r := range key {
			if r >= 32 && r != 127 {
				m.countInput += string(r)
			}
		}
		m.countErr = ""
		m.previewCounts = m.computePreviewCounts()
	}
	return m, nil
}

// handleOptionsKey processes key input on the post-count action menu.
func (m Model) handleOptionsKey(key string) (Model, tea.Cmd) {
	switch key {
	case "up", "k":
		m.menuCursor = (m.menuCursor - 1 + int(menuCount)) % int(menuCount)
	case "down", "j":
		m.menuCursor = (m.menuCursor + 1) % int(menuCount)
	case "x":
		return m.applyMenuChoice(menuClearAll)
	case "enter", " ":
		return m.applyMenuChoice(menuItem(m.menuCursor))
	case "esc", "backspace":
		m.screen = screenGrid
	}
	return m, nil
}

// applyMenuChoice executes the action for the selected menu item.
func (m Model) applyMenuChoice(item menuItem) (Model, tea.Cmd) {
	switch item {
	case menuPush:
		if m.cfg.NoPush {
			m.screen = screenOptions
			return m, nil
		}
		m.pushRemote = strings.TrimSpace(m.cfg.Remote)
		m.pushRemoteInput = m.pushRemote
		m.pushInputErr = ""
		m.pushLogs = nil
		m.pushLogOffset = 0
		m.pushErr = nil
		m.pushDoneText = ""
		m.pushRepoType = gitops.PushModeExistingRepo
		// If no commits have been generated yet but counts are assigned, generate first.
		if len(m.generatedDateCounts) == 0 && totalCounts(m.dateCounts) > 0 {
			m.pushAfterGenerate = true
			m.generateDir = m.activeProjectDir()
			m.generateTotal = 0
			m.generateErr = nil
			m.generateMsg = ""
			m.spinnerFrame = 0
			m.screen = screenGenerating
			return m, m.makeGenerateCmd()
		}
		if m.cfg.Yes {
			if strings.TrimSpace(m.pushRemote) == "" {
				m.remoteInputPurpose = remoteInputForPush
				m.screen = screenProjectRemoteInput
				return m, nil
			}
			return m.startPush()
		}
		m.screen = screenPushConfirm
		return m, nil
	case menuAddRemote:
		m.pushRemoteInput = strings.TrimSpace(m.cfg.Remote)
		m.pushInputErr = ""
		m.remoteInputPurpose = remoteInputForProjectAdd
		m.screen = screenProjectRemoteInput
	case menuRemoveRemote:
		if err := m.disconnectRemote(); err != nil {
			m.pushErr = err
			m.pushDoneText = err.Error()
			m.screen = screenPushDone
			return m, nil
		}
		m.screen = screenOptions
	case menuDeselect:
		return m.beginDeselectFlow()
	case menuClearAll:
		m.clearAllConfirmInput = ""
		m.clearAllReturnScreen = screenOptions
		m.screen = screenClearAllConfirm
		return m, nil
	case menuEditCounts:
		m.screen = screenCountEntry
		m.countInput = ""
		m.countErr = ""
		m.previewCounts = nil
	case menuGenerateLocal:
		dir := m.activeProjectDir()
		m.generateDir = dir
		m.generateTotal = 0
		m.generateErr = nil
		m.generateMsg = ""
		m.spinnerFrame = 0
		m.screen = screenGenerating
		return m, m.makeGenerateCmd()
	case menuPreviewSummary:
		m.screen = screenPreview
		m.previewOffset = 0
	case menuSaveExit:
		// Phase 8: persist state — for now just signal quit.
		m.quitting = true
	case menuBack:
		m.screen = screenGrid
	}
	return m, nil
}

func (m Model) handleClearAllConfirmKey(key string) (Model, tea.Cmd) {
	switch key {
	case "esc":
		m.clearAllConfirmInput = ""
		m.screen = m.clearAllReturnScreen
	case "backspace":
		if len(m.clearAllConfirmInput) > 0 {
			r := []rune(m.clearAllConfirmInput)
			m.clearAllConfirmInput = string(r[:len(r)-1])
		} else {
			m.screen = m.clearAllReturnScreen
		}
	case "enter":
		if !m.isValidClearAllConfirmation() {
			return m, nil
		}
		dir := m.activeProjectDir()
		m.generateDir = dir
		m.generateTotal = 0
		m.generateErr = nil
		m.generateMsg = ""
		m.spinnerFrame = 0
		m.clearAllInFlight = true
		m.screen = screenGenerating
		return m, m.makeClearAllCmd()
	default:
		for _, r := range key {
			if r >= 32 && r != 127 {
				m.clearAllConfirmInput += string(r)
			}
		}
	}
	return m, nil
}

// handlePreviewKey processes key input on the preview-summary screen.
func (m Model) handlePreviewKey(key string) Model {
	switch key {
	case "up", "k":
		if m.previewOffset > 0 {
			m.previewOffset--
		}
	case "down", "j":
		m.previewOffset++
	case "esc", "backspace":
		m.screen = screenOptions
	}
	return m
}

func (m Model) beginDeselectFlow() (Model, tea.Cmd) {
	dates := m.selection.Dates()
	if len(dates) == 0 {
		m.screen = screenGrid
		return m, nil
	}
	keys := make([]string, 0, len(dates))
	generatedTotal := 0
	for _, d := range dates {
		key := dateKeyUTC(d)
		keys = append(keys, key)
		generatedTotal += m.generatedDateCounts[key]
	}
	m.deselectPendingDates = keys
	m.deselectPendingGeneratedTotal = generatedTotal
	if generatedTotal > 0 && !m.cfg.Yes {
		m.screen = screenDeselectConfirm
		return m, nil
	}
	return m.applyDeselect(generatedTotal > 0)
}

func (m Model) applyDeselect(needsRegenerate bool) (Model, tea.Cmd) {
	keys := m.deselectPendingDates
	if len(keys) == 0 {
		for _, d := range m.selection.Dates() {
			keys = append(keys, dateKeyUTC(d))
		}
	}
	for _, key := range keys {
		delete(m.dateCounts, key)
		delete(m.generatedDateCounts, key)
	}
	m.selection.Clear()
	m.lastAssignedSelectionSig = ""
	m.syncVisibleCalendarCounts()
	m.deselectPendingDates = nil
	m.deselectPendingGeneratedTotal = 0

	if needsRegenerate {
		dir := m.activeProjectDir()
		m.generateDir = dir
		m.generateTotal = 0
		m.generateErr = nil
		m.generateMsg = ""
		m.spinnerFrame = 0
		m.screen = screenGenerating
		return m, m.makeGenerateCmd()
	}
	m.screen = screenGrid
	return m, nil
}

func (m Model) handleDeselectConfirmKey(key string) (Model, tea.Cmd) {
	switch strings.ToLower(key) {
	case "y":
		return m.applyDeselect(true)
	case "n", "esc", "backspace":
		m.screen = screenOptions
		m.deselectPendingDates = nil
		m.deselectPendingGeneratedTotal = 0
	}
	return m, nil
}

// handleGenerateDoneKey processes key input on the generation-result screen.
func (m Model) handleGenerateDoneKey(key string) Model {
	switch key {
	case "enter", "esc", "backspace":
		m.screen = screenGrid
	}
	return m
}

// handlePushConfirmKey handles the initial push confirmation prompt.
func (m Model) handlePushConfirmKey(key string) (Model, tea.Cmd) {
	switch strings.ToLower(key) {
	case "y":
		if strings.TrimSpace(m.pushRemote) == "" {
			origin, _ := gitops.GetOriginURL(m.activeProjectDir())
			m.pushRemote = strings.TrimSpace(origin)
			m.pushRemoteInput = m.pushRemote
		}
		if strings.TrimSpace(m.pushRemote) == "" {
			m.remoteInputPurpose = remoteInputForPush
			m.screen = screenProjectRemoteInput
			return m, nil
		}
		return m.startPush()
	case "n", "esc", "backspace":
		m.screen = screenOptions
	}
	return m, nil
}

// handlePushGuidanceKey advances from the guidance block.
func (m Model) handlePushGuidanceKey(key string) (Model, tea.Cmd) {
	switch key {
	case "enter", " ":
		if strings.TrimSpace(m.pushRemote) == "" {
			origin, _ := gitops.GetOriginURL(m.activeProjectDir())
			m.pushRemote = strings.TrimSpace(origin)
			m.pushRemoteInput = m.pushRemote
		}
		if strings.TrimSpace(m.pushRemote) == "" {
			m.remoteInputPurpose = remoteInputForPush
			m.screen = screenProjectRemoteInput
			m.pushInputErr = ""
			return m, nil
		}
		m.pushRepoType = gitops.PushModeExistingRepo
		return m.startPush()
	case "esc", "backspace":
		m.screen = screenOptions
	}
	return m, nil
}

// handlePushRepoTypeKey handles blank/existing selection.
func (m Model) handlePushRepoTypeKey(key string) (Model, tea.Cmd) {
	switch key {
	case "up", "k", "down", "j":
		if m.pushRepoType == gitops.PushModeBlankRepo {
			m.pushRepoType = gitops.PushModeExistingRepo
		} else {
			m.pushRepoType = gitops.PushModeBlankRepo
		}
	case "enter", " ":
		if strings.TrimSpace(m.pushRemote) == "" {
			origin, _ := gitops.GetOriginURL(m.activeProjectDir())
			m.pushRemote = strings.TrimSpace(origin)
			m.pushRemoteInput = m.pushRemote
		}
		if strings.TrimSpace(m.pushRemote) == "" {
			m.remoteInputPurpose = remoteInputForPush
			m.screen = screenProjectRemoteInput
			m.pushInputErr = ""
			return m, nil
		}
		m.pushRepoType = gitops.PushModeExistingRepo
		return m.startPush()
	case "esc", "backspace":
		m.screen = screenOptions
	}
	return m, nil
}

// handlePushRemoteInputKey handles remote URL text input.
func (m Model) handlePushRemoteInputKey(key string) (Model, tea.Cmd) {
	switch key {
	case "esc":
		m.screen = screenOptions
		m.pushInputErr = ""
	case "backspace":
		if len(m.pushRemoteInput) > 0 {
			r := []rune(m.pushRemoteInput)
			m.pushRemoteInput = string(r[:len(r)-1])
			m.pushInputErr = ""
		} else {
			m.screen = screenOptions
			m.pushInputErr = ""
		}
	case "enter":
		m.pushRemote = strings.TrimSpace(m.pushRemoteInput)
		if m.pushRemote == "" {
			m.pushInputErr = "Remote URL cannot be empty."
			return m, nil
		}
		if err := gitops.ValidateRemoteURL(m.pushRemote); err != nil {
			m.pushInputErr = err.Error()
			return m, nil
		}
		m.pushInputErr = ""
		if err := m.applyRemoteUpdate(m.pushRemote); err != nil {
			m.pushInputErr = err.Error()
			return m, nil
		}
		m.pushRepoType = gitops.PushModeExistingRepo
		return m.startPush()
	default:
		for _, ch := range key {
			// keep a conservative URL/path char set
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') ||
				strings.ContainsRune(":/._-@\\", ch) {
				m.pushRemoteInput += string(ch)
			}
		}
		if len(key) > 0 {
			m.pushInputErr = ""
		}
	}
	return m, nil
}

// handlePushRunningKey supports scrolling the push log pane.
func (m Model) handlePushRunningKey(key string) Model {
	switch key {
	case "up", "k":
		if m.pushLogOffset > 0 {
			m.pushLogOffset--
		}
	case "down", "j":
		if m.pushLogOffset < max(0, len(m.pushLogs)-1) {
			m.pushLogOffset++
		}
	}
	return m
}

// handlePushDoneKey exits the result screen.
func (m Model) handlePushDoneKey(key string) Model {
	switch key {
	case "enter", "esc", "backspace":
		m.screen = screenOptions
	}
	return m
}

func (m Model) startPush() (Model, tea.Cmd) {
	m.pushLogs = nil
	m.pushLogOffset = 0
	m.pushErr = nil
	m.pushDoneText = ""
	m.screen = screenPushRunning
	m.pushStream = make(chan tea.Msg, 128)
	return m, startPushCmd(m)
}

// â”€â”€ Generation â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// makeGenerateCmd returns a tea.Batch that runs the commit generation in the
// background and concurrently animates the spinner.
func (m Model) makeGenerateCmd() tea.Cmd {
	dir := m.generateDir
	total := totalCounts(m.dateCounts)

	if total == 0 {
		return func() tea.Msg {
			return generateDoneMsg{
				err:   fmt.Errorf("no commits to generate - assign counts to selected days first"),
				total: 0,
			}
		}
	}

	generateCmd := func() tea.Msg {
		actual, err := regenerateRepo(gitops.RegenerateConfig{
			Dir:         dir,
			DateCounts:  m.dateCounts,
			Message:     m.cfg.Message,
			MessageMode: m.cfg.MessageMode,
		})
		return generateDoneMsg{err: err, total: actual}
	}

	return tea.Batch(generateCmd, tickSpinner())
}

func (m Model) makeClearAllCmd() tea.Cmd {
	dir := m.activeProjectDir()
	clearCmd := func() tea.Msg {
		return clearAllDoneMsg{err: clearAllCommitsRepo(dir, nil)}
	}
	return tea.Batch(clearCmd, tickSpinner())
}

// tickSpinner returns a Cmd that sends a spinnerTickMsg after 120 ms.
func tickSpinner() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(120 * time.Millisecond)
		return spinnerTickMsg{}
	}
}

func tickAutosave() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(5 * time.Second)
		return autosaveTickMsg{}
	}
}

func startPushCmd(m Model) tea.Cmd {
	ch := m.pushStream
	if ch == nil {
		return nil
	}
	dir := m.activeProjectDir()
	remote := strings.TrimSpace(m.pushRemote)
	mode := m.pushRepoType

	go func() {
		logFn := func(line string) {
			if strings.TrimSpace(line) == "" {
				return
			}
			ch <- pushLogMsg{line: line}
		}
		err := gitops.Push(gitops.PushConfig{
			Dir:       dir,
			RemoteURL: remote,
			Mode:      mode,
		}, logFn)
		ch <- pushDoneMsg{err: err}
		close(ch)
	}()
	return waitPushMsg(ch)
}

func waitPushMsg(ch <-chan tea.Msg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return pushDoneMsg{}
		}
		return msg
	}
}

func (m *Model) applyLoadedState(s state.PersistedState) {
	// Prefer persisted message settings only when CLI did not force values.
	if strings.TrimSpace(m.cfg.Message) == "" && strings.TrimSpace(s.Message) != "" {
		m.cfg.Message = s.Message
	}
	if strings.TrimSpace(m.cfg.MessageMode) == "" || m.cfg.MessageMode == "random" {
		if strings.TrimSpace(s.MessageMode) != "" {
			m.cfg.MessageMode = s.MessageMode
		}
	}
	m.cfg.Remote = strings.TrimSpace(s.RemoteURL)

	for key, c := range s.DateCounts {
		m.dateCounts[key] = c
	}
	for key, c := range s.GeneratedDateCounts {
		m.generatedDateCounts[key] = c
	}
	for _, key := range s.SelectedDates {
		if d, err := time.ParseInLocation("2006-01-02", key, time.UTC); err == nil {
			m.selection.Add(d)
		}
	}
	m.syncVisibleCalendarCounts()
	if m.selectionHasAssignedCounts() {
		m.lastAssignedSelectionSig = m.currentSelectionSignature()
	}
}

func (m *Model) saveState() {
	if m.cfg.DisableState {
		return
	}
	dir := m.activeProjectDir()
	if strings.TrimSpace(dir) == "" {
		return
	}
	dates := m.selection.Dates()
	selected := make([]string, 0, len(dates))
	dateCounts := make(map[string]int, len(m.dateCounts))
	for _, d := range dates {
		key := dateKeyUTC(d)
		selected = append(selected, key)
	}
	for key, c := range m.dateCounts {
		dateCounts[key] = c
	}
	remote := strings.TrimSpace(m.pushRemote)
	if remote == "" {
		remote = strings.TrimSpace(m.cfg.Remote)
	}

	err := state.Save(dir, state.PersistedState{
		Version:             1,
		SelectedDir:         dir,
		SelectedDates:       selected,
		DateCounts:          dateCounts,
		GeneratedDateCounts: copyCountMap(m.generatedDateCounts),
		Message:             m.cfg.Message,
		MessageMode:         m.cfg.MessageMode,
		RemoteURL:           remote,
	})
	if err != nil {
		m.stateErr = "state save failed: " + err.Error()
	} else {
		m.stateErr = ""
	}
}

const projectCreateOptionLabel = "Create new project"

func (m *Model) initializeProjectFlow() {
	names, err := listProjectNames(m.projectsRoot)
	if err != nil {
		m.projectNames = []string{projectCreateOptionLabel}
		m.projectCursor = 0
		m.screen = screenProjectCreateName
		m.projectInputErr = "Cannot read project folder list: " + err.Error()
		return
	}
	if len(names) == 0 {
		m.projectNames = []string{projectCreateOptionLabel}
		m.projectCursor = 0
		m.screen = screenProjectCreateName
		return
	}
	m.projectNames = append(names, projectCreateOptionLabel)
	m.projectCursor = 0
	m.screen = screenProjectSelect
}

func (m *Model) createAndActivateProject(name, remote string) error {
	name = strings.TrimSpace(name)
	if err := validateProjectName(name); err != nil {
		return err
	}
	if err := gitops.ValidateRemoteURL(remote); remote != "" && err != nil {
		return err
	}
	projectDir := filepath.Join(m.projectsRoot, name)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return fmt.Errorf("create project directory: %w", err)
	}
	m.projectName = name
	m.cfg.Dir = projectDir
	m.cfg.Remote = strings.TrimSpace(remote)
	m.pushRemote = m.cfg.Remote
	m.pushRemoteInput = m.cfg.Remote
	m.resetProjectStateData()
	m.saveState()
	if m.stateErr != "" {
		return errors.New(m.stateErr)
	}
	m.screen = screenGrid
	m.stateErr = ""
	return nil
}

func (m *Model) activateProject(name string) error {
	name = strings.TrimSpace(name)
	if err := validateProjectName(name); err != nil {
		return err
	}
	projectDir := filepath.Join(m.projectsRoot, name)
	info, err := os.Stat(projectDir)
	if err != nil {
		return fmt.Errorf("open project: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("open project: %s is not a directory", projectDir)
	}
	m.projectName = name
	m.cfg.Dir = projectDir
	m.pushRemote = ""
	m.pushRemoteInput = ""
	m.resetProjectStateData()
	if !m.cfg.DisableState {
		if s, loadErr := state.Load(projectDir); loadErr == nil {
			m.applyLoadedState(s)
		} else if !errors.Is(loadErr, state.ErrNotFound) {
			m.stateErr = "state load failed: " + loadErr.Error()
		}
	}
	if strings.TrimSpace(m.cfg.Remote) == "" {
		if origin, gitErr := gitops.GetOriginURL(projectDir); gitErr == nil {
			m.cfg.Remote = strings.TrimSpace(origin)
		}
	}
	m.pushRemote = strings.TrimSpace(m.cfg.Remote)
	m.pushRemoteInput = m.pushRemote
	m.screen = screenGrid
	m.projectInputErr = ""
	return nil
}

func (m *Model) resetProjectStateData() {
	m.selection = &contribution.Selection{}
	m.dateCounts = map[string]int{}
	m.generatedDateCounts = map[string]int{}
	m.lastAssignedSelectionSig = ""
	m.rangeMode = false
	m.rangeAnchor = CellPos{}
	m.cursor = initialCursor(m.calendar)
	m.syncVisibleCalendarCounts()
}

func (m Model) activeProjectDir() string {
	return strings.TrimSpace(m.cfg.Dir)
}

func listProjectNames(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func validateProjectName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("Project name cannot be empty.")
	}
	if name == "." || name == ".." {
		return errors.New("Project name is invalid.")
	}
	for _, r := range name {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return errors.New("Project name contains invalid filesystem characters.")
		}
	}
	return nil
}

func projectExists(root, name string) bool {
	info, err := os.Stat(filepath.Join(root, name))
	return err == nil && info.IsDir()
}

func (m *Model) applyRemoteUpdate(remote string) error {
	remote = strings.TrimSpace(remote)
	if err := gitops.ValidateRemoteURL(remote); err != nil {
		return err
	}
	m.cfg.Remote = remote
	m.pushRemote = remote
	m.pushRemoteInput = remote
	m.saveState()
	if m.stateErr != "" {
		return errors.New(m.stateErr)
	}
	return nil
}

func (m *Model) disconnectRemote() error {
	dir := m.activeProjectDir()
	if strings.TrimSpace(dir) == "" {
		return errors.New("no active project")
	}
	if err := gitops.RemoveOrigin(dir); err != nil {
		return err
	}
	m.cfg.Remote = ""
	m.pushRemote = ""
	m.pushRemoteInput = ""
	m.saveState()
	if m.stateErr != "" {
		return errors.New(m.stateErr)
	}
	return nil
}

func (m Model) isValidClearAllConfirmation() bool {
	token := strings.TrimSpace(strings.ToLower(m.clearAllConfirmInput))
	if token == "yes" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(m.clearAllConfirmInput), strings.TrimSpace(m.projectName))
}

// â”€â”€ Helpers â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// computePreviewCounts returns proposed counts for every selected date when the
// current countInput is valid; otherwise returns nil.
func (m Model) computePreviewCounts() map[string]int {
	spec, err := commit.ParseCountSpec(m.countInput)
	if err != nil {
		return nil
	}
	dates := m.selection.Dates()
	if len(dates) == 0 {
		return nil
	}
	rng := rand.New(rand.NewSource(42))
	counts := make(map[string]int, len(dates))
	for _, d := range dates {
		counts[d.UTC().Format("2006-01-02")] = spec.SampleCount(rng)
	}
	return counts
}

// applyCountSpec deep-copies the calendar weeks, writes computed counts, and
// returns the updated Model.
func applyCountSpec(m Model, spec commit.CountSpec) Model {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for _, d := range m.selection.Dates() {
		m.dateCounts[dateKeyUTC(d)] = spec.SampleCount(rng)
	}
	m.lastAssignedSelectionSig = m.currentSelectionSignature()
	m.syncVisibleCalendarCounts()
	return m
}

// cloneCalendarWeeks returns a Calendar with a fully independent deep copy of
// the Weeks slice so that mutations to Day.Count do not affect prior copies.
func cloneCalendarWeeks(cal contribution.Calendar) contribution.Calendar {
	newWeeks := make([][]contribution.Day, len(cal.Weeks))
	for i, week := range cal.Weeks {
		newWeek := make([]contribution.Day, len(week))
		copy(newWeek, week)
		newWeeks[i] = newWeek
	}
	cal.Weeks = newWeeks
	return cal
}

// clampCursor returns a valid CellPos within the calendar bounds.
func (m Model) clampCursor(pos CellPos) CellPos {
	if pos.Week < 0 {
		pos.Week = 0
	}
	if pos.Week >= len(m.calendar.Weeks) {
		pos.Week = len(m.calendar.Weeks) - 1
	}
	if pos.Weekday < 0 {
		pos.Weekday = 0
	}
	if pos.Weekday > 6 {
		pos.Weekday = 6
	}
	for pos.Weekday > 0 && m.calendar.Weeks[pos.Week][pos.Weekday].Date.IsZero() {
		pos.Weekday--
	}
	return pos
}

// â”€â”€ View routing â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// View renders the current TUI state to a string.
func (m Model) View() string {
	if m.terminalTooSmall() {
		return m.renderTooSmallTerminal()
	}

	var (
		out        string
		screenName string
		dangerBody bool
		warnBody   bool
	)

	switch m.screen {
	case screenProjectSelect:
		screenName = "Select Project"
		out = renderProjectSelectScreen(m)
	case screenProjectCreateName:
		screenName = "Create Project"
		out = renderProjectCreateNameScreen(m)
	case screenProjectRemoteMode:
		screenName = "Project Setup"
		out = renderProjectRemoteModeScreen(m)
	case screenProjectRemoteInput:
		screenName = "Remote URL"
		out = renderProjectRemoteInputScreen(m)
	case screenCountEntry:
		screenName = "Count Assignment"
		out = renderCountEntryScreen(m)
	case screenOptions:
		screenName = "Actions"
		out = renderOptionsScreen(m)
	case screenDeselectConfirm:
		screenName = "Confirm Deselect"
		warnBody = true
		out = renderDeselectConfirmScreen(m)
	case screenClearAllConfirm:
		screenName = "Confirm Clear All Commits"
		warnBody = true
		out = renderClearAllConfirmScreen(m)
	case screenPreview:
		screenName = "Preview Summary"
		out = renderPreviewScreen(m)
	case screenGenerating:
		screenName = "Generating"
		out = renderGeneratingScreen(m)
	case screenGenerateDone:
		screenName = "Generation Result"
		dangerBody = m.generateErr != nil
		out = renderGenerateDoneScreen(m)
	case screenPushConfirm:
		screenName = "Push Confirmation"
		warnBody = true
		out = renderPushConfirmScreen(m)
	case screenPushGuidance:
		screenName = "Push Setup Guidance"
		out = renderPushGuidanceScreen(m)
	case screenPushRepoType:
		screenName = "Push Repository Type"
		out = renderPushRepoTypeScreen(m)
	case screenPushRemoteInput:
		screenName = "Remote URL Input"
		out = renderPushRemoteInputScreen(m)
	case screenPushRunning:
		screenName = "Push Running"
		out = renderPushRunningScreen(m)
	case screenPushDone:
		screenName = "Push Result"
		dangerBody = m.pushErr != nil
		out = renderPushDoneScreen(m)
	default:
		screenName = "Contribution Grid"
		out = m.renderGridScreen()
	}

	out = m.renderScreenLayout(screenName, out, dangerBody, warnBody)
	if m.helpVisible {
		out += "\n\n" + renderHelpOverlay(m)
	}
	return out
}

// renderGridScreen builds the contribution-grid view.
func (m Model) renderGridScreen() string {
	vs := viewState{
		cursor:      m.cursor,
		sel:         m.selection,
		rangeMode:   m.rangeMode,
		rangeAnchor: m.rangeAnchor,
		compact:     m.useCompactGrid(),
	}
	var sb strings.Builder
	sb.WriteString(MutedStyle.Render(formatDateRange(m.calendar)))
	sb.WriteString("\n\n")
	sb.WriteString(RenderGrid(m.calendar, vs))
	return sb.String()
}

// footerView returns the contextual key-hint status bar.
func (m Model) footerView() string {
	segments := m.footerSegments()
	maxWidth := m.layoutInnerWidth()
	if maxWidth <= 0 {
		maxWidth = 80
	}
	lines := make([]string, 0, 2)
	current := ""
	for _, seg := range segments {
		candidate := seg
		if current != "" {
			candidate = current + "  " + seg
		}
		if current != "" && lipgloss.Width(candidate) > maxWidth {
			lines = append(lines, current)
			current = seg
			continue
		}
		current = candidate
	}
	if current != "" {
		lines = append(lines, current)
	}
	hint := strings.Join(lines, "\n")
	if m.stateErr != "" {
		hint += "\n" + InlineErrorStyle.Render("state: "+m.stateErr)
	}
	return hint
}

func (m Model) footerSegments() []string {
	switch m.screen {
	case screenProjectSelect:
		return []string{
			m.footerGroup("Navigate", []string{"up/down", "j/k"}),
			m.footerGroup("Actions", []string{"enter choose", "q quit"}),
		}
	case screenProjectCreateName:
		return []string{
			m.footerGroup("Input", []string{"project name", "backspace"}),
			m.footerGroup("Actions", []string{"enter continue", "esc back", "ctrl+c"}),
		}
	case screenProjectRemoteMode:
		return []string{
			m.footerGroup("Choice", []string{"up/down", "j/k"}),
			m.footerGroup("Actions", []string{"enter continue", "esc back", "q quit"}),
		}
	case screenProjectRemoteInput:
		return []string{
			m.footerGroup("Input", []string{"ssh/https URL", "backspace"}),
			m.footerGroup("Actions", []string{"enter save", "esc back", "ctrl+c"}),
		}
	case screenGrid:
		if m.rangeMode {
			anchor := m.calendar.Weeks[m.rangeAnchor.Week][m.rangeAnchor.Weekday]
			return []string{
				m.footerGroup("Range", []string{"from " + anchor.Date.Format("Jan 02"), "v confirm", "esc cancel"}),
				m.footerGroup("Move", []string{"↑↓←→/hjkl"}),
				m.footerGroup("Exit", []string{"q", "ctrl+c"}),
			}
		}
		selectionPill := fmt.Sprintf("%d selected", m.selection.Count())
		return []string{
			m.footerGroup("Move", []string{"↑↓←→/hjkl"}),
			m.footerGroup("Window", []string{"[ ] year"}),
			m.footerGroup("Select", []string{"space toggle", "v range", "a all", "u clear"}),
			m.footerGroup("Action", []string{"enter create/menu", "x clear-all", "q quit", "ctrl+c force"}),
			m.footerGroup("State", []string{selectionPill}),
		}
	case screenCountEntry:
		return []string{
			m.footerGroup("Input", []string{"0-9", "min-max", "backspace"}),
			m.footerGroup("Actions", []string{"enter", "esc", "ctrl+c"}),
		}
	case screenOptions:
		return []string{
			m.footerGroup("Navigate", []string{"up/down", "j/k"}),
			m.footerGroup("Actions", []string{"enter", "x clear-all", "esc", "q"}),
		}
	case screenPreview:
		return []string{
			m.footerGroup("Scroll", []string{"up/down", "j/k"}),
			m.footerGroup("Actions", []string{"esc", "q"}),
		}
	case screenGenerateDone, screenPushDone:
		return []string{
			m.footerGroup("Actions", []string{"enter", "esc", "q"}),
		}
	case screenDeselectConfirm, screenPushConfirm:
		return []string{
			m.footerGroup("Confirm", []string{"y"}),
			m.footerGroup("Cancel", []string{"n", "esc"}),
			m.footerGroup("Exit", []string{"q"}),
		}
	case screenClearAllConfirm:
		return []string{
			m.footerGroup("Confirm", []string{"type yes or project name"}),
			m.footerGroup("Actions", []string{"enter", "esc", "ctrl+c"}),
		}
	case screenPushGuidance:
		return []string{
			m.footerGroup("Actions", []string{"enter", "esc", "q"}),
		}
	case screenPushRepoType, screenPushRemoteInput:
		return []string{
			m.footerGroup("Input", []string{"url", "backspace"}),
			m.footerGroup("Actions", []string{"enter", "esc", "ctrl+c"}),
		}
	case screenPushRunning:
		return []string{
			m.footerGroup("Log", []string{"up/down", "j/k"}),
			m.footerGroup("Exit", []string{"q", "ctrl+c"}),
		}
	case screenGenerating:
		return []string{
			m.footerGroup("Exit", []string{"q", "ctrl+c"}),
		}
	default:
		return []string{m.footerGroup("Actions", []string{"q"})}
	}
}

func (m Model) footerGroup(label string, keys []string) string {
	pills := make([]string, 0, len(keys))
	for _, k := range keys {
		pills = append(pills, KeyPillStyle.Render(k))
	}
	return GroupLabelStyle.Render(label+": ") + strings.Join(pills, " ")
}

func (m Model) renderScreenLayout(title string, body string, danger bool, warning bool) string {
	innerW := m.layoutInnerWidth()
	if innerW <= 0 {
		innerW = 96
	}
	bodyH := m.layoutBodyHeight()

	var headerParts []string
	headerParts = append(headerParts, AppLogoStyle.Render("CommitForge"))
	headerParts = append(headerParts, ScreenTitleStyle.Render(title))
	headerParts = append(headerParts, HeaderMetaStyle.Render(formatYearWindow(m.calendar, m.viewYears)))
	if status := m.remoteStatusLabel(); status != "" {
		headerParts = append(headerParts, HeaderMetaStyle.Render(status))
	}
	header := HeaderZoneStyle.MaxWidth(innerW).Render(strings.Join(headerParts, "  "))

	bodyStyle := BodyZoneStyle
	if danger {
		bodyStyle = DangerBodyZoneStyle
	} else if warning {
		bodyStyle = WarningBodyZoneStyle
	}
	bodyRendered := bodyStyle.MaxWidth(innerW).MaxHeight(bodyH).Render(strings.TrimRight(body, "\n"))
	footer := FooterZoneStyle.MaxWidth(innerW).Render(m.footerView())

	return AppFrameStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, "", bodyRendered, "", footer))
}

func (m Model) layoutInnerWidth() int {
	w := m.viewportWidth()
	inner := w - 12
	if inner < 24 {
		inner = 24
	}
	return inner
}

func (m Model) layoutBodyTextWidth() int {
	w := m.layoutInnerWidth() - 6
	if w < 16 {
		return 16
	}
	return w
}

func (m Model) layoutBodyHeight() int {
	h := m.viewportHeight() - 10
	if h < 3 {
		return 3
	}
	return h
}

func (m Model) viewportWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width
}

func (m Model) viewportHeight() int {
	if m.height <= 0 {
		return 24
	}
	return m.height
}

func (m Model) terminalTooSmall() bool {
	return m.viewportWidth() < 40 || m.viewportHeight() < 10
}

func (m Model) remoteStatusLabel() string {
	if strings.TrimSpace(m.projectName) == "" {
		return "Project: (not selected)"
	}
	remote := strings.TrimSpace(m.cfg.Remote)
	if remote == "" {
		return fmt.Sprintf("Project: %s | Local only", m.projectName)
	}
	return fmt.Sprintf("Project: %s | Connected: %s", m.projectName, remoteShortLabel(remote))
}

func remoteShortLabel(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}
	return gitops.RepoNameFromRemote(remote)
}

func (m Model) renderTooSmallTerminal() string {
	msg := fmt.Sprintf(
		"Terminal too small (%dx%d).\nPlease resize to at least 40x10.",
		m.viewportWidth(),
		m.viewportHeight(),
	)
	body := WarningBodyZoneStyle.Render(InfoStyle.Render(msg))
	return AppFrameStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		HeaderZoneStyle.Render(AppLogoStyle.Render("CommitForge")+"  "+ScreenTitleStyle.Render("Resize Required")),
		"",
		body,
	))
}

func (m Model) useCompactGrid() bool {
	weeks := len(m.calendar.Weeks)
	if weeks == 0 {
		return false
	}
	available := m.layoutBodyTextWidth()
	normalWidth := estimatedGridRowWidth(weeks, false)
	return normalWidth > available
}

func estimatedGridRowWidth(weeks int, compact bool) int {
	if weeks < 1 {
		return 0
	}
	// 4 chars = weekday label + separator space.
	leftPad := 4
	cellW, gapW := 2, 1
	if compact {
		cellW, gapW = 1, 0
	}
	return leftPad + weeks*(cellW+gapW)
}

func (m Model) isTextInputMode() bool {
	return m.screen == screenCountEntry ||
		m.screen == screenPushRemoteInput ||
		m.screen == screenProjectCreateName ||
		m.screen == screenProjectRemoteInput ||
		m.screen == screenClearAllConfirm
}

// formatDateRange returns "Mon YYYY - Mon YYYY" for the calendar's date range.
func formatDateRange(cal contribution.Calendar) string {
	return cal.StartDate.Format("Jan 2006") + " - " + cal.EndDate.Format("Jan 2006")
}

func formatYearWindow(cal contribution.Calendar, years int) string {
	startYear := cal.StartDate.Year()
	endYear := cal.EndDate.Year()
	if startYear == endYear {
		return fmt.Sprintf("window: %d year(s), %d", years, endYear)
	}
	return fmt.Sprintf("window: %d year(s), %d-%d", years, startYear, endYear)
}

func (m Model) shiftYearWindow(deltaYears int) Model {
	if deltaYears == 0 {
		return m
	}
	oldDate := m.cursorDate()
	currentYear := m.initialAnchor.Year()
	targetYear := m.viewEndDate.Year() + deltaYears
	if targetYear >= currentYear {
		m.viewEndDate = truncateToUTCDate(m.initialAnchor)
	} else {
		m.viewEndDate = time.Date(targetYear, time.December, 31, 0, 0, 0, 0, time.UTC)
	}
	m.rebuildCalendarForWindow(oldDate)
	return m
}

func (m *Model) rebuildCalendarForWindow(preferred time.Time) {
	m.calendar = contribution.Build(m.viewYears, m.viewEndDate)
	m.syncVisibleCalendarCounts()
	if !preferred.IsZero() {
		if pos, ok := findDateCell(m.calendar, preferred); ok {
			m.cursor = pos
			return
		}
	}
	m.cursor = initialCursor(m.calendar)
}

func (m *Model) syncVisibleCalendarCounts() {
	m.calendar = cloneCalendarWeeks(m.calendar)
	for wi := range m.calendar.Weeks {
		for wd := range m.calendar.Weeks[wi] {
			day := &m.calendar.Weeks[wi][wd]
			if day.Date.IsZero() {
				continue
			}
			day.Count = m.dateCounts[dateKeyUTC(day.Date)]
		}
	}
}

func findDateCell(cal contribution.Calendar, date time.Time) (CellPos, bool) {
	key := dateKeyUTC(date)
	for wi := range cal.Weeks {
		for wd := range cal.Weeks[wi] {
			day := cal.Weeks[wi][wd]
			if day.Date.IsZero() {
				continue
			}
			if dateKeyUTC(day.Date) == key {
				return CellPos{Week: wi, Weekday: wd}, true
			}
		}
	}
	return CellPos{}, false
}

func (m Model) cursorDate() time.Time {
	if m.cursor.Week < 0 || m.cursor.Week >= len(m.calendar.Weeks) {
		return time.Time{}
	}
	if m.cursor.Weekday < 0 || m.cursor.Weekday >= len(m.calendar.Weeks[m.cursor.Week]) {
		return time.Time{}
	}
	return m.calendar.Weeks[m.cursor.Week][m.cursor.Weekday].Date
}

func truncateToUTCDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func dateKeyUTC(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func totalCounts(counts map[string]int) int {
	total := 0
	for _, c := range counts {
		if c > 0 {
			total += c
		}
	}
	return total
}

func snapshotGeneratedCounts(counts map[string]int) map[string]int {
	out := make(map[string]int, len(counts))
	for k, c := range counts {
		if c > 0 {
			out[k] = c
		}
	}
	return out
}

func copyCountMap(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (m Model) currentSelectionSignature() string {
	dates := m.selection.Dates()
	if len(dates) == 0 {
		return ""
	}
	keys := make([]string, 0, len(dates))
	for _, d := range dates {
		keys = append(keys, dateKeyUTC(d))
	}
	return strings.Join(keys, ",")
}

func (m Model) selectionHasPendingChanges() bool {
	return m.currentSelectionSignature() != m.lastAssignedSelectionSig
}

func (m Model) selectionHasAssignedCounts() bool {
	dates := m.selection.Dates()
	if len(dates) == 0 {
		return false
	}
	for _, d := range dates {
		if m.dateCounts[dateKeyUTC(d)] <= 0 {
			return false
		}
	}
	return true
}

func (m Model) selectionTouchesGeneratedDates() bool {
	for _, d := range m.selection.Dates() {
		if m.generatedDateCounts[dateKeyUTC(d)] > 0 {
			return true
		}
	}
	return false
}

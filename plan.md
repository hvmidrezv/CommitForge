# CommitForge — Development Plan for GitHub Copilot

> Paste this file into Copilot Chat (or reference it via `#file`) one section/phase at a time.
> Keep phases separate in your prompts — this avoids huge context dumps and keeps Copilot's output focused and higher quality (token-efficient workflow).

---

## 0. Project Summary (give this first, always)

Build **CommitForge**, a Go terminal (TUI) application that generates fake Git commits with backdated timestamps to fill in a GitHub-style contribution graph. The user navigates a GitHub-like contribution grid using arrow keys, selects days (single or range), sets a commit count per day (fixed or random), then generates the commits in a local `output/` git repo and optionally pushes them to a remote (new or existing repository).

**Binary name:** `commitforge`
**Entry points:**
- `go run main.go` → launches TUI directly
- `go build -o commitforge && ./commitforge panel` → launches TUI after build
- `commitforge help` / `-h` / `--help` → CLI help
- Additional flags (see §5)

---

## 1. Tech Stack (be explicit with Copilot)

- **Language:** Go 1.22+
- **TUI framework:** `github.com/charmbracelet/bubbletea` (Elm-architecture TUI runtime)
- **Styling:** `github.com/charmbracelet/lipgloss` (colors, borders, layout)
- **Components (optional but recommended):** `github.com/charmbracelet/bubbles` (viewport, help, textinput, spinner)
- **CLI/flags:** `github.com/spf13/cobra` + `github.com/spf13/pflag` (or stdlib `flag` if you want zero deps — decide once, stay consistent)
- **Git operations:** shell out to system `git` via `os/exec` (simplest, most reliable — avoid `go-git` unless a specific feature needs it)
- **Persistence:** local JSON or `bbolt`/`sqlite` — start with JSON file in `output/.commitforge/state.json` (simplest, human-readable, no CGO)
- **Testing:** stdlib `testing` + `testify` (`assert`/`require`) for assertions; `bubbletea`'s built-in test helpers for TUI model updates
- **Docs:** Go doc comments on all exported identifiers + a `docs/` folder + README usage section explaining `go doc`

---

## 2. Project Structure

```
commitforge/
├── main.go                     # entry point, cobra root cmd, launches TUI or subcommands
├── cmd/
│   ├── root.go                 # cobra root command + global flags
│   ├── panel.go                # `commitforge panel` subcommand
│   └── help.go                 # custom help text/examples
├── internal/
│   ├── tui/
│   │   ├── model.go             # bubbletea Model (state, Init/Update/View)
│   │   ├── keymap.go            # keybindings (arrows, vim-like hjkl, space, enter, esc, p, d, ?)
│   │   ├── grid.go               # contribution-grid rendering & selection logic
│   │   ├── styles.go             # lipgloss styles (colors matching GitHub's green scale)
│   │   ├── views_selection.go    # tile selection screen
│   │   ├── views_options.go      # post-selection menu (push/deselect/edit count/etc.)
│   │   ├── views_push.go         # push flow screens (repo type, remote input, guidance)
│   │   └── views_help.go         # in-app help overlay
│   ├── contribution/
│   │   ├── calendar.go           # build a GitHub-like 52x7 week/day grid for N years
│   │   └── selection.go          # single-day / range selection model
│   ├── commit/
│   │   ├── generator.go          # decides commit counts per day (fixed/random) and dates
│   │   └── committer.go          # runs `git commit --date=... --allow-empty -m "..."` etc.
│   ├── gitops/
│   │   ├── repo.go                # git init, remote add, branch -M main, status checks
│   │   └── push.go                # push flow, detect blank vs existing remote, error handling
│   ├── state/
│   │   ├── store.go               # load/save persisted state (selected folder, selections, config)
│   │   └── model.go               # state structs
│   └── config/
│       └── flags.go               # flag definitions shared across cmd/
├── testdata/                     # fixtures for tests
├── docs/
│   └── godoc.md                   # how to generate/read godoc locally
├── README.md                      # English
├── README.fa.md                   # Persian (فارسی)
├── go.mod / go.sum
└── output/                        # DEFAULT working repo (git-ignored from CommitForge's own repo, created at runtime)
```

---

## 3. Core Features Breakdown

### 3.1 Contribution Grid (TUI)
- Render a GitHub-style grid: weeks as columns, days as rows (Sun–Sat), colored squares matching GitHub's 5-level green scale (plus a distinct color for "already generated" days).
- Support at least 1 year back (configurable via flag, default 1, allow up to 5).
- Cursor navigation: `←↑↓→` and `h j k l`.
- `Space` toggles selection of the current tile.
- `v` (or `Shift+arrows`) enters **range-select mode**: pick a start tile, move cursor, confirm end tile → selects the whole contiguous range (by date order, not just grid rectangle).
- `a` selects all, `Esc`/`u` clears/deselects all.
- Legend shown at the bottom (colors → intensity meaning).
- Footer shows contextual key hints (like a status bar).

### 3.2 Commit Count Assignment
After selection is confirmed (`Enter`):
- Prompt: fixed number vs random range.
  - Fixed: user types a number (e.g., `5`) → every selected day gets exactly 5 commits.
  - Random: user provides a min-max range (e.g., `1-8`) → each day gets a random count within that range.
- Apply to: single day, or all days in the current selection/range.
- Show a live preview of intensity color update as counts are assigned (no actual commit yet — just staged in memory/state).

### 3.3 Post-Selection Options Menu
After assigning counts, show an action menu (arrow-key selectable):
- **Push** — proceed to push flow (§3.4)
- **Deselect** — clear current selection, go back to grid
- **Edit counts** — re-open count assignment for current selection
- **Generate locally only** (no push) — just create commits in `output/`
- **Preview summary** — show table: date, commit count, day-of-week
- **Save & exit** — persist state, quit without generating
- **Back** — return to grid without discarding selections

### 3.4 Push Flow
1. Confirm: `Do you want to push now? (y/n)`
2. On `y`, **always show setup guidance first** (static text block, exact commands as provided by GitHub, using the user's chosen repo name/remote once known):
   ```
   …or create a new repository on the command line
   echo "# <repo>" >> README.md
   git init
   git add README.md
   git commit -m "first commit"
   git branch -M main
   git remote add origin git@github.com:<user>/<repo>.git
   git push -u origin main

   …or push an existing repository from the command line
   git remote add origin git@github.com:<user>/<repo>.git
   git branch -M main
   git push -u origin main
   ```
3. Ask: is this a **blank repo** or an **existing repo that already has this project's files**?
   - **Blank repo:** prompt for remote URL (SSH or HTTPS) → app runs the init sequence itself inside `output/` (git init if not already, add remote, rename branch to `main`, push -u origin main), streaming git's real output into the TUI (or a scrollable log pane).
   - **Existing repo (already has files):** prompt for remote URL if not already configured → just `git push` (handle upstream tracking, detect diverged/rejected push and show a clear error + suggested `git pull --rebase` guidance).
4. Handle common failure cases with friendly messages: no SSH key / auth failure, remote already exists, non-fast-forward rejection, no internet.

### 3.5 Folder Selection / Resume
- Flag `--dir <path>` or an in-app "Open existing project" option on startup.
- If `output/` (or chosen dir) already contains a CommitForge state file, load it: restore prior selections/counts/grid, let user continue editing before generating/pushing.
- If the dir has a `.git` already, detect remote and skip re-asking for it (but allow override).

### 3.6 Navigation & Persistence Rules
- Every screen supports `Esc`/`Backspace` to go back **without losing in-memory state** (selections, counts, entered remote URL, etc. persist across screen transitions).
- State is periodically autosaved to `output/.commitforge/state.json` so a crash/quit doesn't lose progress.
- Only actual `git commit`/`git push` actions are irreversible — everything else (selection, counts, screen navigation) is fully re-editable.

### 3.7 In-App Help
- `?` toggles a help overlay anywhere in the TUI listing all active keybindings for the current screen.
- `commitforge help` (CLI) prints full usage, flags, and examples before entering the TUI.

---

## 4. Commit Generation Logic

- For each (date, count) pair: create `count` commits dated on that date.
- Use `git commit --allow-empty --date="YYYY-MM-DDTHH:MM:SS" -m "<message>"` with `GIT_AUTHOR_DATE` and `GIT_COMMITTER_DATE` env vars both set (GitHub graph reads author date, but set both to be safe).
- Spread multiple same-day commits across distinct times (e.g., stagger by a few minutes) so they don't collide.
- Commit messages: small pool of realistic-looking generic messages (configurable via `--message` flag or a `messages.txt` file), or `--message-mode=random|fixed:"text"`.
- All generation happens inside `output/` (or the selected `--dir`), which the app `git init`s if not already a repo.

---

## 5. CLI Flags (document all with examples in both READMEs)

| Flag | Description | Example |
|---|---|---|
| `--dir` | Target output/working directory | `commitforge panel --dir ./my-graph` |
| `--years` | How many years back to render in the grid | `--years 2` |
| `--message` | Fixed commit message | `--message "update"` |
| `--message-mode` | `random` or `fixed` | `--message-mode random` |
| `--remote` | Pre-fill remote URL, skip prompt | `--remote git@github.com:user/repo.git` |
| `--no-push` | Skip push flow entirely, generate only | `--no-push` |
| `--yes` | Auto-confirm prompts (non-interactive-friendly where possible) | `--yes` |
| `-h, --help` | Show help and examples | `commitforge --help` |

---

## 6. Testing Plan

- `internal/contribution`: unit tests for calendar generation (correct week/day layout, leap years, N-years-back boundaries) and range-selection logic (contiguous date ranges from arbitrary start/end).
- `internal/commit`: unit tests for fixed vs random count generation (seeded RNG for determinism), date/time staggering logic.
- `internal/gitops`: tests using a temp dir + real local `git init` (no network) to verify init/add-remote/branch flows; mock/skip actual `push` in unit tests (or use a local bare repo as a fake remote for integration tests).
- `internal/state`: save/load round-trip tests (temp dir, JSON marshal/unmarshal, corrupted-file handling).
- `internal/tui`: use bubbletea's model testing pattern — send `tea.KeyMsg` sequences to `Update()` and assert resulting `Model` state (selection set, screen transitions) without rendering a real terminal.
- Target: every exported function in `internal/` has at least one test; use `testdata/` fixtures for calendar edge cases.
- Add `go test ./... -cover` to README as the standard test command.

---

## 7. Documentation Requirements

- **Code comments:** doc comments on every exported type/func (`// FuncName does X...`), plus inline comments at non-obvious logic (date math, range-selection algorithm, git command construction).
- **README.md (English) and README.fa.md (Persian)** must each include:
  1. What the project does + tech stack used (bubbletea, lipgloss, cobra, git via exec, etc.)
  2. Installation (`go install` / `go build`)
  3. How to run (`go run main.go`, `commitforge panel`)
  4. Full keybinding reference for the TUI
  5. All CLI flags with real example commands (mirror the table in §5)
  6. Step-by-step usage walkthrough (select tiles → set counts → options menu → push flow → new vs existing repo)
  7. Testing instructions (`go test ./...`)
  8. **How to generate and read the Go docs locally**, e.g.:
     ```
     go install golang.org/x/tools/cmd/godoc@latest
     godoc -http=:6060
     # then open http://localhost:6060/pkg/<module-path>/
     ```
     or the modern equivalent: `go doc ./...` and `go doc <package>.<Symbol>`.
  9. Screenshots or ASCII mockup of the grid + menus (optional but nice).

---

## 8. Suggested Build Order (feed to Copilot as separate prompts/phases)

1. **Scaffold:** `go.mod`, folder structure, cobra root + `panel` command, empty bubbletea model that just renders "Hello CommitForge".
2. **Contribution grid:** calendar generation + static rendering (no interactivity yet) with lipgloss color scale.
3. **Navigation & selection:** cursor movement, single-select, range-select, legend, footer hints.
4. **Commit count assignment screen:** fixed/random input, preview coloring.
5. **Options menu:** push / deselect / edit / preview / save-exit / back.
6. **Git generation logic:** `internal/commit` + `internal/gitops` init/commit, generate into `output/`.
7. **Push flow:** guidance screen, blank-vs-existing prompt, remote input, push execution, error handling.
8. **State persistence & folder resume:** `internal/state`, `--dir` flag, autosave, reload on start.
9. **Help system:** `?` overlay + `commitforge help` CLI text.
10. **Tests:** write alongside each phase above (don't defer to the end) — at minimum, backfill after phases 2, 4, 6, 7, 8.
11. **Docs:** README.md + README.fa.md + `docs/godoc.md`, final comment pass for `go vet`/`golint` cleanliness.

> Tip: when prompting Copilot per phase, reference only the relevant section number(s) above instead of pasting the whole plan each time — this keeps each request focused and saves tokens while Copilot still has the full spec available in this file for cross-reference.

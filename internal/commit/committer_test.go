package commit

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

// requireGit skips the test if git is not found in PATH.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH — skipping integration test")
	}
}

// gitSetupIdentity configures a local git user.name / user.email so commits
// succeed even without a global git config (e.g. in CI).
func gitSetupIdentity(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"config", "user.email", "commitforge@example.com"},
		{"config", "user.name", "CommitForge"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}

// gitLog returns the one-line git log for the repo in dir.
func gitLog(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	return string(out)
}

// gitLogFormat returns git log formatted with the given --format string.
func gitLogFormat(t *testing.T, dir, format string) string {
	t.Helper()
	cmd := exec.Command("git", "log", "--format="+format)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log --format: %v", err)
	}
	return string(out)
}

// ── RunCommit ─────────────────────────────────────────────────────────────────

func TestRunCommit_CreatesCommit(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitInit(t, dir)
	gitSetupIdentity(t, dir)

	ts := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	if err := RunCommit(dir, ts, "test commit"); err != nil {
		t.Fatalf("RunCommit: %v", err)
	}
	log := gitLog(t, dir)
	if !strings.Contains(log, "test commit") {
		t.Errorf("git log does not contain 'test commit':\n%s", log)
	}
}

func TestRunCommit_BackdatedTimestamp(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitInit(t, dir)
	gitSetupIdentity(t, dir)

	ts := time.Date(2020, 3, 15, 10, 0, 0, 0, time.UTC)
	if err := RunCommit(dir, ts, "backdated"); err != nil {
		t.Fatalf("RunCommit: %v", err)
	}
	// %ad = author date using %date format
	dateOutput := gitLogFormat(t, dir, "%ad --date=format:%Y-%m-%d")
	if !strings.Contains(dateOutput, "2020-03-15") {
		// Fall back: check ISO format
		isoOutput := gitLogFormat(t, dir, "%aI")
		if !strings.Contains(isoOutput, "2020-03-15") {
			t.Errorf("backdated timestamp not reflected in git log.\nformat output: %q", isoOutput)
		}
	}
}

func TestRunCommit_SetsGitAuthorDate(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitInit(t, dir)
	gitSetupIdentity(t, dir)

	ts := time.Date(2022, 11, 5, 14, 0, 0, 0, time.UTC)
	if err := RunCommit(dir, ts, "date check"); err != nil {
		t.Fatalf("RunCommit: %v", err)
	}
	// %aI = author date in strict ISO 8601 format
	iso := strings.TrimSpace(gitLogFormat(t, dir, "%aI"))
	if !strings.HasPrefix(iso, "2022-11-05") {
		t.Errorf("author date = %q, want prefix 2022-11-05", iso)
	}
}

// ── GenerateCommits ───────────────────────────────────────────────────────────

func TestGenerateCommits_CreatesAllCommits(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitInit(t, dir)
	gitSetupIdentity(t, dir)

	jobs := []CommitJob{
		{Timestamp: time.Date(2024, 6, 1, 9, 0, 0, 0, time.UTC), Message: "alpha"},
		{Timestamp: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC), Message: "beta"},
		{Timestamp: time.Date(2024, 6, 1, 11, 0, 0, 0, time.UTC), Message: "gamma"},
	}

	if err := GenerateCommits(dir, jobs, nil); err != nil {
		t.Fatalf("GenerateCommits: %v", err)
	}
	log := gitLog(t, dir)
	lines := strings.Split(strings.TrimSpace(log), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d commits, want 3\n%s", len(lines), log)
	}
}

func TestGenerateCommits_ReportsProgress(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitInit(t, dir)
	gitSetupIdentity(t, dir)

	jobs := []CommitJob{
		{Timestamp: time.Date(2024, 6, 1, 9, 0, 0, 0, time.UTC), Message: "one"},
		{Timestamp: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC), Message: "two"},
	}

	var progressLog [][2]int
	err := GenerateCommits(dir, jobs, func(done, total int) {
		progressLog = append(progressLog, [2]int{done, total})
	})
	if err != nil {
		t.Fatalf("GenerateCommits: %v", err)
	}
	if len(progressLog) != 2 {
		t.Fatalf("got %d progress calls, want 2", len(progressLog))
	}
	if progressLog[0] != [2]int{1, 2} {
		t.Errorf("progress[0] = %v, want [1 2]", progressLog[0])
	}
	if progressLog[1] != [2]int{2, 2} {
		t.Errorf("progress[1] = %v, want [2 2]", progressLog[1])
	}
}

func TestGenerateCommits_IntegrationWithStagger(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitInit(t, dir)
	gitSetupIdentity(t, dir)

	// Build jobs via StaggerJobs for 2024-06-01, 3 commits.
	date := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	jobs := StaggerJobs(date, 3, []string{"stagger test"}, nil)
	if len(jobs) != 3 {
		t.Fatalf("StaggerJobs returned %d jobs", len(jobs))
	}

	if err := GenerateCommits(dir, jobs, nil); err != nil {
		t.Fatalf("GenerateCommits: %v", err)
	}
	log := gitLog(t, dir)
	lines := strings.Split(strings.TrimSpace(log), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d commits, want 3\n%s", len(lines), log)
	}
	// All must have the right date prefix.
	dates := gitLogFormat(t, dir, "%aI")
	for _, line := range strings.Split(strings.TrimSpace(dates), "\n") {
		if !strings.HasPrefix(line, "2024-06-01") {
			t.Errorf("commit date = %q, want prefix 2024-06-01", line)
		}
	}
}

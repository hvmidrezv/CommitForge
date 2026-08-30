// Package commit handles commit count generation and backdated git commit execution.
package commit

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RunCommit creates a single backdated empty commit in dir.
// Both GIT_AUTHOR_DATE and GIT_COMMITTER_DATE are set to ts in RFC3339 format,
// which causes the commit to appear on the GitHub contribution graph for that date.
func RunCommit(dir string, ts time.Time, message string) error {
	dateStr := ts.UTC().Format(time.RFC3339)
	// Keep --date and both env vars aligned so git records the intended
	// timestamp consistently for both author and committer metadata.
	cmd := exec.Command("git", "commit", "--allow-empty", "--date="+dateStr, "-m", message)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+dateStr,
		"GIT_COMMITTER_DATE="+dateStr,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// GenerateCommits runs every job in sequence, calling RunCommit for each.
// After each successful commit it calls progressFn(done, total) if non-nil.
// Returns the first error encountered.
func GenerateCommits(dir string, jobs []CommitJob, progressFn func(done, total int)) error {
	total := len(jobs)
	for i, job := range jobs {
		if err := RunCommit(dir, job.Timestamp, job.Message); err != nil {
			return fmt.Errorf("job %d/%d: %w", i+1, total, err)
		}
		if progressFn != nil {
			progressFn(i+1, total)
		}
	}
	return nil
}

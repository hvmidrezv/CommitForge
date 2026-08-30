// Package gitops manages git repository initialisation and push operations.
package gitops

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ClearAllCommits rewrites repository history to a single empty commit and
// force-pushes it to origin/main.
func ClearAllCommits(dir string, stream StreamFn) error {
	if strings.TrimSpace(dir) == "" {
		dir = "output"
	}
	if err := Init(dir); err != nil {
		return err
	}

	origin, err := GetOriginURL(dir)
	if err != nil {
		return err
	}
	if strings.TrimSpace(origin) == "" {
		return &PushError{
			Op:     "clear-all preflight",
			Err:    errors.New("missing remote URL"),
			Output: "No origin remote configured. Configure a remote and push once before using clear-all.",
		}
	}

	branch := "commitforge-clear"
	if err := runGitCommand(dir, stream, "git checkout --orphan commitforge-clear", "git", "checkout", "--orphan", branch); err != nil {
		return friendly(err)
	}
	// Clearing tracked files can fail on empty repos; treat that specific case as non-fatal.
	if err := runGitCommand(dir, stream, "git rm -rf .", "git", "rm", "-rf", "."); err != nil {
		pe := asPushError(err)
		raw := strings.ToLower(pe.Output)
		if !strings.Contains(raw, "did not match any files") && !strings.Contains(raw, "pathspec") {
			return friendly(err)
		}
	}

	if err := createEmptyCommit(dir, "commitforge: clear all commits"); err != nil {
		return friendly(err)
	}
	if err := runGitCommand(dir, stream, "git branch -M main", "git", "branch", "-M", "main"); err != nil {
		return friendly(err)
	}
	if err := runGitCommand(dir, stream, "git push --force -u origin main", "git", "push", "--force", "-u", "origin", "main"); err != nil {
		return friendly(err)
	}
	return nil
}

func createEmptyCommit(dir, message string) error {
	ts := time.Now().UTC().Format(time.RFC3339)
	cmd := exec.Command("git", "commit", "--allow-empty", "--date="+ts, "-m", message)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+ts,
		"GIT_COMMITTER_DATE="+ts,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &PushError{
			Op:     "git commit --allow-empty",
			Err:    err,
			Output: strings.TrimSpace(string(out)),
		}
	}
	return nil
}

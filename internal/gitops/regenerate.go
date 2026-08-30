// Package gitops manages git repository initialisation and push operations.
package gitops

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"commitforge/internal/commit"
	"commitforge/internal/state"
)

// ErrUnsafeRegenerate is returned when directory safety checks fail.
var ErrUnsafeRegenerate = errors.New("unsafe regenerate target")

// RegenerateConfig defines inputs for rebuilding repo history from state.
type RegenerateConfig struct {
	Dir         string
	DateCounts  map[string]int
	Message     string
	MessageMode string
}

// Regenerate rebuilds repository history so commits match DateCounts exactly.
// It refuses to run unless <dir>/.commitforge/state.json exists.
func Regenerate(cfg RegenerateConfig) (int, error) {
	dir := strings.TrimSpace(cfg.Dir)
	if dir == "" {
		dir = "output"
	}
	if err := ensureCommitForgeMarker(dir); err != nil {
		return 0, err
	}

	remote, err := GetOriginURL(dir)
	if err != nil {
		// A target folder can be CommitForge-managed but not yet git-initialized.
		// In that case there is no remote to preserve, so regenerate can proceed.
		if !isNotGitRepoErr(err) {
			return 0, err
		}
		remote = ""
	}
	if err := resetRepository(dir); err != nil {
		return 0, err
	}
	if strings.TrimSpace(remote) != "" {
		if err := runGitNoStream(dir, "git remote add origin", "git", "remote", "add", "origin", remote); err != nil {
			return 0, err
		}
	}

	ensureLocalIdentity(dir)

	jobs, total, err := buildJobsFromState(cfg)
	if err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, nil
	}
	if err := commit.GenerateCommits(dir, jobs, nil); err != nil {
		return 0, err
	}
	return total, nil
}

func ensureCommitForgeMarker(dir string) error {
	path := state.FilePath(dir)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s does not contain %s", ErrUnsafeRegenerate, dir, filepath.FromSlash(".commitforge/state.json"))
		}
		return fmt.Errorf("checking regenerate marker: %w", err)
	}
	return nil
}

func resetRepository(dir string) error {
	gitDir := filepath.Join(dir, ".git")
	if err := os.RemoveAll(gitDir); err != nil {
		return fmt.Errorf("remove git history: %w", err)
	}
	if err := Init(dir); err != nil {
		return err
	}
	return nil
}

func runGitNoStream(dir, op string, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", op, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ensureLocalIdentity guarantees git has a usable author identity for the
// repository in dir.  It only writes local config values when the corresponding
// global setting is absent; a real global identity is never overridden so that
// commits are attributed to the user's actual GitHub-verified email and appear
// on the contribution graph.
func ensureLocalIdentity(dir string) {
	if globalGitConfig("user.email") == "" {
		_ = runGitNoStream(dir, "git config user.email", "git", "config", "user.email", "commitforge@example.com")
	}
	if globalGitConfig("user.name") == "" {
		_ = runGitNoStream(dir, "git config user.name", "git", "config", "user.name", "CommitForge")
	}
}

// globalGitConfig returns the value of a global git config key, or "" if unset.
func globalGitConfig(key string) string {
	cmd := exec.Command("git", "config", "--global", key)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func buildJobsFromState(cfg RegenerateConfig) ([]commit.CommitJob, int, error) {
	keys := make([]string, 0, len(cfg.DateCounts))
	total := 0
	for k, c := range cfg.DateCounts {
		if c <= 0 {
			continue
		}
		keys = append(keys, k)
		total += c
	}
	sort.Strings(keys)

	var messages []string
	if strings.TrimSpace(cfg.Message) != "" {
		messages = []string{cfg.Message}
	} else if strings.TrimSpace(cfg.MessageMode) == "fixed" {
		messages = []string{"update"}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	jobs := make([]commit.CommitJob, 0, total)
	for _, key := range keys {
		d, err := time.ParseInLocation("2006-01-02", key, time.UTC)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid state date key %q: %w", key, err)
		}
		jobs = append(jobs, commit.StaggerJobs(d, cfg.DateCounts[key], messages, rng)...)
	}
	return jobs, total, nil
}

func isNotGitRepoErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "not a git repository")
}

package gitops

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestClearAllCommits_NoRemoteConfigured(t *testing.T) {
	requireGitForPush(t)
	tmp := t.TempDir()
	work := filepath.Join(tmp, "work")
	setupRepoWithOneCommit(t, work)

	err := ClearAllCommits(work, nil)
	if err == nil {
		t.Fatal("expected missing remote error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "no origin remote configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClearAllCommits_RewritesAndForcePushes(t *testing.T) {
	requireGitForPush(t)
	tmp := t.TempDir()
	work := filepath.Join(tmp, "work")
	remote := filepath.Join(tmp, "remote.git")

	runGit(t, tmp, "init", "--bare", remote)
	setupRepoWithOneCommit(t, work)
	runGit(t, work, "remote", "add", "origin", remote)
	runGit(t, work, "branch", "-M", "main")
	runGit(t, work, "push", "-u", "origin", "main")
	runGit(t, work, "commit", "--allow-empty", "-m", "second")
	runGit(t, work, "push")

	err := ClearAllCommits(work, nil)
	if err != nil {
		t.Fatalf("ClearAllCommits: %v", err)
	}

	// Remote should now have exactly one commit on main.
	count := runGit(t, tmp, "--git-dir", remote, "rev-list", "--count", "refs/heads/main")
	if strings.TrimSpace(count) != "1" {
		t.Fatalf("remote commit count = %s, want 1", count)
	}
	msg := runGit(t, work, "log", "-1", "--pretty=%s")
	if !strings.Contains(strings.ToLower(msg), "clear all commits") {
		t.Fatalf("unexpected clear commit message: %q", msg)
	}
}

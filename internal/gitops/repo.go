// Package gitops manages git repository initialisation and push operations.
package gitops

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Init ensures dir exists and contains a git repository.
// It creates dir (and any missing parents) if needed, then runs `git init`.
// Calling Init on an already-initialised repository is safe and idempotent.
func Init(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveOrigin removes the origin remote if present.
func RemoveOrigin(dir string) error {
	cmd := exec.Command("git", "remote", "remove", "origin")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := strings.ToLower(strings.TrimSpace(string(out)))
		if strings.Contains(s, "no such remote") || strings.Contains(s, "not a git repository") {
			return nil
		}
		return fmt.Errorf("git remote remove origin: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

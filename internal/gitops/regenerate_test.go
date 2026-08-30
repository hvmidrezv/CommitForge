package gitops

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"commitforge/internal/state"
)

func TestRegenerate_RefusesWithoutMarker(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	dir := t.TempDir()
	_, err := Regenerate(RegenerateConfig{
		Dir:        dir,
		DateCounts: map[string]int{"2024-06-10": 2},
	})
	if err == nil {
		t.Fatal("expected safety refusal error")
	}
	if !errors.Is(err, ErrUnsafeRegenerate) {
		t.Fatalf("expected ErrUnsafeRegenerate, got %v", err)
	}
}

func TestRegenerate_ReplaysExactlyFromStateMap(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	dir := t.TempDir()
	if err := state.Save(dir, state.PersistedState{
		Version:     1,
		SelectedDir: dir,
		DateCounts: map[string]int{
			"2024-06-10": 2,
			"2024-06-11": 3,
		},
	}); err != nil {
		t.Fatalf("state.Save: %v", err)
	}
	total, err := Regenerate(RegenerateConfig{
		Dir:        dir,
		DateCounts: map[string]int{"2024-06-10": 2, "2024-06-11": 3},
	})
	if err != nil {
		t.Fatalf("Regenerate: %v", err)
	}
	if total != 5 {
		t.Fatalf("total=%d want 5", total)
	}

	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-list: %v\n%s", err, out)
	}
	got, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("parse rev-list count: %v", err)
	}
	if got != 5 {
		t.Fatalf("repo commit count=%d want 5", got)
	}
}

func TestRegenerate_PreservesRemoteWhenPresent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "repo")
	remote := filepath.Join(tmp, "remote.git")
	runGit(t, tmp, "init", "--bare", remote)
	if err := state.Save(dir, state.PersistedState{
		Version:     1,
		SelectedDir: dir,
		DateCounts:  map[string]int{"2024-06-10": 1},
	}); err != nil {
		t.Fatalf("state.Save: %v", err)
	}
	if err := Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	runGit(t, dir, "remote", "add", "origin", remote)

	if _, err := Regenerate(RegenerateConfig{
		Dir:        dir,
		DateCounts: map[string]int{"2024-06-10": 1},
	}); err != nil {
		t.Fatalf("Regenerate: %v", err)
	}
	got, err := GetOriginURL(dir)
	if err != nil {
		t.Fatalf("GetOriginURL: %v", err)
	}
	if strings.TrimSpace(got) != strings.TrimSpace(remote) {
		t.Fatalf("origin=%q want %q", got, remote)
	}
}

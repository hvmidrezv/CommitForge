package gitops

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// requireGit skips the test if git is not found in PATH.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH — skipping integration test")
	}
}

func TestInit_CreatesDirectoryAndGitRepo(t *testing.T) {
	requireGit(t)
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "myrepo")

	if err := Init(targetDir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, ".git")); os.IsNotExist(err) {
		t.Error(".git not found after Init in non-existent directory")
	}
}

func TestInit_ExistingDirectory(t *testing.T) {
	requireGit(t)
	tmpDir := t.TempDir() // already exists

	if err := Init(tmpDir); err != nil {
		t.Fatalf("Init on existing dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".git")); os.IsNotExist(err) {
		t.Error(".git not found after Init in existing directory")
	}
}

func TestInit_Idempotent(t *testing.T) {
	requireGit(t)
	tmpDir := t.TempDir()

	if err := Init(tmpDir); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	if err := Init(tmpDir); err != nil {
		t.Fatalf("second Init (idempotent): %v", err)
	}
}

func TestInit_NestedDirectories(t *testing.T) {
	requireGit(t)
	tmpDir := t.TempDir()
	nested := filepath.Join(tmpDir, "a", "b", "c")

	if err := Init(nested); err != nil {
		t.Fatalf("Init nested: %v", err)
	}
	if _, err := os.Stat(filepath.Join(nested, ".git")); os.IsNotExist(err) {
		t.Error(".git not found after Init in nested directory")
	}
}

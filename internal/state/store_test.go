package state

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := PersistedState{
		Version:       1,
		SelectedDir:   dir,
		SelectedDates: []string{"2024-06-10", "2024-06-11"},
		DateCounts: map[string]int{
			"2024-06-10": 5,
			"2024-06-11": 2,
		},
		GeneratedDateCounts: map[string]int{
			"2024-06-10": 5,
		},
		Message:     "update",
		MessageMode: "random",
		RemoteURL:   "git@github.com:user/repo.git",
	}
	if err := Save(dir, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.SelectedDir != in.SelectedDir {
		t.Fatalf("SelectedDir mismatch: got %q want %q", out.SelectedDir, in.SelectedDir)
	}
	if len(out.SelectedDates) != 2 {
		t.Fatalf("SelectedDates len=%d want 2", len(out.SelectedDates))
	}
	if out.DateCounts["2024-06-10"] != 5 || out.DateCounts["2024-06-11"] != 2 {
		t.Fatalf("DateCounts mismatch: %+v", out.DateCounts)
	}
	if out.GeneratedDateCounts["2024-06-10"] != 5 {
		t.Fatalf("GeneratedDateCounts mismatch: %+v", out.GeneratedDateCounts)
	}
	if out.Message != in.Message || out.MessageMode != in.MessageMode || out.RemoteURL != in.RemoteURL {
		t.Fatalf("metadata mismatch: got %+v", out)
	}
}

func TestLoad_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestLoad_CorruptedFile(t *testing.T) {
	dir := t.TempDir()
	path := FilePath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{ this is not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected corrupted error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "corrupted") {
		t.Fatalf("expected corrupted error message, got: %v", err)
	}
}

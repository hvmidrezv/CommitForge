package gitops

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func requireGitForPush(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func setupRepoWithOneCommit(t *testing.T, dir string) {
	t.Helper()
	if err := Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	runGit(t, dir, "config", "user.email", "push-test@example.com")
	runGit(t, dir, "config", "user.name", "Push Test")
	ts := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	cmd := exec.Command("git", "commit", "--allow-empty", "--date="+ts, "-m", "initial")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+ts,
		"GIT_COMMITTER_DATE="+ts,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

func TestSetupGuidance_UsesRemote(t *testing.T) {
	got := SetupGuidance("git@github.com:alice/demo.git")
	if !strings.Contains(got, "echo \"# demo\" >> README.md") {
		t.Fatalf("guidance missing repo substitution:\n%s", got)
	}
	if !strings.Contains(got, "git remote add origin git@github.com:alice/demo.git") {
		t.Fatalf("guidance missing remote substitution:\n%s", got)
	}
}

func TestPush_BlankRepo_FullSequence_LocalBareRemote(t *testing.T) {
	requireGitForPush(t)
	tmp := t.TempDir()
	work := filepath.Join(tmp, "work")
	remote := filepath.Join(tmp, "remote.git")

	runGit(t, tmp, "init", "--bare", remote)
	setupRepoWithOneCommit(t, work)

	var logs []string
	err := Push(PushConfig{
		Dir:       work,
		RemoteURL: remote,
		Mode:      PushModeBlankRepo,
	}, func(line string) { logs = append(logs, line) })
	if err != nil {
		t.Fatalf("Push blank: %v", err)
	}

	// Verify remote branch exists.
	out := runGit(t, tmp, "--git-dir", remote, "show-ref", "--verify", "refs/heads/main")
	if out == "" {
		t.Fatal("refs/heads/main not found in bare remote")
	}
	if len(logs) == 0 {
		t.Fatal("expected streaming output lines, got none")
	}
}

func TestPush_NoCommits_ReturnsInternalErrorWithoutRunningPush(t *testing.T) {
	requireGitForPush(t)
	tmp := t.TempDir()
	work := filepath.Join(tmp, "work")
	if err := Init(work); err != nil {
		t.Fatalf("Init: %v", err)
	}

	origRunner := runGitCommand
	defer func() { runGitCommand = origRunner }()

	pushCalls := 0
	runGitCommand = func(dir string, stream StreamFn, op string, args ...string) error {
		if len(args) >= 2 && args[0] == "git" && args[1] == "push" {
			pushCalls++
		}
		return origRunner(dir, stream, op, args...)
	}

	err := Push(PushConfig{
		Dir:       work,
		RemoteURL: filepath.Join(tmp, "remote.git"),
		Mode:      PushModeBlankRepo,
	}, nil)
	if err == nil {
		t.Fatal("expected no-commits preflight error")
	}
	if !strings.Contains(err.Error(), "No commits to push") {
		t.Fatalf("expected no-commits error, got: %v", err)
	}
	if pushCalls != 0 {
		t.Fatalf("git push should not be invoked when repo has no commits; got %d calls", pushCalls)
	}
}

func TestPush_ExistingRepo_UsesOriginWhenRemoteNotProvided(t *testing.T) {
	requireGitForPush(t)
	tmp := t.TempDir()
	work := filepath.Join(tmp, "work")
	remote := filepath.Join(tmp, "remote.git")

	runGit(t, tmp, "init", "--bare", remote)
	setupRepoWithOneCommit(t, work)
	runGit(t, work, "remote", "add", "origin", remote)
	runGit(t, work, "branch", "-M", "main")

	err := Push(PushConfig{
		Dir:  work,
		Mode: PushModeExistingRepo,
	}, nil)
	if err != nil {
		t.Fatalf("Push existing: %v", err)
	}
	out := runGit(t, tmp, "--git-dir", remote, "show-ref", "--verify", "refs/heads/main")
	if out == "" {
		t.Fatal("refs/heads/main not found in bare remote")
	}
}

func TestPush_ExistingRepo_RenamesBranchToMainBeforePush(t *testing.T) {
	requireGitForPush(t)
	tmp := t.TempDir()
	work := filepath.Join(tmp, "work")
	remote := filepath.Join(tmp, "remote.git")

	runGit(t, tmp, "init", "--bare", remote)
	setupRepoWithOneCommit(t, work)
	runGit(t, work, "branch", "-M", "feature-x")

	err := Push(PushConfig{
		Dir:       work,
		RemoteURL: remote,
		Mode:      PushModeExistingRepo,
	}, nil)
	if err != nil {
		t.Fatalf("Push existing with branch rename: %v", err)
	}

	branch := runGit(t, work, "rev-parse", "--abbrev-ref", "HEAD")
	if branch != "main" {
		t.Fatalf("current branch = %q, want main", branch)
	}

	out := runGit(t, tmp, "--git-dir", remote, "show-ref", "--verify", "refs/heads/main")
	if out == "" {
		t.Fatal("refs/heads/main not found in bare remote")
	}
}

func TestPush_NonFastForward_AutoForcePushes(t *testing.T) {
	requireGitForPush(t)
	tmp := t.TempDir()
	remote := filepath.Join(tmp, "remote.git")
	a := filepath.Join(tmp, "a")
	b := filepath.Join(tmp, "b")

	runGit(t, tmp, "init", "--bare", remote)

	// Repo A pushes first commit.
	setupRepoWithOneCommit(t, a)
	runGit(t, a, "remote", "add", "origin", remote)
	runGit(t, a, "branch", "-M", "main")
	runGit(t, a, "push", "-u", "origin", "main")

	// Repo B clones and pushes another commit, causing divergence.
	runGit(t, tmp, "clone", "-b", "main", remote, b)
	runGit(t, b, "config", "user.email", "push-test@example.com")
	runGit(t, b, "config", "user.name", "Push Test")
	runGit(t, b, "commit", "--allow-empty", "-m", "from-b")
	runGit(t, b, "push")

	// Repo A has a diverged local commit — CommitForge must auto-force-push to recover.
	runGit(t, a, "commit", "--allow-empty", "-m", "from-a")
	var logs []string
	err := Push(PushConfig{Dir: a, Mode: PushModeExistingRepo}, func(l string) { logs = append(logs, l) })
	if err != nil {
		t.Fatalf("expected auto-force-push to succeed, got: %v", err)
	}
	// Remote's main branch should now have repo A's commit (force-push overwrote B's divergence).
	head := runGit(t, tmp, "--git-dir", remote, "log", "-1", "--pretty=%s", "refs/heads/main")
	if !strings.Contains(head, "from-a") {
		t.Fatalf("remote HEAD = %q, want commit from repo A after force push", head)
	}
	// A notice about force-push should have been streamed.
	joined := strings.Join(logs, "\n")
	if !strings.Contains(strings.ToLower(joined), "force") {
		t.Fatalf("expected force-push notice in log output, got: %s", joined)
	}
}

func TestFriendlyErrorMessage_CommonPatterns(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Permission denied (publickey).", "Authentication failed"},
		{"fatal: remote origin already exists.", "Remote 'origin' already exists"},
		{"fatal: Could not resolve host: github.com", "Network error"},
	}
	for _, tc := range cases {
		got := FriendlyErrorMessage(tc.in)
		if !strings.Contains(got, tc.want) {
			t.Fatalf("FriendlyErrorMessage(%q)=%q want contains %q", tc.in, got, tc.want)
		}
	}
}

func TestGetOriginURL(t *testing.T) {
	requireGitForPush(t)
	tmp := t.TempDir()
	if err := Init(tmp); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// No origin yet.
	got, err := GetOriginURL(tmp)
	if err != nil {
		t.Fatalf("GetOriginURL no origin: %v", err)
	}
	if got != "" {
		t.Fatalf("GetOriginURL=%q want empty", got)
	}
	remote := filepath.Join(tmp, "remote.git")
	runGit(t, tmp, "remote", "add", "origin", remote)
	got, err = GetOriginURL(tmp)
	if err != nil {
		t.Fatalf("GetOriginURL with origin: %v", err)
	}
	if strings.TrimSpace(got) != strings.TrimSpace(remote) {
		t.Fatalf("GetOriginURL=%q want %q", got, remote)
	}
}

func TestRepoNameFromRemote(t *testing.T) {
	cases := map[string]string{
		"git@github.com:alice/demo.git":     "demo",
		"https://github.com/alice/demo.git": "demo",
		"C:\\repos\\demo.git":               "demo",
		"demo":                              "demo",
	}
	for in, want := range cases {
		if got := RepoNameFromRemote(in); got != want {
			t.Fatalf("RepoNameFromRemote(%q)=%q want %q", in, got, want)
		}
	}
}

func TestPushError_UnwrapAndError(t *testing.T) {
	root := errors.New("root")
	pe := &PushError{Op: "push", Err: root, Output: "details"}
	if pe.Unwrap() != root {
		t.Fatal("Unwrap should return root error")
	}
	if !errors.Is(pe, root) {
		t.Fatal("errors.Is should find wrapped root error")
	}
	msg := pe.Error()
	if !strings.Contains(msg, "push:") || !strings.Contains(msg, "details") {
		t.Fatalf("unexpected PushError.Error output: %q", msg)
	}
}

func TestPushMode_TypeValues(t *testing.T) {
	var mode PushMode = PushModeBlankRepo
	if mode != 0 {
		t.Fatalf("PushModeBlankRepo = %d, want 0", mode)
	}
	if PushModeExistingRepo == PushModeBlankRepo {
		t.Fatal("push modes should be distinct")
	}
}

func TestStreamFn_TypeUsage(t *testing.T) {
	// Ensures StreamFn is exercised by tests as part of API contract.
	var fn StreamFn = func(line string) {
		if line == "" {
			t.Fatal("line should not be empty in this test")
		}
	}
	fn("ok")
}

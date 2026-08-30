// Package gitops manages git repository initialisation and push operations.
package gitops

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
)

// PushMode chooses the push strategy.
type PushMode int

const (
	// PushModeBlankRepo runs init/add-remote/branch/push for a blank remote.
	PushModeBlankRepo PushMode = iota
	// PushModeExistingRepo pushes an existing repository (handling upstream).
	PushModeExistingRepo
)

// PushConfig contains inputs for a push operation.
type PushConfig struct {
	Dir       string
	RemoteURL string
	Mode      PushMode
}

// StreamFn receives live command output lines.
type StreamFn func(line string)

// PushError wraps push failures with classified output.
type PushError struct {
	Op     string
	Output string
	Err    error
}

// Error formats PushError with operation context and captured git output.
func (e *PushError) Error() string {
	if e == nil {
		return ""
	}
	if e.Output != "" {
		if e.Err != nil {
			return fmt.Sprintf("%s\n\n(%s: %v)", e.Output, e.Op, e.Err)
		}
		return e.Output
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

// Unwrap returns the wrapped underlying error.
func (e *PushError) Unwrap() error { return e.Err }

// GetOriginURL returns the configured origin remote URL (if present).
func GetOriginURL(dir string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := strings.TrimSpace(string(out))
		// Standard git message when remote is absent.
		if strings.Contains(strings.ToLower(s), "no such remote") {
			return "", nil
		}
		return "", fmt.Errorf("git remote get-url origin: %w\n%s", err, s)
	}
	return strings.TrimSpace(string(out)), nil
}

// RepoNameFromRemote extracts the repository name from SSH/HTTPS/local-path remotes.
func RepoNameFromRemote(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "<repo>"
	}
	remote = strings.TrimSuffix(remote, ".git")
	remote = strings.TrimSuffix(remote, "/")
	remote = strings.TrimSuffix(remote, "\\")
	slash := strings.LastIndexAny(remote, "/\\:")
	if slash >= 0 && slash+1 < len(remote) {
		return remote[slash+1:]
	}
	return remote
}

// SetupGuidance renders the static guidance block from the spec, substituting
// repo/remote once known.
func SetupGuidance(remote string) string {
	repo := RepoNameFromRemote(remote)
	remoteForDoc := remote
	if remoteForDoc == "" {
		remoteForDoc = "git@github.com:<your-username>/<your-repo>.git"
	}

	return strings.Join([]string{
		"…or create a new repository on the command line",
		fmt.Sprintf("echo \"# %s\" >> README.md", repo),
		"git init",
		"git add README.md",
		"git commit -m \"first commit\"",
		"git branch -M main",
		fmt.Sprintf("git remote add origin %s", remoteForDoc),
		"git push -u origin main",
		"",
		"…or push an existing repository from the command line",
		fmt.Sprintf("git remote add origin %s", remoteForDoc),
		"git branch -M main",
		"git push -u origin main",
	}, "\n")
}

// ValidateRemoteURL validates SSH/HTTPS git remote formats.
func ValidateRemoteURL(remote string) error {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return errors.New("remote URL cannot be empty")
	}
	if isValidSCPRemote(remote) {
		return nil
	}
	if isValidURLRemote(remote) {
		return nil
	}
	return errors.New("remote URL must be a valid SSH or HTTPS git URL")
}

func isValidSCPRemote(remote string) bool {
	// Examples: git@github.com:user/repo.git, user@host:org/repo
	re := regexp.MustCompile(`^[^@\s]+@[^:\s]+:[^\s]+$`)
	return re.MatchString(remote)
}

func isValidURLRemote(remote string) bool {
	u, err := url.Parse(remote)
	if err != nil || u == nil {
		return false
	}
	if u.Scheme != "https" && u.Scheme != "ssh" {
		return false
	}
	if strings.TrimSpace(u.Host) == "" {
		return false
	}
	path := strings.Trim(strings.TrimSpace(u.Path), "/")
	return path != ""
}

// Push runs the configured push flow and streams real git output line-by-line.
func Push(cfg PushConfig, stream StreamFn) error {
	if cfg.Dir == "" {
		cfg.Dir = "output"
	}
	if cfg.Mode == PushModeBlankRepo {
		return pushBlank(cfg.Dir, cfg.RemoteURL, stream)
	}
	return pushExisting(cfg.Dir, cfg.RemoteURL, stream)
}

type gitRunner func(dir string, stream StreamFn, op string, args ...string) error

var runGitCommand gitRunner = runGitStreaming

func pushBlank(dir, remote string, stream StreamFn) error {
	if err := Init(dir); err != nil {
		return err
	}
	if err := ensureReadyToPush(dir, stream); err != nil {
		return err
	}
	if err := ensureOrigin(dir, remote, stream); err != nil {
		return err
	}
	return pushMain(dir, stream)
}

func pushExisting(dir, remote string, stream StreamFn) error {
	if err := ensureReadyToPush(dir, stream); err != nil {
		return err
	}
	if remote != "" {
		if err := ensureOrigin(dir, remote, stream); err != nil {
			return err
		}
	} else {
		if err := ensureOrigin(dir, "", stream); err != nil {
			return err
		}
	}
	return pushMain(dir, stream)
}

func pushMain(dir string, stream StreamFn) error {
	err := runGitCommand(dir, stream, "git push -u origin main", "git", "push", "-u", "origin", "main")
	if err == nil {
		return nil
	}
	// Non-fast-forward means the remote has diverged. CommitForge always
	// regenerates history from scratch, so force-pushing is correct here.
	if isNonFastForwardErr(err) {
		if stream != nil {
			stream("Remote history diverged — retrying with --force (CommitForge owns this repo's history).")
		}
		if ferr := runGitCommand(dir, stream, "git push --force -u origin main", "git", "push", "--force", "-u", "origin", "main"); ferr == nil {
			return nil
		}
		// Force push also failed — fall through to report the original error.
	}
	return friendly(err)
}

func isNonFastForwardErr(err error) bool {
	if err == nil {
		return false
	}
	pe := asPushError(err)
	l := strings.ToLower(pe.Output)
	return strings.Contains(l, "non-fast-forward") ||
		strings.Contains(l, "[rejected]") ||
		strings.Contains(l, "[remote rejected]") ||
		strings.Contains(l, "fetch first") ||
		(strings.Contains(l, "failed to push some refs") && !strings.Contains(l, "auth"))
}

func ensureOrigin(dir, remote string, stream StreamFn) error {
	if strings.TrimSpace(remote) == "" {
		existing, err := GetOriginURL(dir)
		if err != nil {
			return err
		}
		if strings.TrimSpace(existing) == "" {
			return &PushError{
				Op:     "configure remote",
				Err:    errors.New("missing remote URL"),
				Output: "no origin remote configured; provide --remote or enter one in the push flow",
			}
		}
		return nil
	}

	existing, err := GetOriginURL(dir)
	if err != nil {
		return err
	}
	if strings.TrimSpace(existing) == strings.TrimSpace(remote) {
		return nil
	}
	if strings.TrimSpace(existing) == "" {
		return runGitCommand(dir, stream, "git remote add origin <url>", "git", "remote", "add", "origin", remote)
	}
	return runGitCommand(dir, stream, "git remote set-url origin <url>", "git", "remote", "set-url", "origin", remote)
}

func ensureReadyToPush(dir string, stream StreamFn) error {
	hasCommits, err := repoHasCommits(dir)
	if err != nil {
		return err
	}
	if !hasCommits {
		return &PushError{
			Op:  "push preflight",
			Err: errors.New("no commits to push"),
			Output: "No commits to push. This indicates an internal flow error: " +
				"commit generation must run before push.",
		}
	}
	branch, err := currentBranchName(dir)
	if err != nil {
		return err
	}
	if branch != "main" {
		if err := runGitCommand(dir, stream, "git branch -M main", "git", "branch", "-M", "main"); err != nil {
			return friendly(err)
		}
	}
	return nil
}

func repoHasCommits(dir string) (bool, error) {
	out, err := runGitCombined(dir, "git rev-parse --verify HEAD", "git", "rev-parse", "--verify", "HEAD")
	if err == nil {
		return true, nil
	}
	l := strings.ToLower(out)
	switch {
	case strings.Contains(l, "does not have any commits yet"),
		strings.Contains(l, "needed a single revision"),
		strings.Contains(l, "unknown revision or path not in the working tree"),
		strings.Contains(l, "ambiguous argument 'head'"),
		strings.Contains(l, "bad revision 'head'"):
		return false, nil
	default:
		return false, &PushError{Op: "git rev-parse --verify HEAD", Err: err, Output: strings.TrimSpace(out)}
	}
}

// HasCommits reports whether the current repository has at least one commit.
func HasCommits(dir string) (bool, error) {
	return repoHasCommits(dir)
}

func currentBranchName(dir string) (string, error) {
	branch, err := runGitCombined(dir, "git symbolic-ref --short HEAD", "git", "symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil {
		b := strings.TrimSpace(branch)
		if b != "" && b != "HEAD" {
			return b, nil
		}
	}
	fallback, fallbackErr := runGitCombined(dir, "git rev-parse --abbrev-ref HEAD", "git", "rev-parse", "--abbrev-ref", "HEAD")
	if fallbackErr == nil {
		b := strings.TrimSpace(fallback)
		if b != "" && b != "HEAD" {
			return b, nil
		}
	}
	raw := strings.TrimSpace(branch + "\n" + fallback)
	if raw == "" {
		raw = "unable to detect current branch"
	}
	return "", &PushError{
		Op:     "detect current branch",
		Err:    errors.New("no current branch"),
		Output: formatFriendlyWithDebug("No current branch is checked out. Create or switch to a branch and try again.", raw),
	}
}

func runGitCombined(dir, op string, args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), &PushError{
			Op:     op,
			Err:    err,
			Output: strings.TrimSpace(string(out)),
		}
	}
	return strings.TrimSpace(string(out)), nil
}

func runGitStreaming(dir string, stream StreamFn, op string, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return &PushError{Op: op, Err: err}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return &PushError{Op: op, Err: err}
	}
	if err := cmd.Start(); err != nil {
		return &PushError{Op: op, Err: err}
	}

	var lines []string
	read := func(r io.Reader) {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			line := sc.Text()
			lines = append(lines, line)
			if stream != nil {
				stream(line)
			}
		}
	}
	done := make(chan struct{}, 2)
	go func() { read(stdout); done <- struct{}{} }()
	go func() { read(stderr); done <- struct{}{} }()
	<-done
	<-done

	if err := cmd.Wait(); err != nil {
		return &PushError{
			Op:     op,
			Err:    err,
			Output: strings.Join(lines, "\n"),
		}
	}
	return nil
}

func asPushError(err error) *PushError {
	var pe *PushError
	if errors.As(err, &pe) {
		return pe
	}
	return &PushError{Op: "push", Err: err}
}

func friendly(err error) error {
	pe := asPushError(err)
	msg := FriendlyErrorMessage(pe.Output)
	if msg == "" {
		msg = "Git push failed. See raw git output below."
	}
	return &PushError{
		Op:     pe.Op,
		Err:    pe.Err,
		Output: formatFriendlyWithDebug(msg, pe.Output),
	}
}

// FriendlyErrorMessage maps common push failures to clear actionable messages.
func FriendlyErrorMessage(output string) string {
	l := strings.ToLower(output)
	switch {
	case strings.Contains(l, "src refspec main does not match any"),
		strings.Contains(l, "does not have any commits yet"),
		strings.Contains(l, "needed a single revision"),
		strings.Contains(l, "no commits yet"):
		return "No commits to push. Generate commits first, then try pushing again."
	case strings.Contains(l, "not currently on a branch"),
		strings.Contains(l, "detached head"),
		strings.Contains(l, "no current branch"):
		return "No branch is currently checked out. Check out a branch, then push again."
	case strings.Contains(l, "permission denied (publickey)") ||
		(strings.Contains(l, "could not read from remote repository") && strings.Contains(l, "publickey")) ||
		strings.Contains(l, "authentication failed") ||
		strings.Contains(l, "could not read username"):
		return "Authentication failed. Verify your SSH/HTTPS credentials and remote access."
	case strings.Contains(l, "remote origin already exists"):
		return "Remote 'origin' already exists. Use a different remote or update origin URL."
	case strings.Contains(l, "non-fast-forward") ||
		strings.Contains(l, "[rejected]") ||
		strings.Contains(l, "[remote rejected]") ||
		strings.Contains(l, "failed to push some refs"):
		return "Push rejected (non-fast-forward). Run `git pull --rebase` and push again."
	case strings.Contains(l, "could not resolve host") ||
		strings.Contains(l, "name or service not known") ||
		strings.Contains(l, "network is unreachable") ||
		strings.Contains(l, "failed to connect") ||
		strings.Contains(l, "connection timed out"):
		return "Network error while pushing. Check your internet connection and remote URL."
	default:
		return ""
	}
}

func formatFriendlyWithDebug(message, raw string) string {
	message = strings.TrimSpace(message)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return message
	}
	return fmt.Sprintf("%s\n\nRaw git output:\n%s", message, raw)
}

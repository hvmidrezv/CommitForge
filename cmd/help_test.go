package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteRootHelp_IncludesFlagsAndExamples(t *testing.T) {
	var b bytes.Buffer
	writeRootHelp(&b)
	out := b.String()

	mustContain := []string{
		"How the TUI works",
		"--dir <path>",
		"--years <n>",
		"--message <text>",
		"--message-mode <mode>",
		"--remote <url>",
		"--no-push",
		"--yes",
		"commitforge panel --years 2",
		"commitforge panel --remote git@github.com:user/repo.git",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Fatalf("help output missing %q\n---\n%s", s, out)
		}
	}
}

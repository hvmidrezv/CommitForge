package tui

import (
	"strings"
	"testing"

	"commitforge/internal/contribution"
)

func TestRenderGrid_NotEmpty(t *testing.T) {
	cal := contribution.Build(1, testCal().EndDate)
	out := RenderGrid(cal, viewState{})
	if strings.TrimSpace(out) == "" {
		t.Fatal("RenderGrid returned empty output")
	}
}

func TestRenderGrid_ContainsLegendLabels(t *testing.T) {
	cal := testCal()
	out := RenderGrid(cal, viewState{})
	if !strings.Contains(out, "Less") || !strings.Contains(out, "More") {
		t.Fatalf("RenderGrid legend missing. output:\n%s", out)
	}
}

func TestRenderGrid_ShowsTodayMarkerAndLegendEntry(t *testing.T) {
	cal := contribution.Build(1, testCal().EndDate)
	out := RenderGrid(cal, viewState{})
	if !strings.Contains(out, "◆ = today") {
		t.Fatalf("today legend entry missing. output:\n%s", out)
	}
	if count := strings.Count(out, "◆"); count < 2 {
		t.Fatalf("expected today marker in grid plus legend, got count=%d\n%s", count, out)
	}
}

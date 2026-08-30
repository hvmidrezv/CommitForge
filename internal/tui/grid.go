// Package tui contains the bubbletea TUI model and views for CommitForge.
package tui

import (
	"strings"
	"time"

	"commitforge/internal/contribution"

	"github.com/charmbracelet/lipgloss"
)

const todayMarker = "◆"

var weekdayLabels = [7]string{"   ", "Mon", "   ", "Wed", "   ", "Fri", "   "}

var monthAbbr = map[time.Month]string{
	time.January:   "Jan",
	time.February:  "Feb",
	time.March:     "Mar",
	time.April:     "Apr",
	time.May:       "May",
	time.June:      "Jun",
	time.July:      "Jul",
	time.August:    "Aug",
	time.September: "Sep",
	time.October:   "Oct",
	time.November:  "Nov",
	time.December:  "Dec",
}

// viewState bundles interactive rendering state passed down from the Model.
type viewState struct {
	cursor        CellPos
	sel           *contribution.Selection
	rangeMode     bool
	rangeAnchor   CellPos
	previewCounts map[string]int
	compact       bool
}

type tileKind int

const (
	tileBase tileKind = iota
	tileSelected
	tileCursor
	tileCursorSelected
	tileRangeAnchor
	tileRangePreview
)

// RenderGrid builds the full contribution grid as a styled terminal string.
func RenderGrid(cal contribution.Calendar, vs viewState) string {
	if len(cal.Weeks) == 0 {
		return ""
	}

	cellW := 2
	colGap := " "
	monthLabelW := 3
	if vs.compact {
		cellW = 1
		colGap = ""
		monthLabelW = 1
	}
	leftPad := "    "

	var rangeFrom, rangeTo time.Time
	if vs.rangeMode {
		a := cal.Weeks[vs.rangeAnchor.Week][vs.rangeAnchor.Weekday].Date
		c := cal.Weeks[vs.cursor.Week][vs.cursor.Weekday].Date
		rangeFrom, rangeTo = a, c
		if rangeFrom.After(rangeTo) {
			rangeFrom, rangeTo = rangeTo, rangeFrom
		}
	}

	var sb strings.Builder
	sb.WriteString(renderMonthRow(cal.Weeks, leftPad, monthLabelW, colGap))
	sb.WriteByte('\n')
	for wd := 0; wd < 7; wd++ {
		sb.WriteString(renderDayRow(cal.Weeks, wd, cal.EndDate, vs, rangeFrom, rangeTo, cellW, colGap))
		sb.WriteByte('\n')
	}
	sb.WriteByte('\n')
	sb.WriteString(renderLegend(vs.compact, leftPad, cellW, colGap))
	return sb.String()
}

func renderMonthRow(weeks [][]contribution.Day, leftPad string, monthLabelW int, colGap string) string {
	var sb strings.Builder
	sb.WriteString(leftPad)
	for _, week := range weeks {
		abbr := monthLabelForWeek(week)
		if abbr == "" {
			sb.WriteString(strings.Repeat(" ", monthLabelW))
		} else if monthLabelW == 1 {
			sb.WriteString(abbr[:1])
		} else {
			sb.WriteString(MonthLabelStyle.Render(abbr))
		}
		sb.WriteString(colGap)
	}
	return sb.String()
}

func renderDayRow(
	weeks [][]contribution.Day,
	wd int,
	today time.Time,
	vs viewState,
	rangeFrom, rangeTo time.Time,
	cellW int,
	colGap string,
) string {
	var sb strings.Builder
	sb.WriteString(WeekdayStyle.Render(weekdayLabels[wd]))
	sb.WriteString(" ")
	for wi, week := range weeks {
		day := week[wd]
		text, style := renderCell(day, wi, wd, today, vs, rangeFrom, rangeTo, cellW)
		sb.WriteString(style.Render(text))
		sb.WriteString(colGap)
	}
	return sb.String()
}

func renderCell(
	day contribution.Day,
	wi, wd int,
	today time.Time,
	vs viewState,
	rangeFrom, rangeTo time.Time,
	cellW int,
) (string, lipgloss.Style) {
	if day.Date.IsZero() {
		return strings.Repeat(" ", cellW), IntensityCellStyles[0]
	}

	level := int(contribution.CountToLevel(day.Count))
	if vs.previewCounts != nil {
		k := day.Date.UTC().Format("2006-01-02")
		if cnt, ok := vs.previewCounts[k]; ok {
			level = int(contribution.CountToLevel(cnt))
		}
	}

	style := IntensityCellStyles[level]
	kind := resolveTileKind(day, wi, wd, vs, rangeFrom, rangeTo)
	text := tileText(kind, cellW)

	switch kind {
	case tileCursor, tileCursorSelected:
		style = style.Inherit(GridCursorStyle)
	case tileSelected:
		style = style.Inherit(GridSelectedStyle)
	case tileRangeAnchor:
		style = style.Inherit(GridRangeAnchorStyle)
	case tileRangePreview:
		style = style.Inherit(GridRangePreviewStyle)
	}

	if isToday(day.Date, today) {
		text = overlayTodayMarker(text)
		style = style.Inherit(GridTodayStyle)
	}

	return text, style
}

func resolveTileKind(
	day contribution.Day,
	wi, wd int,
	vs viewState,
	rangeFrom, rangeTo time.Time,
) tileKind {
	isCursor := wi == vs.cursor.Week && wd == vs.cursor.Weekday
	isSelected := vs.sel != nil && vs.sel.IsSelected(day.Date)

	if isCursor && isSelected {
		return tileCursorSelected
	}
	if isCursor {
		return tileCursor
	}
	if vs.rangeMode {
		if wi == vs.rangeAnchor.Week && wd == vs.rangeAnchor.Weekday {
			return tileRangeAnchor
		}
		if !day.Date.Before(rangeFrom) && !day.Date.After(rangeTo) {
			return tileRangePreview
		}
	}
	if isSelected {
		return tileSelected
	}
	return tileBase
}

func tileText(kind tileKind, cellW int) string {
	if cellW <= 1 {
		switch kind {
		case tileCursor:
			return "▣"
		case tileSelected:
			return "•"
		case tileCursorSelected:
			return "◉"
		case tileRangeAnchor:
			return "▤"
		case tileRangePreview:
			return "·"
		default:
			return " "
		}
	}

	switch kind {
	case tileCursor:
		return "[]"
	case tileSelected:
		return "{}"
	case tileCursorSelected:
		return "<>"
	case tileRangeAnchor:
		return "||"
	case tileRangePreview:
		return ".."
	default:
		return "  "
	}
}

func overlayTodayMarker(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return todayMarker
	}
	runes[0] = []rune(todayMarker)[0]
	return string(runes)
}

func renderLegend(compact bool, leftPad string, cellW int, colGap string) string {
	var sb strings.Builder
	sb.WriteString(leftPad)
	sb.WriteString(LegendLabelStyle.Render("Less"))
	sb.WriteString(" ")
	for lvl := 0; lvl <= 4; lvl++ {
		block := strings.Repeat(" ", cellW)
		if compact {
			block = " "
		}
		sb.WriteString(IntensityCellStyles[lvl].Render(block))
		sb.WriteString(colGap)
	}
	sb.WriteString(" ")
	sb.WriteString(LegendLabelStyle.Render("More"))
	sb.WriteString("   ")
	sb.WriteString(GridSelectedStyle.Render("{} = selected"))
	sb.WriteString("  ")
	sb.WriteString(GridCursorStyle.Render("[] = focus"))
	sb.WriteString("  ")
	sb.WriteString(GridTodayStyle.Render(todayMarker + " = today"))
	return sb.String()
}

func monthLabelForWeek(week []contribution.Day) string {
	for _, d := range week {
		if !d.Date.IsZero() && d.Date.Day() == 1 {
			return monthAbbr[d.Date.Month()]
		}
	}
	return ""
}

func isToday(date time.Time, today time.Time) bool {
	if date.IsZero() || today.IsZero() {
		return false
	}
	return date.Equal(today)
}

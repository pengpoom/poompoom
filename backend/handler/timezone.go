package handler

import (
	"fmt"
	"time"
	_ "time/tzdata"
)

const (
	parseTimeZoneName           = "America/New_York"
	parseTimeFallbackOffsetSecs = -5 * 60 * 60
)

func loadLocationOrFixed(name string, fallbackOffsetSeconds int) *time.Location {
	loc, err := time.LoadLocation(name)
	if err == nil && loc != nil {
		return loc
	}
	return time.FixedZone(name, fallbackOffsetSeconds)
}

func formatBrowserParseTime(now time.Time, loc *time.Location) string {
	local := now.In(loc)
	_, offsetSeconds := local.Zone()
	return fmt.Sprintf(
		"%s GMT%s (%s)",
		local.Format("Mon Jan 02 2006 15:04:05"),
		formatGMTOffset(offsetSeconds),
		easternTimeLabel(offsetSeconds),
	)
}

func formatGMTOffset(offsetSeconds int) string {
	sign := "+"
	if offsetSeconds < 0 {
		sign = "-"
		offsetSeconds = -offsetSeconds
	}

	hours := offsetSeconds / 3600
	minutes := (offsetSeconds % 3600) / 60
	return fmt.Sprintf("%s%02d%02d", sign, hours, minutes)
}

func easternTimeLabel(offsetSeconds int) string {
	if offsetSeconds == -4*60*60 {
		return "Eastern Daylight Time"
	}
	return "Eastern Standard Time"
}

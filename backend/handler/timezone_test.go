package handler

import (
	"strings"
	"testing"
	"time"
)

func TestLoadLocationOrFixedFallsBack(t *testing.T) {
	loc := loadLocationOrFixed("Mars/OlympusMons", -5*60*60)
	if loc == nil {
		t.Fatal("expected fallback location")
	}

	_, offset := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC).In(loc).Zone()
	if offset != -5*60*60 {
		t.Fatalf("expected fallback offset -18000, got %d", offset)
	}
}

func TestFormatBrowserParseTimeTracksEasternDST(t *testing.T) {
	loc := loadLocationOrFixed(parseTimeZoneName, parseTimeFallbackOffsetSecs)

	winter := formatBrowserParseTime(time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC), loc)
	if !strings.Contains(winter, "GMT-0500 (Eastern Standard Time)") {
		t.Fatalf("expected winter parse time to use EST, got %q", winter)
	}

	summer := formatBrowserParseTime(time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC), loc)
	if !strings.Contains(summer, "GMT-0400 (Eastern Daylight Time)") {
		t.Fatalf("expected summer parse time to use EDT, got %q", summer)
	}
}

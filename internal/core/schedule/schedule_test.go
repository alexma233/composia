package schedule

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeAndDisabledSchedule(t *testing.T) {
	t.Parallel()

	if got := Normalize(" NoNe "); got != Disabled {
		t.Fatalf("Normalize disabled = %q", got)
	}
	if got := Normalize(" 0 4 * * * "); got != "0 4 * * *" {
		t.Fatalf("Normalize cron = %q", got)
	}
	if !IsDisabled(" NONE ") {
		t.Fatalf("expected NONE to be disabled")
	}
	if IsDisabled("0 4 * * *") {
		t.Fatalf("expected cron schedule to be enabled")
	}
}

func TestParseRejectsMissingOrDisabledSchedule(t *testing.T) {
	t.Parallel()

	for _, spec := range []string{"", "none"} {
		if _, err := Parse(spec); err == nil {
			t.Fatalf("Parse(%q) expected error", spec)
		}
	}
}

func TestValidateAllowsEmptyAndDisabledSchedule(t *testing.T) {
	t.Parallel()

	for _, spec := range []string{"", "none", "NONE"} {
		if err := Validate(spec); err != nil {
			t.Fatalf("Validate(%q) returned error: %v", spec, err)
		}
	}
}

func TestValidateRejectsInvalidSchedule(t *testing.T) {
	t.Parallel()

	err := Validate("invalid")
	if err == nil || !strings.Contains(err.Error(), "parse schedule") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestDueNowUsesMinuteWindow(t *testing.T) {
	t.Parallel()

	parsed, err := Parse("5 4 * * *")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !DueNow(parsed, time.Date(2026, 5, 31, 12, 5, 42, 0, time.FixedZone("local", 8*60*60))) {
		t.Fatalf("expected schedule to be due in matching UTC minute")
	}
	if DueNow(parsed, time.Date(2026, 5, 31, 4, 6, 0, 0, time.UTC)) {
		t.Fatalf("expected schedule not to be due outside matching minute")
	}
	if DueNow(nil, time.Now()) {
		t.Fatalf("nil schedule should never be due")
	}
}

func TestWindowStartTruncatesUTCMinute(t *testing.T) {
	t.Parallel()

	got := WindowStart(time.Date(2026, 5, 31, 12, 34, 56, 789, time.FixedZone("local", 8*60*60)))
	want := time.Date(2026, 5, 31, 4, 34, 0, 0, time.UTC)
	if !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("WindowStart = %s, want %s UTC", got, want)
	}
}

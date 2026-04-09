package schedule

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

const Disabled = "none"

var parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func Normalize(spec string) string {
	spec = strings.TrimSpace(spec)
	if strings.EqualFold(spec, Disabled) {
		return Disabled
	}
	return spec
}

func IsDisabled(spec string) bool {
	return Normalize(spec) == Disabled
}

func Parse(spec string) (cron.Schedule, error) {
	spec = Normalize(spec)
	if spec == "" {
		return nil, fmt.Errorf("schedule is required")
	}
	if spec == Disabled {
		return nil, fmt.Errorf("disabled schedule cannot be parsed")
	}
	parsed, err := parser.Parse(spec)
	if err != nil {
		return nil, fmt.Errorf("parse schedule %q: %w", spec, err)
	}
	return parsed, nil
}

func Validate(spec string) error {
	spec = Normalize(spec)
	if spec == "" || spec == Disabled {
		return nil
	}
	_, err := Parse(spec)
	return err
}

func DueNow(parsed cron.Schedule, now time.Time) bool {
	if parsed == nil {
		return false
	}
	windowStart := now.UTC().Truncate(time.Minute)
	return parsed.Next(windowStart.Add(-time.Minute)) == windowStart
}

func WindowStart(now time.Time) time.Time {
	return now.UTC().Truncate(time.Minute)
}

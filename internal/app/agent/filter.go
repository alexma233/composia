package agent

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
)

func candidateImageTags(current string, tags []string, filter *repo.ImageUpdateFilter) []string {
	if filter == nil {
		return nil
	}
	current = strings.TrimSpace(current)
	candidates := make([]string, 0, len(tags))
	switch filter.Type {
	case "semver":
		currentVersion, ok := parseSimpleSemver(current)
		if !ok {
			return nil
		}
		allowed := semverAllowedUpdates(filter.Allow)
		versions := make([]simpleSemverTag, 0, len(tags))
		for _, tag := range tags {
			version, ok := parseSimpleSemver(tag)
			if !ok || !version.greaterThan(currentVersion) || !semverUpdateAllowed(currentVersion, version, allowed) {
				continue
			}
			versions = append(versions, simpleSemverTag{Tag: tag, Version: version})
		}
		slices.SortFunc(versions, func(left, right simpleSemverTag) int { return right.Version.compare(left.Version) })
		for _, version := range versions {
			candidates = append(candidates, version.Tag)
		}
	case "date":
		currentTime, err := time.Parse(filter.Format, current)
		if err != nil {
			return nil
		}
		type dateTag struct {
			Tag string
			At  time.Time
		}
		dateTags := make([]dateTag, 0, len(tags))
		for _, tag := range tags {
			parsed, err := time.Parse(filter.Format, tag)
			if err == nil && parsed.After(currentTime) {
				dateTags = append(dateTags, dateTag{Tag: tag, At: parsed})
			}
		}
		slices.SortFunc(dateTags, func(left, right dateTag) int { return right.At.Compare(left.At) })
		for _, tag := range dateTags {
			candidates = append(candidates, tag.Tag)
		}
	case "regex":
		re, err := regexp.Compile(filter.Pattern)
		if err != nil {
			return nil
		}
		currentKey, ok := regexOrderKey(re, current, filter.Order)
		if !ok {
			return nil
		}
		type regexTag struct {
			Tag string
			Key string
		}
		regexTags := make([]regexTag, 0, len(tags))
		for _, tag := range tags {
			key, ok := regexOrderKey(re, tag, filter.Order)
			if !ok || compareRegexKeys(key, currentKey, filter.Order) <= 0 {
				continue
			}
			regexTags = append(regexTags, regexTag{Tag: tag, Key: key})
		}
		slices.SortFunc(regexTags, func(left, right regexTag) int { return compareRegexKeys(right.Key, left.Key, filter.Order) })
		for _, tag := range regexTags {
			candidates = append(candidates, tag.Tag)
		}
	case "latest":
		candidates = append(candidates, tags...)
	}
	return candidates
}

type simpleSemver struct{ Major, Minor, Patch int }
type simpleSemverTag struct {
	Tag     string
	Version simpleSemver
}

func parseSimpleSemver(value string) (simpleSemver, bool) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return simpleSemver{}, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return simpleSemver{}, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return simpleSemver{}, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return simpleSemver{}, false
	}
	return simpleSemver{Major: major, Minor: minor, Patch: patch}, true
}

func (version simpleSemver) compare(other simpleSemver) int {
	if version.Major != other.Major {
		return version.Major - other.Major
	}
	if version.Minor != other.Minor {
		return version.Minor - other.Minor
	}
	return version.Patch - other.Patch
}

func (version simpleSemver) greaterThan(other simpleSemver) bool { return version.compare(other) > 0 }

func semverAllowedUpdates(allow []string) map[string]struct{} {
	if len(allow) == 0 {
		allow = []string{"patch", "minor"}
	}
	allowed := make(map[string]struct{}, len(allow))
	for _, item := range allow {
		allowed[strings.TrimSpace(item)] = struct{}{}
	}
	return allowed
}

func semverUpdateAllowed(current, candidate simpleSemver, allowed map[string]struct{}) bool {
	updateType := "patch"
	if candidate.Major != current.Major {
		updateType = "major"
	} else if candidate.Minor != current.Minor {
		updateType = "minor"
	}
	_, ok := allowed[updateType]
	return ok
}

func regexOrderKey(re *regexp.Regexp, value, order string) (string, bool) {
	matches := re.FindStringSubmatch(value)
	if len(matches) == 0 {
		return "", false
	}
	key := matches[0]
	if len(matches) > 1 {
		key = matches[1]
	}
	if order == "numeric" {
		if _, err := strconv.ParseInt(key, 10, 64); err != nil {
			return "", false
		}
	}
	return key, true
}

func compareRegexKeys(left, right, order string) int {
	if order == "numeric" {
		leftNumber, _ := strconv.ParseInt(left, 10, 64)
		rightNumber, _ := strconv.ParseInt(right, 10, 64)
		switch {
		case leftNumber < rightNumber:
			return -1
		case leftNumber > rightNumber:
			return 1
		default:
			return 0
		}
	}
	return strings.Compare(left, right)
}

package agent

import (
	"context"
	"fmt"
	"strings"

	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
)

func resolveImageUpdateDiscovery(discovery repo.ImageUpdateDiscovery, sources map[string]repo.ImageUpdateDiscovery) repo.ImageUpdateDiscovery {
	if discovery.Ref == "" {
		return discovery
	}
	if source, ok := sources[discovery.Ref]; ok {
		return source
	}
	return discovery
}

func discoverImageUpdateTags(ctx context.Context, imageRef, currentTag string, discovery repo.ImageUpdateDiscovery, filter *repo.ImageUpdateFilter) ([]string, error) {
	if discovery.Auto != nil && *discovery.Auto {
		return discoverMergedImageUpdateTags(ctx, imageRef, currentTag, []repo.ImageUpdateDiscoverySource{{Type: "probe"}, {Type: "registry"}}, filter)
	}
	if len(discovery.Sources) > 0 {
		if discovery.Combine == "merge" {
			return discoverMergedImageUpdateTags(ctx, imageRef, currentTag, discovery.Sources, filter)
		}
		for _, source := range discovery.Sources {
			tags, err := discoverImageUpdateTagsFromSource(ctx, imageRef, currentTag, source, filter)
			if err != nil {
				return nil, err
			}
			if len(tags) > 0 {
				return tags, nil
			}
		}
		return nil, nil
	}
	if discovery.Type != "" {
		return discoverImageUpdateTagsFromSource(ctx, imageRef, currentTag, repo.ImageUpdateDiscoverySource{Type: discovery.Type, Repo: discovery.Repo, Project: discovery.Project}, filter)
	}
	return discoverImageUpdateTagsFromSource(ctx, imageRef, currentTag, repo.ImageUpdateDiscoverySource{Type: "registry"}, filter)
}

func discoverMergedImageUpdateTags(ctx context.Context, imageRef, currentTag string, sources []repo.ImageUpdateDiscoverySource, filter *repo.ImageUpdateFilter) ([]string, error) {
	seen := make(map[string]struct{})
	merged := make([]string, 0)
	for _, source := range sources {
		tags, err := discoverImageUpdateTagsFromSource(ctx, imageRef, currentTag, source, filter)
		if err != nil {
			return nil, err
		}
		for _, tag := range tags {
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			merged = append(merged, tag)
		}
	}
	return merged, nil
}

func discoverImageUpdateTagsFromSource(ctx context.Context, imageRef, currentTag string, source repo.ImageUpdateDiscoverySource, filter *repo.ImageUpdateFilter) ([]string, error) {
	switch source.Type {
	case "probe":
		if filter == nil || filter.Type != "semver" {
			return nil, nil
		}
		candidate, found, err := probeSemverImageTag(ctx, imageRef, currentTag, filter)
		if err != nil || !found {
			return nil, err
		}
		return []string{candidate}, nil
	case "registry", "":
		return listRegistryTags(ctx, imageRef)
	case "github", "gitlab", "forgejo":
		return nil, fmt.Errorf("%s release discovery is not implemented yet", source.Type)
	default:
		return nil, fmt.Errorf("unsupported image update discovery source %q", source.Type)
	}
}

func probeSemverImageTag(ctx context.Context, imageRef, current string, filter *repo.ImageUpdateFilter) (string, bool, error) {
	return probeSemverImageTagWithExists(ctx, imageRef, current, filter, func(ctx context.Context, imageRef, tag string) (bool, error) {
		return registryManifestExists(ctx, imageRef, tag)
	})
}

func probeSemverImageTagWithExists(ctx context.Context, imageRef, current string, filter *repo.ImageUpdateFilter, exists func(context.Context, string, string) (bool, error)) (string, bool, error) {
	if filter == nil {
		return "", false, nil
	}
	currentVersion, ok := parseSimpleSemver(current)
	if !ok {
		return "", false, nil
	}
	prefix := ""
	if strings.HasPrefix(strings.TrimSpace(current), "v") {
		prefix = "v"
	}
	allowed := semverAllowedUpdates(filter.Allow)
	type semverProbeCandidate struct {
		Tag     string
		Version simpleSemver
	}
	var best semverProbeCandidate
	consider := func(version simpleSemver, found bool) {
		if !found {
			return
		}
		tag := fmt.Sprintf("%s%d.%d.%d", prefix, version.Major, version.Minor, version.Patch)
		if best.Tag == "" || version.compare(best.Version) > 0 {
			best = semverProbeCandidate{Tag: tag, Version: version}
		}
	}
	probePatch := func(major, minor, startPatch int) (simpleSemver, bool, error) {
		highestPatch, found, err := probeContiguousSemverComponent(ctx, startPatch, func(patch int) (bool, error) {
			return exists(ctx, imageRef, fmt.Sprintf("%s%d.%d.%d", prefix, major, minor, patch))
		})
		if err != nil || !found {
			return simpleSemver{}, found, err
		}
		return simpleSemver{Major: major, Minor: minor, Patch: highestPatch}, true, nil
	}
	if _, ok := allowed["patch"]; ok {
		version, found, err := probePatch(currentVersion.Major, currentVersion.Minor, currentVersion.Patch+1)
		if err != nil {
			return "", false, err
		}
		consider(version, found)
	}
	if _, ok := allowed["minor"]; ok {
		highestMinor, found, err := probeContiguousSemverComponent(ctx, currentVersion.Minor+1, func(minor int) (bool, error) {
			return exists(ctx, imageRef, fmt.Sprintf("%s%d.%d.0", prefix, currentVersion.Major, minor))
		})
		if err != nil {
			return "", false, err
		}
		if found {
			version, found, err := probePatch(currentVersion.Major, highestMinor, 0)
			if err != nil {
				return "", false, err
			}
			consider(version, found)
		}
	}
	if _, ok := allowed["major"]; ok {
		highestMajor, found, err := probeContiguousSemverComponent(ctx, currentVersion.Major+1, func(major int) (bool, error) {
			return exists(ctx, imageRef, fmt.Sprintf("%s%d.0.0", prefix, major))
		})
		if err != nil {
			return "", false, err
		}
		if found {
			highestMinor, found, err := probeContiguousSemverComponent(ctx, 0, func(minor int) (bool, error) {
				return exists(ctx, imageRef, fmt.Sprintf("%s%d.%d.0", prefix, highestMajor, minor))
			})
			if err != nil {
				return "", false, err
			}
			if found {
				version, found, err := probePatch(highestMajor, highestMinor, 0)
				if err != nil {
					return "", false, err
				}
				consider(version, found)
			}
		}
	}
	if best.Tag == "" {
		return "", false, nil
	}
	return best.Tag, true, nil
}

func probeContiguousSemverComponent(ctx context.Context, start int, exists func(int) (bool, error)) (int, bool, error) {
	if start < 0 {
		return 0, false, nil
	}
	ok, err := exists(start)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	low := start
	step := 1
	for {
		candidate := start + step
		ok, err := exists(candidate)
		if err != nil {
			return 0, false, err
		}
		if !ok {
			high := candidate - 1
			for low < high {
				mid := (low + high + 1) / 2
				ok, err := exists(mid)
				if err != nil {
					return 0, false, err
				}
				if ok {
					low = mid
				} else {
					high = mid - 1
				}
			}
			return low, true, nil
		}
		low = candidate
		step *= 2
		if step < 0 {
			return low, true, nil
		}
		select {
		case <-ctx.Done():
			return 0, false, ctx.Err()
		default:
		}
	}
}

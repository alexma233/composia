package agent

const (
	backupStrategyFilesCopy          = "files.copy"
	backupStrategyFilesCopyAfterStop = "files.copy_after_stop"
	backupStrategyPostgresDumpAll    = "database.pgdumpall"

	imageUpdateDiscoveryAuto     = "auto"
	imageUpdateDiscoveryProbe    = "probe"
	imageUpdateDiscoveryRegistry = "registry"
	imageUpdateDiscoveryGitHub   = "github"
	imageUpdateDiscoveryGitLab   = "gitlab"
	imageUpdateDiscoveryForgejo  = "forgejo"
	imageUpdateDiscoveryMerge    = "merge"

	imageUpdateFilterSemver = "semver"
	imageUpdatePolicyDigest = "digest"

	semverUpdateMajor = "major"
	semverUpdateMinor = "minor"
	semverUpdatePatch = "patch"

	dockerCommandPrune = "prune"

	dockerResourceContainer = "container"
	dockerResourceNetwork   = "network"
	dockerResourceVolume    = "volume"
	dockerResourceImage     = "image"
)

package controller

import (
	"time"
)

const (
	heartbeatOfflineAfter = 45 * time.Second
	offlineSweepInterval  = 15 * time.Second
	pullNextTaskMaxWait   = 25 * time.Second
	pullNextTaskRetryWait = 500 * time.Millisecond
	taskExecutionLease    = 60 * time.Second

	backupProviderRustic = "rustic"

	composeRecreateAuto  = "auto"
	composeRecreateNo    = "no_recreate"
	composeRecreateForce = "force_recreate"

	dnsRecordTypeA     = "A"
	dnsRecordTypeAAAA  = "AAAA"
	dnsRecordTypeCNAME = "CNAME"

	dockerActionList    = "list"
	dockerActionInspect = "inspect"
	dockerActionLogs    = "logs"
	dockerActionExec    = "exec"
	dockerActionRemove  = "remove"
	dockerActionStart   = "start"
	dockerActionStop    = "stop"
	dockerActionRestart = "restart"

	dockerParamAction   = "action"
	dockerParamResource = "resource"
	dockerParamID       = "id"

	dockerResourceContainer  = "container"
	dockerResourceContainers = "containers"
	dockerResourceNetwork    = "network"
	dockerResourceNetworks   = "networks"
	dockerResourceVolume     = "volume"
	dockerResourceVolumes    = "volumes"
	dockerResourceImage      = "image"
	dockerResourceImages     = "images"

	imageUpdateDiscoveryAuto    = "auto"
	imageUpdateDiscoveryGitHub  = "github"
	imageUpdateDiscoveryGitLab  = "gitlab"
	imageUpdateDiscoveryForgejo = "forgejo"
	imageUpdateDiscoveryMerge   = "merge"
)

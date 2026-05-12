package controller

import (
	"time"
)

const (
	heartbeatOfflineAfter = 45 * time.Second
	offlineSweepInterval  = 15 * time.Second
	pullNextTaskMaxWait   = 25 * time.Second
	pullNextTaskRetryWait = 500 * time.Millisecond
)

package cyberdaemon

import (
	"fmt"

	"github.com/stephen-fox/cyberdaemon/internal/osutil"
)

type errDaemonizer struct {
	reason string
}

func (o *errDaemonizer) RunUntilExit(_ Application) error {
	return fmt.Errorf("no suitable daemonization logic available for this system - %s", o.reason)
}

func NewDaemonizer(logConfig LogConfig) Daemonizer {
	if _, isSystemd := osutil.IsSystemd(); isSystemd {
		return newSystemdDaemonizer(logConfig)
	}

	_, _, notVReason, isSystemv := osutil.IsSystemv()
	if isSystemv {
		return newSystemvDaemonizer(logConfig)
	}

	return &errDaemonizer{
		reason: notVReason,
	}
}

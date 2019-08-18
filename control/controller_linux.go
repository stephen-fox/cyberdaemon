package control

import (
	"fmt"

	"github.com/stephen-fox/cyberdaemon/internal/osutil"
)

// TODO: Provide a means to override the daemon CLI executable path. Also,
//  search some common directories for the executable after trying defaults.
func NewController(controllerConfig ControllerConfig) (Controller, error) {
	if systemctlPath, isSystemd := osutil.IsSystemd(); isSystemd {
		return newSystemdController(controllerConfig, systemctlPath)
	}

	servicePath, isRedHat, notVReason, isSystemv := osutil.IsSystemv()
	if isSystemv {
		return newSystemvController(controllerConfig, servicePath, isRedHat)
	}

	return nil, fmt.Errorf(notVReason)
}

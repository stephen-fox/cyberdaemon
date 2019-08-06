package cyberdaemon

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func NewDaemon(config Config) (Daemon, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}

	if isSystemd() {
		return nil, fmt.Errorf("systemd is currently unsupported")
	} else if isSystemv() {
		return newSystemvDaemon(exePath, config)
	}

	return nil, fmt.Errorf("systemd and system v were not found - no supported daemon types available")
}

func isSystemd() bool {
	_, err := exec.Command("/bin/systemctl").CombinedOutput()
	if err != nil {
		return false
	}

	return true
}

func isSystemv() bool {
	output, _, _ := runServiceCommand()
	if strings.HasPrefix(output, "Usage: service <") {
		return true
	}

	return false
}

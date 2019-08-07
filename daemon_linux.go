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
	}

	servicePath, err := serviceExePath()
	if err != nil {
		return nil, err
	}

	output, _, _ := runServiceCommand(servicePath)
	if strings.HasPrefix(output, "Usage: service <") {
		return newSystemvDaemon(exePath, config, servicePath)
	}

	return nil, fmt.Errorf("failed to determine linux daemon type after checking for systemd and system v")
}

func isSystemd() bool {
	_, err := exec.Command("/bin/systemctl").CombinedOutput()
	if err != nil {
		return false
	}

	return true
}

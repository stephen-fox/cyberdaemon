package cyberdaemon

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
)

// TODO: Provide a means to override the daemon CLI executable path. Also,
//  search some common directories for the executable after trying defaults.
func NewController(config Config) (Controller, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}

	if systemctlPath, isSystemd := isSystemd(); isSystemd {
		return newSystemController(exePath, config, systemctlPath)
	}

	servicePath, isRedHat, notVReason, isSystemv := isSystemv()
	if isSystemv {
		return newSystemvController(exePath, config, servicePath, isRedHat)
	}

	return nil, fmt.Errorf(notVReason)
}

func NewDaemonizer(logConfig LogConfig) Daemonizer {
	if _, isSystemd := isSystemd(); isSystemd {
		return newSystemdDaemonizer(logConfig)
	}

	// TODO: What if this is not a system v machine?
	//  Return an error? Is this a sane default?
	return newSystemdDaemonizer(logConfig)
}

func isSystemd() (systemctlPath string, ok bool) {
	systemctlPath, findErr := searchForExeInPaths(systemctlExeName, systemctlExeDirPaths)
	if findErr == nil {
		if _, systemctlExitCode, _ := runDaemonCli(systemctlPath); systemctlExitCode == 0 {
			return systemctlPath, true
		}
	}

	return "", false
}

func isSystemv() (servicePath string, isRedHat bool, whyNotSysV string, ok bool) {
	servicePath, err := searchForExeInPaths(serviceExeName, serviceExeDirPaths)
	if err != nil {
		return "", false, err.Error(), false
	}

	output, _, _ := runDaemonCli(servicePath)
	if !strings.HasPrefix(output, "Usage: service <") {
		return "", false,
			fmt.Sprintf("'%s' did not produce expected output", servicePath), false
	}

	i, redhatStatErr := os.Stat("/etc/redhat-release")
	if redhatStatErr != nil || i.IsDir() {
		return servicePath, false, "", true
	}

	return servicePath, true, "", true
}

func searchForExeInPaths(exeName string, dirSearchPaths []string) (string, error) {
	for i := range dirSearchPaths {
		filePath := path.Join(dirSearchPaths[i], exeName)
		info, err := os.Stat(filePath)
		if err == nil && !info.IsDir() {
			return filePath, nil
		}
	}

	return "", fmt.Errorf("failed to locate '%s' executable in the following directory paths: %v",
		exeName, dirSearchPaths)
}

func runDaemonCli(exePath string, args ...string) (string, int, error) {
	s := exec.Command(exePath, args...)
	output, err := s.CombinedOutput()
	trimmedOutput := strings.TrimSpace(string(output))
	if err != nil {
		return trimmedOutput, s.ProcessState.ExitCode(),
			fmt.Errorf("failed to execute '%s %s' - %s - output: %s",
				exePath, args, err.Error(), trimmedOutput)
	}

	return trimmedOutput, s.ProcessState.ExitCode(), nil
}

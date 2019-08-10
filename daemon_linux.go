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
func NewDaemon(config Config) (Daemon, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}

	systemctlPath, findErr := searchForExeInPaths(systemctlExeName, systemctlExeDirPaths)
	if findErr == nil {
		if _, systemctlExitCode, _ := runDaemonCli(systemctlPath); systemctlExitCode == 0 {
			return newSystemdDaemon(exePath, config, systemctlPath)
		}
	}

	servicePath, err := searchForExeInPaths(serviceExeName, serviceExeDirPaths)
	if err != nil {
		return nil, err
	}

	output, _, _ := runDaemonCli(servicePath)
	if strings.HasPrefix(output, "Usage: service <") {
		return newSystemvDaemon(exePath, config, servicePath)
	}

	return nil, fmt.Errorf("failed to determine linux daemon type after checking for systemd and system v")
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

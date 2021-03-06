package osutil

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
)

const (
	systemctlExeName = "systemctl"
	serviceExeName   = "service"
	chkconfigExeName = "chkconfig"
	updatercdExeName = "update-rc.d"
)

var (
	systemctlExeDirPaths = []string{"/bin"}
	serviceExeDirPaths   = []string{
		"/sbin",
		"/usr/sbin",
	}
)

func IsSystemd() (systemctlPath string, ok bool) {
	systemctlPath, findErr := searchForExeInPaths(systemctlExeName, systemctlExeDirPaths)
	if findErr == nil {
		if _, systemctlExitCode, _ := RunDaemonCli(systemctlPath); systemctlExitCode == 0 {
			return systemctlPath, true
		}
	}

	return "", false
}

func IsSystemv() (servicePath string, isRedHat bool, whyNotSysV string, ok bool) {
	servicePath, err := searchForExeInPaths(serviceExeName, serviceExeDirPaths)
	if err != nil {
		return "", false, err.Error(), false
	}

	output, _, _ := RunDaemonCli(servicePath)
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

func ChkconfigPath() (string, error) {
	return searchForExeInPaths(chkconfigExeName, serviceExeDirPaths)
}

func UpdatercdPath() (string, error) {
	return searchForExeInPaths(updatercdExeName, serviceExeDirPaths)
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

func RunDaemonCli(exePath string, args ...string) (string, int, error) {
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

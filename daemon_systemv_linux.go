package cyberdaemon

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
)

const (
	pidFilePerm    = 0644
	PIDFilePathVar = "PID_FILE_PATH"
)

type systemvDaemonizer struct {
	logConfig LogConfig
}

func (o *systemvDaemonizer) RunUntilExit(application Application) error {
	// The 'PS1' environment variable will be empty / not set when
	// this is run non-interactively.
	if len(os.Getenv("PS1")) == 0 {
		// Only do native log things when running non-interactively.
		if o.logConfig.UseNativeLogger {
			log.SetOutput(os.Stderr)

			if o.logConfig.NativeLogFlags > 0 {
				originalLogFlags := log.Flags()
				log.SetFlags(o.logConfig.NativeLogFlags)
				defer log.SetFlags(originalLogFlags)
			}
		}

		// Check if init.d started us. If it did, then we need to
		// forkexec (AKA, start a new process and exit this one).
		// We do this because init.d expects the process to fork and
		// not block.
		//
		// Golang cannot fork because forking only provides the new
		// process with a single thread. The runtime needs more than
		// one thread to run - so that is not an option.
		if initdScriptPath, startedByInitd, err := isInitdOurParent(); startedByInitd {
			exePath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to get executable path when exec'ing daemon - %s", err.Error())
			}

			// TODO: Just use 'os.Args[0]' as the path?
			daemon := exec.Command(exePath, os.Args[1:]...)
			if o.logConfig.UseNativeLogger {
				// Set stderr of new process to the current
				// stderr so that input redirection will
				// be honored.
				daemon.Stderr = os.Stderr
			}

			// Either get the PID file from the init.d script,
			// or try a sane default.
			pidFilePath, findErr := pidFilePathFromInitdScript(initdScriptPath)
			if findErr != nil {
				pidFilePath = DefaultPidFilePath(path.Base(initdScriptPath))
			}

			pidFile, err := os.OpenFile(pidFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, pidFilePerm)
			if err != nil {
				return fmt.Errorf("failed to open pid file - %s", err.Error())
			}

			// Pass the PID file to the process as file
			// descriptor number 3 (per the 'ExtraFiles'
			// documentation).
			daemon.ExtraFiles = []*os.File{pidFile}

			err = daemon.Start()
			if err != nil {
				return fmt.Errorf("failed to exec daemon process - %s", err.Error())
			}

			_, err = pidFile.WriteString(fmt.Sprintf("%d\n", daemon.Process.Pid))
			if err != nil {
				return fmt.Errorf("failed to write daemon pid to pid file - %s", err.Error())
			}

			// Exit.
			// TODO: Should we just os.Exit() here? Can we trust
			//  the implementer to properly structure their code?
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to determine if init.d started the process - %s", err.Error())
		}

		// Now we are running as a daemon. File descriptor 3 will be
		// the PID file.
		pidFile := os.NewFile(3, "")
		if pidFile == nil {
			return fmt.Errorf("failed to open pid file passed to daemon - file descriptor might of been crushed?")
		}
		defer func() {
			// Debian does not remove the PID file for us
			// (perhaps other OSes might not either).
			// Delete the contents of the file so the OS
			// knows the process stopped cleanly.
			pidFile.Seek(0, 0)
			pidFile.Truncate(0)
			pidFile.Close()
		}()
	}

	err := application.Start()
	if err != nil {
		return err
	}

	interruptsAndTerms := make(chan os.Signal)
	signal.Notify(interruptsAndTerms, os.Interrupt, syscall.SIGTERM)
	<-interruptsAndTerms
	signal.Stop(interruptsAndTerms)

	return application.Stop()
}

func isInitdOurParent() (scriptPath string, isInitd bool, err error) {
	pid := os.Getppid()
	for i := 0; i < 5; i++ {
		if i != 0 {
			pid, err = manualParentPid(pid)
			if err != nil {
				return "", false, err
			}
			if pid == 0 {
				return "", false, nil
			}
		}

		scriptPath, isInitd, err = isPidInitd(pid)
		if err != nil {
			return "", false, err
		}

		if isInitd {
			return scriptPath, true, nil
		}
	}

	return "", false, nil
}

func manualParentPid(pid int) (int, error) {
	contents, err := tinyRead(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}

	split := strings.Split(contents, " ")
	if len(split) < 4 {
		return 0, fmt.Errorf("stat file for pid %d has too few elements to get parent pid", pid)
	}

	parentPid, err := strconv.ParseUint(split[3], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to convert parent pid for pid %d - %s", pid, err.Error())
	}

	return int(parentPid), nil
}

func isPidInitd(pid int) (scriptPath string, isInitd bool, err error) {
	// The 'cmdline' file contains the command line arguments of the
	// process separated by null. We can derive whether the process
	// is init.d by looking at its cmdline.
	cmdlineContents, err := tinyRead(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return "", false, fmt.Errorf("failed to read cmdline for process %d - %s", pid, err.Error())
	}

	// File contents are null terminated.
	splitContents := strings.Split(string(cmdlineContents), "\x00")
	for i := range splitContents {
		switch i {
		case 0, 1, 2, 3:
			if strings.HasPrefix(splitContents[i], "/etc/init.d/") {
				return splitContents[i], true, nil
			}
		}
	}

	return "", false, nil
}

func tinyRead(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	contents, err := ioutil.ReadAll(io.LimitReader(f, 100000))
	if err != nil {
		return "", err
	}

	return string(contents), nil
}

func pidFilePathFromInitdScript(scriptPath string) (string, error) {
	f, err := os.Open(scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to open init.d script for parsing - %s", err.Error())
	}
	defer f.Close()

	scanner := bufio.NewScanner(io.LimitReader(f, 100000))
	prefix := PIDFilePathVar + "="

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, prefix) {
			return strings.Trim(strings.TrimPrefix(line, prefix), "\"'"), nil
		}
	}

	err = scanner.Err()
	if err != nil {
		return "", fmt.Errorf("failed to scan init.d script - %s", err.Error())
	}

	return "", fmt.Errorf("failed to find pid file path ('%s') in init.d script", prefix)
}

func newSystemvDaemonizer(logConfig LogConfig) Daemonizer {
	return &systemvDaemonizer{
		logConfig: logConfig,
	}
}

// PID file path example: '/var/run/mydaemon.pid'.
func DefaultPidFilePath(serviceName string) string {
	return fmt.Sprintf("/var/run/%s.pid", serviceName)
}

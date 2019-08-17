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
	// systemvTemplate is a System V init.d script template that
	// contains placeholders for customizable options. This template
	// is based on '/etc/init.d/sshd' from CentOS 6.10.
	//
	// Credit to the OpenSSH team et al:
	//  Taken from: https://github.com/openssh/openssh-portable/blob/79226e5413c5b0fda3511351a8511ff457e306d8/contrib/redhat/sshd.init
	//  Commit: 79226e5413c5b0fda3511351a8511ff457e306d8
	//
	// TODO: Support additional 'Required' and 'Should' statements,
	//  such as '$network'.
	// TODO: Support run level customization.
	systemvTemplate = `#!/bin/bash
#
# This file is based on '/etc/init.d/sshd' from the OpenSSH project.
# See https://github.com/openssh/openssh-portable/blob/master/LICENCE
# for details.

### BEGIN INIT INFO
# Provides: ` + serviceNamePlaceholder + `
# Required-Start: $local_fs $syslog
# Required-Stop: $local_fs $syslog
# Should-Start: $syslog
# Should-Stop: $syslog
# Default-Start: 2 3 4 5
# Default-Stop: 0 1 6
# Short-Description: ` + shortDescriptionPlaceholder + `
# Description:       ` + descriptionPlaceholder + `
### END INIT INFO

IS_REDHAT=""
if [ -f "/etc/redhat-release" ]
then
    IS_REDHAT=true
    . /etc/rc.d/init.d/functions
else
    . /lib/lsb/init-functions
    export PATH="${PATH:+$PATH:}/usr/sbin:/sbin"
    if init_is_upstart
    then
        exit 1
    fi
fi

PROGRAM_NAME='` + serviceNamePlaceholder + `'
PROGRAM_PATH='` + exePathPlaceholder + `'
ARGUMENTS=''
RUN_AS='` + runAsPlaceholder + `'
if [ -z "${RUN_AS}" ]
then
	RUN_AS='root'
fi
` + pidFilePathVar + `='` + pidFilePathPlaceholder + `'

runlevel=$(set -- $(runlevel); eval "echo \$$#" )

start() {
    [ -x "${PROGRAM_PATH}" ] || exit 5
    if [ -n "${IS_REDHAT}" ]
    then
        echo -n $"Starting $PROGRAM_NAME: "
    else
        check_dev_null
        log_daemon_msg "Starting ${SHORT_DESCRIPTION}" "${PROGRAM_NAME}" || true
    fi
    local logFilePath="` + logFilePathPlaceholder + `"
    if [ -z "${logFilePath}" ]
    then
        logFilePath=/dev/null
    else
        mkdir -p -m 0700 "${logFilePath%/*}"
        chown -R "${RUN_AS}:${RUN_AS}" "${logFilePath%/*}"
    fi
    local r=0
    if [ "${RUN_AS}" == "root" ]
    then
        $PROGRAM_PATH $ARGUMENTS 2> "$logFilePath"
    else
        touch $PID_FILE_PATH
        chown ${RUN_AS}:${RUN_AS} $PID_FILE_PATH
        su $RUN_AS -c "$PROGRAM_PATH $ARGUMENTS 2> '$logFilePath'"
    fi
    r=$?
    if [ -n "${IS_REDHAT}" ]
    then
        if [ $r -eq 0 ]
        then
            success
        else
            failure
        fi
        echo
    else
        if [ $r -eq 0 ]
        then
            log_end_msg 0 || true
        else
            log_end_msg 1 || true
        fi
    fi
    return $r
}

stop() {
    if [ -n "${IS_REDHAT}" ]
    then
        echo -n $"Stopping $PROGRAM_NAME: "
        killproc -p $PID_FILE_PATH $PROGRAM_PATH
        echo
    else
        log_daemon_msg "Stopping ${SHORT_DESCRIPTION}" "${PROGRAM_NAME}" || true
        if start-stop-daemon --stop --pidfile ${PID_FILE_PATH}
        then
            log_end_msg 0 || true
        else
            log_end_msg 1 || true
        fi
    fi
    # if we are in halt or reboot runlevel kill all running sessions
    # so the TCP connections are closed cleanly
    if [ "x$runlevel" = x0 -o "x$runlevel" = x6 ]; then
        trap '' TERM
        pkill $PROGRAM_NAME 2>/dev/null
        trap TERM
    fi
    return $?
}

rh_status() {
    status -p $PID_FILE_PATH $PROGRAM_NAME
}

rh_status_q() {
    rh_status >/dev/null 2>&1
}

run_by_init() {
    ([ "$previous" ] && [ "$runlevel" ]) || [ "$runlevel" = S ]
}

check_dev_null() {
    if [ ! -c /dev/null ]
    then
        if [ "$1" = log_end_msg ]
        then
            log_end_msg 1 || true
        fi
        if ! run_by_init
        then
            log_action_msg "/dev/null is not a character device!" || true
        fi
        exit 1
    fi
}

case "$1" in
    start)
        if [ -n "${IS_REDHAT}" ]
        then
            rh_status_q && exit 0
        else
            start-stop-daemon --status --pidfile ${PID_FILE_PATH} && exit 0
        fi
        start
        ;;
    stop)
        if [ -n "${IS_REDHAT}" ]
        then
            if ! rh_status_q; then
                exit 0
            fi
        else
            start-stop-daemon --status --pidfile ${PID_FILE_PATH} || exit 0
        fi
        stop
        ;;
    restart)
        stop
        start
        exit $?
        ;;
    reload|force-reload)
        if [ -n "${IS_REDHAT}" ]
        then
            echo -n $"Reloading $PROGRAM_NAME: "
            killproc -p $PID_FILE_PATH $PROGRAM_PATH -HUP
            r=$?
            echo
            exit $r
        else
            log_daemon_msg "Reloading ${SHORT_DESCRIPTION}" "${PROGRAM_NAME}" || true
            if start-stop-daemon --signal HUP --pidfile ${PID_FILE_PATH} --stop; then
                log_end_msg 0 || true
            else
                log_end_msg 1 || true
            fi
        fi
        ;;
    condrestart|try-restart)
        if [ -n "${IS_REDHAT}" ]
        then
            rh_status_q || exit 0
            stop
            start
            exit $?
        else
            start-stop-daemon --status --pidfile ${PID_FILE_PATH} && exit 0
            log_daemon_msg "Restarting ${SHORT_DESCRIPTION}" "${PROGRAM_NAME}" || true
            r=0
            start-stop-daemon --stop --quiet --retry 30 --pidfile ${PID_FILE_PATH} || r="$?"
            case $r in
                0)
                # old daemon stopped
                check_dev_null log_end_msg
                if start
                then
                    log_end_msg 0 || true
                else
                    log_end_msg 1 || true
                fi
                ;;
                1)
                # daemon not running
                log_progress_msg "(not running)" || true
                log_end_msg 0 || true
                ;;
                *)
                # failed to stop
                log_progress_msg "(failed to stop)" || true
                log_end_msg 1 || true
                ;;
            esac
        fi
        ;;
    status)
        if [ -n "${IS_REDHAT}" ]
        then
            rh_status
            exit $?
        else
            status_of_proc -p ${PID_FILE_PATH} "${PROGRAM_PATH}" "${PROGRAM_NAME}" && exit 0 || exit $?
        fi
        ;;
    *)
        if [ -n "${IS_REDHAT}" ]
        then
            echo $"Usage: $0 {start|stop|restart|reload|force-reload|condrestart|try-restart|status}"
        else
            log_action_msg "Usage: $0 {start|stop|reload|force-reload|restart|try-restart|status}"
        fi
        exit 2
esac
exit $?
`

	serviceNamePlaceholder      = placeholderDelim + "NAME" + placeholderDelim
	shortDescriptionPlaceholder = placeholderDelim + "SHORT_DESCRIPTION" + placeholderDelim
	descriptionPlaceholder      = placeholderDelim + "DESCRIPTION" + placeholderDelim
	exePathPlaceholder          = placeholderDelim + "EXE_PATH" + placeholderDelim
	logFilePathPlaceholder      = placeholderDelim + "LOG_FILE_PATH" + placeholderDelim
	pidFilePathPlaceholder      = placeholderDelim + "PID_FILE_PATH" + placeholderDelim
	runAsPlaceholder            = placeholderDelim + "RUN_AS" + placeholderDelim
	placeholderDelim            = "^"

	serviceExeName   = "service"
	chkconfigExeName = "chkconfig"
	updatercdExeName = "update-rc.d"
	pidFilePerm      = 0644
	pidFilePathVar   = "PID_FILE_PATH"
)

var (
	serviceExeDirPaths = []string{
		"/sbin",
		"/usr/sbin",
	}
)

type systemvController struct {
	servicePath  string
	daemonID     string
	initContents string
	initFilePath string
	startType    StartType
	isRedHat     bool
	chkconfig    string
	updatercd    string
	logConfig    LogConfig
}

func (o *systemvController) Status() (Status, error) {
	initInfo, statErr := os.Stat(o.initFilePath)
	if statErr != nil || initInfo.IsDir() {
		return NotInstalled, nil
	}

	_, exitCode, statusErr := runDaemonCli(o.servicePath, o.daemonID, "status")
	if statusErr != nil {
		switch exitCode {
		case 3:
			return Stopped, nil
		case 1:
			return StoppedDead, nil
		}
	}

	if exitCode == 0 {
		return Running, nil
	}

	return Unknown, nil
}

func (o *systemvController) Install() error {
	err := ioutil.WriteFile(o.initFilePath, []byte(o.initContents), 0755)
	if err != nil {
		return fmt.Errorf("failed to write init.d script file - %s", err.Error())
	}

	switch o.startType {
	case StartImmediately:
		err := o.Start()
		if err != nil {
			return err
		}
		fallthrough
	case StartOnLoad:
		var err error
		if o.isRedHat {
			_, _, err = runDaemonCli(o.chkconfig, o.daemonID, "on",)
		} else {
			_, _, err = runDaemonCli(o.updatercd, o.daemonID, "defaults",)
		}
		if err != nil {
			return err
		}
	case ManualStart:
		// By default, Linux sets system v services to auto start after
		// installation completes. We need to tell the OS to disable
		// auto start when the user requests that the daemon
		// only start manually.
		if o.isRedHat {
			_, _, err = runDaemonCli(o.chkconfig, o.daemonID, "off",)
		} else {
			_, _, err = runDaemonCli(o.updatercd, o.daemonID, "disable",)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *systemvController) Uninstall() error {
	// Try to stop the daemon. Ignore any errors because it might be
	// stopped already, or the stop failed (which there is nothing
	// we can do.
	o.Stop()

	return os.Remove(o.initFilePath)
}

func (o *systemvController) Start() error {
	_, _, err := runDaemonCli(o.servicePath, o.daemonID, "start")
	if err != nil {
		return err
	}

	return nil
}

func (o *systemvController) Stop() error {
	_, _, err := runDaemonCli(o.servicePath, o.daemonID, "stop")
	if err != nil {
		return err
	}

	return nil
}

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
				pidFilePath = defaultPidFilePath(path.Base(initdScriptPath))
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
	prefix := pidFilePathVar + "="

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

func newSystemvController(exePath string, config ControllerConfig, serviceExePath string, isRedHat bool) (*systemvController, error) {
	var logFilePath string

	if config.LogConfig.UseNativeLogger {
		// Log file path example: '/var/log/mydaemon/mydaemon.log'.
		// TODO: Use a friendly name for the log directory
		//  and file name.
		logFilePath = path.Join("/var/log", config.DaemonID, config.DaemonID+ ".log")
	}

	replacer := strings.NewReplacer(serviceNamePlaceholder, config.DaemonID,
		shortDescriptionPlaceholder, fmt.Sprintf("%s daemon.", config.DaemonID),
		descriptionPlaceholder, config.Description,
		exePathPlaceholder, exePath,
		runAsPlaceholder, config.RunAs,
		logFilePathPlaceholder, logFilePath,
		pidFilePathPlaceholder, defaultPidFilePath(config.DaemonID))

	script := replacer.Replace(systemvTemplate)
	if strings.Contains(script, placeholderDelim) {
		return nil, fmt.Errorf("failed to replace all placeholders in daemon init.d script")
	}

	var enableCliToolPath string
	var err error
	if isRedHat {
		enableCliToolPath, err = searchForExeInPaths(chkconfigExeName, serviceExeDirPaths)
	} else {
		enableCliToolPath, err = searchForExeInPaths(updatercdExeName, serviceExeDirPaths)
	}
	if err != nil {
		return nil, err
	}

	return &systemvController{
		servicePath:  serviceExePath,
		daemonID:     config.DaemonID,
		logConfig:    config.LogConfig,
		initContents: script,
		initFilePath: fmt.Sprintf("/etc/init.d/%s", config.DaemonID),
		startType:    config.StartType,
		isRedHat:     isRedHat,
		chkconfig:    enableCliToolPath,
		updatercd:    enableCliToolPath,
	}, nil
}

// PID file path example: '/var/run/mydaemon.pid'.
func defaultPidFilePath(serviceName string) string {
	return fmt.Sprintf("/var/run/%s.pid", serviceName)
}

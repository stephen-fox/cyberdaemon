package control

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/stephen-fox/cyberdaemon"
	"github.com/stephen-fox/cyberdaemon/internal/osutil"
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
ARGUMENTS='` + argumentsPlaceholder + `'
RUN_AS='` + runAsPlaceholder + `'
if [ -z "${RUN_AS}" ]
then
	RUN_AS='root'
fi
` + cyberdaemon.PIDFilePathVar + `='` + pidFilePathPlaceholder + `'

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
	argumentsPlaceholder        = placeholderDelim + "ARGUMENTS" + placeholderDelim
	logFilePathPlaceholder      = placeholderDelim + "LOG_FILE_PATH" + placeholderDelim
	pidFilePathPlaceholder      = placeholderDelim + "PID_FILE_PATH" + placeholderDelim
	runAsPlaceholder            = placeholderDelim + "RUN_AS" + placeholderDelim
	placeholderDelim            = "^"
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
	logConfig    cyberdaemon.LogConfig
}

func (o *systemvController) Status() (Status, error) {
	initInfo, statErr := os.Stat(o.initFilePath)
	if statErr != nil || initInfo.IsDir() {
		return NotInstalled, nil
	}

	_, exitCode, statusErr := osutil.RunDaemonCli(o.servicePath, o.daemonID, "status")
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
			_, _, err = osutil.RunDaemonCli(o.chkconfig, o.daemonID, "on",)
		} else {
			_, _, err = osutil.RunDaemonCli(o.updatercd, o.daemonID, "defaults",)
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
			_, _, err = osutil.RunDaemonCli(o.chkconfig, o.daemonID, "off",)
		} else {
			_, _, err = osutil.RunDaemonCli(o.updatercd, o.daemonID, "disable",)
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
	_, _, err := osutil.RunDaemonCli(o.servicePath, o.daemonID, "start")
	if err != nil {
		return err
	}

	return nil
}

func (o *systemvController) Stop() error {
	_, _, err := osutil.RunDaemonCli(o.servicePath, o.daemonID, "stop")
	if err != nil {
		return err
	}

	return nil
}

func newSystemvController(config ControllerConfig, serviceExePath string, isRedHat bool) (*systemvController, error) {
	err := config.Validate()
	if err != nil {
		return nil, err
	}

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
		exePathPlaceholder, config.ExePath,
		argumentsPlaceholder, config.argumentsAsString(),
		runAsPlaceholder, config.RunAs,
		logFilePathPlaceholder, logFilePath,
		pidFilePathPlaceholder, defaultPidFilePath(config.DaemonID))

	script := replacer.Replace(systemvTemplate)
	if strings.Contains(script, placeholderDelim) {
		return nil, fmt.Errorf("failed to replace all placeholders in daemon init.d script")
	}

	var enableCliToolPath string
	if isRedHat {
		enableCliToolPath, err = osutil.ChkconfigPath()
	} else {
		enableCliToolPath, err = osutil.UpdatercdPath()
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

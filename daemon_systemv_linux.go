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
	"strings"
	"syscall"
	"time"
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

PROGRAM_NAME="` + serviceNamePlaceholder + `"
PROGRAM_PATH="` + exePathPlaceholder + `"
ARGUMENTS=""
RUN_AS=""
if [ -z "${RUN_AS}" ]
then
	RUN_AS="root"
fi
PID_FILE_PATH="` + pidFilePathPlaceholder + `"

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
    mkdir -p "${PID_FILE_PATH%/*}"
    chown -R "${RUN_AS}:${RUN_AS}" "${PID_FILE_PATH%/*}"
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
        su $RUN_AS -c "$PROGRAM_PATH $ARGUMENTS  2> '$logFilePath'"
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
	placeholderDelim            = "^"

	pidFilePerm      = 0644
	runAsDaemonMagic = "CYBERDAEMON_RESERVED_DAEMONIZE_R3OGMOJ405FMHT"
)

type systemvDaemon struct {
	daemonId     string
	logConfig    LogConfig
	initContents string
	initFilePath string
	pidFilePath  string
}

func (o *systemvDaemon) Status() (Status, error) {
	initInfo, statErr := os.Stat(o.initFilePath)
	if statErr != nil || initInfo.IsDir() {
		return NotInstalled, nil
	}

	_, exitCode, statusErr := runServiceCommand(o.daemonId, "status")
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

func (o *systemvDaemon) Install() error {
	return ioutil.WriteFile(o.initFilePath, []byte(o.initContents), 0755)
}

func (o *systemvDaemon) Uninstall() error {
	// TODO: Should we do this before uninstalling other daemons?
	o.Stop()

	return os.Remove(o.initFilePath)
}

func (o *systemvDaemon) Start() error {
	_, _, err := runServiceCommand(o.daemonId, "start")
	if err != nil {
		return err
	}

	return nil
}

func (o *systemvDaemon) Stop() error {
	_, _, err := runServiceCommand(o.daemonId, "stop")
	if err != nil {
		return err
	}

	return nil
}

func (o *systemvDaemon) RunUntilExit(logic ApplicationLogic) error {
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

		// TODO: Super hack. We need to fork so we can do things
		//  like check the PID file, check configuration, etc. and
		//  then go to the background. However, go cannot fork...
		//  Instead, we can use exec to start a new process.
		//  The problem with that is the code needs a way to
		//  determine if it should start a new process, or continue
		//  running as-is.
		//  Without "some state", the code will just exec itself
		//  forever (in other words, it needs to know when it is
		//  the exec'd process).
		//  Using the PID file is not really an option without
		//  writing something that is not a PID to the file.
		//  I do not want to risk reinterpretation by ps, or
		//  another utility.
		//  The remaining solutions are:
		//   - set a reserved env. variable on the new process
		//   - pass a reserved value to the new process via stdin
		//   - determine parent process / group?
		//  Originally, I felt that using an environment variable
		//  was the least complicated and most minimal hack. After
		//  further consideration, I feel that using stdin is
		//  cleaner, and has the least probability of being
		//  exploited. It is slower, and more brittle, but reading
		//  a known / reserved environment variable seems more
		//  easily exploitable to me.
		if isDaemon, _, _ := isRunningAsDaemon(); !isDaemon {
			exePath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to get executable path when exec'ing daemon - %s", err.Error())
			}

			daemon := exec.Command(exePath, os.Args[1:]...)
			if o.logConfig.UseNativeLogger {
				daemon.Stderr = os.Stderr
			}

			pipe, err := daemon.StdinPipe()
			if err != nil {
				return fmt.Errorf("failed to open pipe to daemon process's stdin - %s", err.Error())
			}

			writeErr := make(chan error)
			go func(errs chan error, writer io.WriteCloser) {
				_, err = writer.Write([]byte(runAsDaemonMagic + "\n"))
				writer.Close()
				if err != nil {
					errs <- err
				} else {
					errs <- nil
				}
			}(writeErr, pipe)

			err = daemon.Start()
			if err != nil {
				return fmt.Errorf("failed to exec daemon process - %s", err.Error())
			}

			err = <-writeErr
			if err != nil {
				return fmt.Errorf("failed to write daemon run command to daemon process's stdin - %s", err.Error())
			}

			// Exit.
			// TODO: Should we just os.Exit() here? Can we trust
			//  the implementer to properly structure their code?
			return nil
		}

		// Now we are running as a daemon.
		err := ioutil.WriteFile(o.pidFilePath, []byte(fmt.Sprintf("%d\n", os.Getpid())), pidFilePerm)
		if err != nil {
			return fmt.Errorf("failed to write PID to PID file when daemonized - %s", err.Error())
		}
		defer os.Remove(o.pidFilePath)
	}

	err := logic.Start()
	if err != nil {
		return err
	}

	interruptsAndTerms := make(chan os.Signal)
	signal.Notify(interruptsAndTerms, os.Interrupt, syscall.SIGTERM)
	<-interruptsAndTerms
	signal.Stop(interruptsAndTerms)

	return logic.Stop()
}

func isRunningAsDaemon() (bool, time.Duration, error) {
	start := time.Now()

	type r struct {
		isDaemon bool
		err      error
	}

	results := make(chan r, 1)
	go func(results chan r) {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			results <- r{
				isDaemon: scanner.Text() == runAsDaemonMagic,
			}
			return
		}

		results <- r{
			err: scanner.Err(),
		}
	}(results)

	timeout := time.NewTimer(50 * time.Millisecond)
	select {
	case <-timeout.C:
		return false, time.Since(start), nil
	case res := <-results:
		timeout.Stop()
		return res.isDaemon, time.Since(start), res.err
	}
}

func newSystemvDaemon(exePath string, config Config) (*systemvDaemon, error) {
	var logFilePath string

	if config.LogConfig.UseNativeLogger {
		// Log file path example: '/var/log/mydaemon/mydaemon.log'.
		// TODO: Use a friendly name for the log directory
		//  and file name.
		logFilePath = path.Join("/var/log", config.DaemonId, config.DaemonId + ".log")
	}

	// PID file path example: '/var/run/mydaemon/mydaemon.pid'.
	pidFilePath := fmt.Sprintf("/var/run/%s/%s.pid", config.DaemonId, config.DaemonId)

	replacer := strings.NewReplacer(serviceNamePlaceholder, config.DaemonId,
		shortDescriptionPlaceholder, fmt.Sprintf("%s daemon.", config.DaemonId),
		descriptionPlaceholder, config.Description,
		exePathPlaceholder, exePath,
		logFilePathPlaceholder, logFilePath,
		pidFilePathPlaceholder, pidFilePath)

	script := replacer.Replace(systemvTemplate)
	if strings.Contains(script, placeholderDelim) {
		return nil, fmt.Errorf("failed to replace all placeholders in daemon init.d script")
	}

	return &systemvDaemon{
		daemonId:     config.DaemonId,
		logConfig:    config.LogConfig,
		initContents: script,
		initFilePath: fmt.Sprintf("/etc/init.d/%s", config.DaemonId),
		pidFilePath:  pidFilePath,
	}, nil
}

func runServiceCommand(args ...string) (string, int, error) {
	// TODO: Try to find 'service' in '/sbin/' or '/usr/sbin/'.
	servicePath := "service"

	s := exec.Command(servicePath, args...)
	output, err := s.CombinedOutput()
	trimmedOutput := strings.TrimSpace(string(output))
	if err != nil {
		return trimmedOutput, s.ProcessState.ExitCode(),
			fmt.Errorf("failed to execute '%s %s' - %s - output: %s",
				servicePath, args, err.Error(), trimmedOutput)
	}

	return trimmedOutput, s.ProcessState.ExitCode(), nil
}

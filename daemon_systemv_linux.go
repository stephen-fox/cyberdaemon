package cyberdaemon

import (
	"bytes"
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
	// TODO: Support additional 'Required' and 'Should' statements,
	//  such as '$network'.
	// TODO: Support run level customization.
	// systemvRedHatTemplate is a System V init.d script template
	// that contains placeholders for customizable options. This
	// template is based on '/etc/init.d/sshd' from CentOS 6.10.
	// Credit to the OpenSSH team et al:
	//  Taken from: https://github.com/openssh/openssh-portable/blob/master/contrib/redhat/sshd.init
	//  Commit: 79226e5413c5b0fda3511351a8511ff457e306d8
	systemvRedHatTemplate =`#!/bin/bash
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

# source function library
. /etc/rc.d/init.d/functions

RETVAL=0
PROGRAM_NAME="` + serviceNamePlaceholder + `"
PROGRAM_PATH="` + exePathPlaceholder + `"
ARGUMENTS=""
RUN_AS=""
LOG_FILE_PATH="` + logFilePathPlaceholder + `"
PID_FILE="` + pidFilePathPlaceholder + `"

runlevel=$(set -- $(runlevel); eval "echo \$$#" )

start()
{
	[ -x $PROGRAM_PATH ] || exit 5
	echo -n $"Starting $PROGRAM_NAME: "
	if [ -z "${RUN_AS}" ] || [ "${RUN_AS}" == "root" ]
	then
		$PROGRAM_PATH $ARGUMENTS && success || failure
	else
		su ${RUN_AS} -c "$PROGRAM_PATH $ARGUMENTS" && success || failure
	fi
	RETVAL=$?
	echo
	return $RETVAL
}

stop()
{
	echo -n $"Stopping $PROGRAM_NAME: "
	killproc -p $PID_FILE $PROGRAM_PATH
	RETVAL=$?
	# if we are in halt or reboot runlevel kill all running sessions
	# so the TCP connections are closed cleanly
	if [ "x$runlevel" = x0 -o "x$runlevel" = x6 ] ; then
	    trap '' TERM
	    killall $PROGRAM_NAME 2>/dev/null
	    trap TERM
	fi
	echo
}

reload()
{
	echo -n $"Reloading $PROGRAM_NAME: "
	killproc -p $PID_FILE $PROGRAM_PATH -HUP
	RETVAL=$?
	echo
}

restart() {
	stop
	start
}

force_reload() {
	restart
}

rh_status() {
	status -p $PID_FILE $PROGRAM_NAME
}

rh_status_q() {
	rh_status >/dev/null 2>&1
}

case "$1" in
	start)
		rh_status_q && exit 0
		start
		;;
	stop)
		if ! rh_status_q; then
			exit 0
		fi
		stop
		;;
	restart)
		restart
		;;
	reload)
		rh_status_q || exit 7
		reload
		;;
	force-reload)
		force_reload
		;;
	condrestart|try-restart)
		rh_status_q || exit 0
		;;
	status)
		rh_status
		RETVAL=$?
		;;
	*)
		echo $"Usage: $0 {start|stop|restart|reload|force-reload|condrestart|try-restart|status}"
		RETVAL=2
esac
exit $RETVAL`

	serviceNamePlaceholder      = placeholderDelim + "NAME" + placeholderDelim
	shortDescriptionPlaceholder = placeholderDelim + "SHORT_DESCRIPTION" + placeholderDelim
	descriptionPlaceholder      = placeholderDelim + "DESCRIPTION" + placeholderDelim
	exePathPlaceholder          = placeholderDelim + "EXE_PATH" + placeholderDelim
	logFilePathPlaceholder      = placeholderDelim + "LOG_FILE_PATH" + placeholderDelim
	pidFilePathPlaceholder      = placeholderDelim + "PID_FILE_PATH" + placeholderDelim
	placeholderDelim            = "^"

	pidFilePerm    = 0644
	runAsDaemonEnv = "CYBERDAEMON_RESERVED_DAEMONIZE_R3OGMOJ405FMHT"
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

		if pidFile, openErr := os.Open(o.pidFilePath); openErr == nil {
			raw, _ := ioutil.ReadAll(io.LimitReader(pidFile, 1000))
			pidFile.Close()
			if isAlreadyRunning, pid := isPidRunning(raw); isAlreadyRunning {
				return fmt.Errorf("daemon is already running as PID %d", pid)
			}
		}

		// TODO: Super hack. We need to fork so we can do things like
		//  check the PID file, check configuration, etc and then go to
		//  the background. However, go cannot fork... Instead, we can
		//  use exec to start a new process. The problem with that is
		//  the new process needs to know when to do the exec.
		//  Otherwise, it will just exec itself forever (in other
		//  words, it needs to know when it is the exec'd process).
		//  Using the PID file is not really an option without writing
		//  something that is not a PID to the file. I do not want to
		//  risk reinterpretation by ps, or another utility.
		//  The next-least-hackiest solution is to pass en environment
		//  variable. Using stdin is a possibility as well, but that
		//  is more complicated and error prone (all while still being
		//  a big 'ol hack). So, a reserved environment variable it is!
		if _, hasEnv := os.LookupEnv(runAsDaemonEnv); !hasEnv {
			exePath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to get executable path when restarting as daemon - %s", err.Error())
			}

			daemon := exec.Command(exePath, os.Args[1:]...)
			daemon.Env = append(os.Environ(), fmt.Sprintf("%s=true", runAsDaemonEnv))

			err = daemon.Start()
			if err != nil {
				return fmt.Errorf("failed to exec daemon binary when restarting as daemon - %s", err.Error())
			}

			// Exit.
			// TODO: Should we just os.Exit() here? Can we trust
			//  the implementer to properly structure their code?
			return nil
		}

		os.Unsetenv(runAsDaemonEnv)

		// Now we are running as a daemon.
		err := ioutil.WriteFile(o.pidFilePath, []byte(fmt.Sprintf("%d\n", os.Getpid())), pidFilePerm)
		if err != nil {
			return fmt.Errorf("failed to write PID to PID file when daemonized - %s", err.Error())
		}
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

func isPidRunning(raw []byte) (bool, uint64) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return false, 0
	}

	pid, err := strconv.ParseUint(string(raw), 10, 32)
	if err != nil {
		return false, 0
	}

	info, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	if err != nil || !info.IsDir() {
		return false, pid
	}

	return true, pid
}

func NewDaemon(config Config) (Daemon, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}

	var logFilePath string

	if config.LogConfig.UseNativeLogger {
		// TODO: Use a friendly name for the log directory
		//  and file name.
		logFilePath = path.Join("/var/log", config.DaemonId, config.DaemonId + ".log")
	}

	pidFilePath := fmt.Sprintf("/var/run/%s.pid", config.DaemonId)

	replacer := strings.NewReplacer(serviceNamePlaceholder, config.DaemonId,
		shortDescriptionPlaceholder, fmt.Sprintf("%s daemon.", config.DaemonId),
		descriptionPlaceholder, config.Description,
		exePathPlaceholder, exePath,
		logFilePathPlaceholder, logFilePath,
		pidFilePathPlaceholder, pidFilePath)

	// TODO: Debian support.
	script := replacer.Replace(systemvRedHatTemplate)
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

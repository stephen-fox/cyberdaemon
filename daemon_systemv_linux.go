package cyberdaemon

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"
)

const (
	// systemvTemplate represents a template for a System V init script.
	// This template is based on work by Felix H. "fhd" Dahlke et al.
	//  from: https://github.com/fhd/init-script-template/blob/master/template
	//  commit: 5bc40ef4814128f4358af09e2295004e8ef9b30d
	systemvTemplate = `#!/bin/sh
### BEGIN INIT INFO
# Provides:          ` + serviceNamePlaceholder + `
# Required-Start:    $remote_fs $syslog
# Required-Stop:     $remote_fs $syslog
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: ` + shortDescriptionPlaceholder + `
# Description:       ` + descriptionPlaceholder + `
### END INIT INFO

dir="` + workDirPathPlaceholder + `"
cmd="` + commandPlaceholder + `"
user=""

name="$(basename $0)"
pid_file="/var/run/$name.pid"
log_file_path="` + logFilePathPlaceholder + `"

get_pid() {
    cat "$pid_file"
}

is_running() {
    [ -f "$pid_file" ] && ps -p $(get_pid) > /dev/null 2>&1
}

case "$1" in
    start)
    if is_running; then
        echo "Already started"
    else
        echo "Starting $name"
        cd "$dir"
        if [ -z "$user" ]; then
            sudo $cmd 2>> "$log_file_path" &
        else
            sudo -u "$user" $cmd 2>> "$log_file_path" &
        fi
        echo $! > "$pid_file"
        if ! is_running; then
            echo "Unable to start, see $log_file_path"
            exit 1
        fi
    fi
    ;;
    stop)
    if is_running; then
        echo -n "Stopping $name..."
        kill $(get_pid)
        for i in 1 2 3 4 5 6 7 8 9 10
        do
            if ! is_running; then
                break
            fi

            echo -n "."
            sleep 1
        done
        echo

        if is_running; then
            echo "Not stopped; may still be shutting down or shutdown may have failed"
            exit 1
        else
            echo "Stopped"
            if [ -f "$pid_file" ]; then
                rm "$pid_file"
            fi
        fi
    else
        echo "Not running"
    fi
    ;;
    restart)
    $0 stop
    if is_running; then
        echo "Unable to stop, will not attempt to start"
        exit 1
    fi
    $0 start
    ;;
    status)
    if is_running; then
        echo "` + string(Running) + `"
    else
        echo "` + string(Stopped) + `"
        exit 1
    fi
    ;;
    *)
    echo "Usage: $0 {start|stop|restart|status}"
    exit 1
    ;;
esac

exit 0
`
	serviceNamePlaceholder      = placeholderDelim + "NAME" + placeholderDelim
	shortDescriptionPlaceholder = placeholderDelim + "SHORT_DESCRIPTION" + placeholderDelim
	descriptionPlaceholder      = placeholderDelim + "DESCRIPTION" + placeholderDelim
	commandPlaceholder          = placeholderDelim + "COMMAND" + placeholderDelim
	workDirPathPlaceholder      = placeholderDelim + "WORKING_DIRECTORY" + placeholderDelim
	logFilePathPlaceholder      = placeholderDelim + "LOG_FILE_PATH" + placeholderDelim
	placeholderDelim            = "^"

	notInstalledSuffix = ": unrecognized service"
)

type systemvDaemon struct {
	config       Config
	initContents string
	initFilePath string
	logDirPath   string
}

func (o *systemvDaemon) Status() (Status, error) {
	initInfo, statErr := os.Stat(o.initFilePath)
	if statErr != nil || initInfo.IsDir() {
		return NotInstalled, nil
	}

	output, err := runServiceCommand(o.config.DaemonId, "status")
	if err != nil {
		if strings.HasSuffix(output, Stopped.String()) {
			return Stopped, nil
		}
		return Unknown, err
	}

	switch output {
	case Running.String():
		return Running, nil
	case Stopped.String():
		return Stopped, nil
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
	_, err := runServiceCommand(o.config.DaemonId, "start")
	if err != nil {
		return err
	}

	return nil
}

func (o *systemvDaemon) Stop() error {
	_, err := runServiceCommand(o.config.DaemonId, "stop")
	if err != nil {
		return err
	}

	return nil
}

func (o *systemvDaemon) RunUntilExit(logic ApplicationLogic) error {
	// The 'PS1' environment variable will be empty / not set when
	// this is run non-interactively. Only do native log things
	// when running non-interactively.
	if o.config.LogConfig.UseNativeLogger && len(os.Getenv("PS1")) == 0 {
		err := os.MkdirAll(o.logDirPath, 0700)
		if err != nil {
			return err
		}

		log.SetOutput(os.Stderr)

		if o.config.LogConfig.NativeLogFlags > 0 {
			originalLogFlags := log.Flags()
			log.SetFlags(o.config.LogConfig.NativeLogFlags)
			defer log.SetFlags(originalLogFlags)
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

	// TODO: Make working directory customizable.
	replacer := strings.NewReplacer(serviceNamePlaceholder, config.DaemonId,
		shortDescriptionPlaceholder, fmt.Sprintf("%s daemon.", config.DaemonId),
		descriptionPlaceholder, config.Description,
		commandPlaceholder, exePath,
		workDirPathPlaceholder, "/tmp",
		logFilePathPlaceholder, logFilePath)

	script := replacer.Replace(systemvTemplate)
	if strings.Contains(script, placeholderDelim) {
		return nil, fmt.Errorf("failed to replace all placeholders in daemon init.d script")
	}

	return &systemvDaemon{
		config:       config,
		initContents: script,
		initFilePath: path.Join("/etc/init.d", config.DaemonId),
		logDirPath:   path.Dir(logFilePath),
	}, nil
}

func runServiceCommand(args ...string) (string, error) {
	servicePath := "service"
	output, err := exec.Command(servicePath, args...).CombinedOutput()
	trimmedOutput := strings.TrimSpace(string(output))
	if err != nil {
		return trimmedOutput, fmt.Errorf("failed to execute '%s %s' - %s - output: %s",
			servicePath, args, err.Error(), trimmedOutput)
	}

	return trimmedOutput, nil
}

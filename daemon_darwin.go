package cyberdaemon

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/stephen-fox/launchctlutil"
)

type darwinController struct {
	config            launchctlutil.Configuration
	stderrLogFilePath string
	logConfig         LogConfig
}

func (o *darwinController) Status() (Status, error) {
	details, err := launchctlutil.CurrentStatus(o.config.GetLabel())
	if err != nil {
		return "", err
	}

	switch details.Status {
	case launchctlutil.NotInstalled:
		return NotInstalled, nil
	case launchctlutil.NotRunning:
		return Stopped, nil
	case launchctlutil.Running:
		return Running, nil
	}

	return Unknown, nil
}

func (o *darwinController) Install() error {
	if o.logConfig.UseNativeLogger {
		err := os.MkdirAll(path.Dir(o.stderrLogFilePath), 0700)
		if err != nil {
			return err
		}
	}

	return launchctlutil.Install(o.config)
}

func (o *darwinController) Uninstall() error {
	configFilePath, err := o.config.GetFilePath()
	if err != nil {
		return err
	}

	// FYI: This call stops the daemon if it is running, and removes it.
	return launchctlutil.Remove(configFilePath, o.config.GetKind())
}

func (o *darwinController) Start() error {
	return launchctlutil.Start(o.config.GetLabel(), o.config.GetKind())
}

func (o *darwinController) Stop() error {
	return launchctlutil.Stop(o.config.GetLabel(), o.config.GetKind())
}

type darwinDaemonizer struct {
	logConfig LogConfig
}

func (o *darwinDaemonizer) RunUntilExit(application Application) error {
	// The 'PS1' environment variable will be empty / not set when
	// this is run non-interactively. Only do native log things
	// when running non-interactively.
	if o.logConfig.UseNativeLogger && len(os.Getenv("PS1")) == 0 {
		log.SetOutput(os.Stderr)

		if o.logConfig.NativeLogFlags > 0 {
			originalLogFlags := log.Flags()
			log.SetFlags(o.logConfig.NativeLogFlags)
			defer log.SetFlags(originalLogFlags)
		}
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

func NewController(config Config) (Controller, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}

	// TODO: Allow user to provide reverse DNS prefix using OS option.
	if strings.Count(config.DaemonId, ".") < 2 {
		return nil, fmt.Errorf("daemon ID must be in reverse DNS format on macOS")
	}

	var logFilePath string

	if config.LogConfig.UseNativeLogger {
		// TODO: Support user, or system logs.
		// TODO: Use a friendly name for the log directory
		//  and file name.
		logFilePath = path.Join(os.Getenv("HOME"), "Library", "Logs", config.DaemonId, config.DaemonId + ".log")
	}

	// Caveat: launchd does not have any concept similar to
	// 'systemctl enable'. You can only choose to run the job
	// on load, when specific events occur - you cannot configure
	// it to run on boot or login without making it start when
	// launchd loads it.
	runOnLoad := false
	switch config.StartType {
	case StartOnLoad, StartImmediately:
		runOnLoad = true
	}

	// TODO: Make macOS options customizable.
	lconfig, err := launchctlutil.NewConfigurationBuilder().
		SetKind(launchctlutil.UserAgent).
		SetLabel(config.DaemonId).
		SetRunAtLoad(runOnLoad).
		SetCommand(exePath).
		SetStandardErrorPath(logFilePath).
		Build()
	if err != nil {
		return nil, err
	}

	return &darwinController{
		config:            lconfig,
		stderrLogFilePath: logFilePath,
		logConfig:         config.LogConfig,
	}, nil
}

func NewDaemonizer(logConfig LogConfig) Daemonizer {
	return &darwinDaemonizer{
		logConfig: logConfig,
	}
}

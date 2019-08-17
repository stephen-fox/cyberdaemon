package control

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"strings"

	"github.com/stephen-fox/cyberdaemon"
	"github.com/stephen-fox/launchctlutil"
)

type darwinController struct {
	config            launchctlutil.Configuration
	stderrLogFilePath string
	logConfig         cyberdaemon.LogConfig
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

func NewController(controllerConfig ControllerConfig) (Controller, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}

	// TODO: Allow user to provide reverse DNS prefix using OS option.
	if strings.Count(controllerConfig.DaemonID, ".") < 2 {
		return nil, fmt.Errorf("daemon ID must be in reverse DNS format on macOS (e.g., net.website.MyApp)")
	}

	kind, setRunAs, logFilePath, err := runSettings(controllerConfig)
	if err != nil {
		return nil, err
	}

	var runAs string
	if setRunAs {
		runAs = controllerConfig.RunAs
	}

	// Caveat: launchd does not have any concept similar to
	// 'systemctl enable'. You can only choose to run the job
	// on load, when specific events occur - you cannot configure
	// it to run on boot or login without making it start when
	// launchd loads it.
	runOnLoad := false
	switch controllerConfig.StartType {
	case StartOnLoad, StartImmediately:
		runOnLoad = true
	}

	builder := launchctlutil.NewConfigurationBuilder().
		SetKind(kind).
		SetLabel(controllerConfig.DaemonID).
		SetRunAtLoad(runOnLoad).
		SetCommand(exePath).
		SetStandardErrorPath(logFilePath).
		SetUserName(runAs)

	for i := range controllerConfig.Arguments {
		builder.AddArgument(controllerConfig.Arguments[i])
	}

	lconfig, err := builder.Build()
	if err != nil {
		return nil, err
	}

	return &darwinController{
		config:            lconfig,
		stderrLogFilePath: logFilePath,
		logConfig:         controllerConfig.LogConfig,
	}, nil
}

// runSettings returns the launchd service kind, whether the run as username
// should be specified in the launchd config, and the log file path.
func runSettings(config ControllerConfig) (launchctlutil.Kind, bool, string, error) {
	// TODO: Use a friendly name for the log directory and file name.
	logPathSuffix := fmt.Sprintf("Library/Logs/%s/%s.log",
		config.DaemonID, config.DaemonID)

	if len(config.RunAs) == 0 {
		return launchctlutil.Daemon, false, path.Join("/", logPathSuffix), nil
	}

	current, err := user.Current()
	if err != nil {
		return launchctlutil.Daemon, false, "",
			fmt.Errorf("failed to get current user - %s", err.Error())
	}

	_, onlyRunWhenLoggedIn := config.SystemSpecificOptions[RunOnlyWhenLoggedIn]
	if onlyRunWhenLoggedIn {
		if config.RunAs == current.Username {
			return launchctlutil.UserAgent, false, path.Join(current.HomeDir, logPathSuffix), nil
		}
		return launchctlutil.Daemon, false, "",
			fmt.Errorf("the '%s' option cannot be used when the curret user is not the RunAs user",
				RunOnlyWhenLoggedIn)
	}

	runAs, lookUpErr := user.Lookup(config.RunAs)
	if lookUpErr != nil {
		return launchctlutil.Daemon, true, path.Join("/Users", config.RunAs, logPathSuffix), nil
	}

	return launchctlutil.Daemon, true, path.Join(runAs.HomeDir, logPathSuffix), nil
}

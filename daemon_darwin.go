package cyberdaemon

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/stephen-fox/launchctlutil"
)

type darwinDaemon struct {
	config launchctlutil.Configuration
}

func (o *darwinDaemon) Status() (Status, error) {
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

func (o *darwinDaemon) Install() error {
	return launchctlutil.Install(o.config)
}

func (o *darwinDaemon) Uninstall() error {
	configFilePath, err := o.config.GetFilePath()
	if err != nil {
		return err
	}

	return launchctlutil.Remove(configFilePath, o.config.GetKind())
}

func (o *darwinDaemon) Start() error {
	return launchctlutil.Start(o.config.GetLabel(), o.config.GetKind())
}

func (o *darwinDaemon) Stop() error {
	return launchctlutil.Stop(o.config.GetLabel(), o.config.GetKind())
}

func (o *darwinDaemon) RunUntilExit(logic ApplicationLogic) error {
	c := make(chan os.Signal)

	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(c)

	err := logic.Start()
	if err != nil {
		return err
	}

	<-c

	err = logic.Stop()
	if err != nil {
		return err
	}

	return nil
}

func NewDaemon(config Config) (Daemon, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}

	if strings.Count(config.DaemonId, ".") < 2 {
		return nil, fmt.Errorf("daemon ID must be in reverse DNS format on macOS")
	}

	// TODO: Make macOS options customizable.
	lconfig, err := launchctlutil.NewConfigurationBuilder().
		SetKind(launchctlutil.UserAgent).
		SetLabel(config.DaemonId).
		SetRunAtLoad(true).
		SetCommand(exePath).
		Build()
	if err != nil {
		return nil, err
	}

	return &darwinDaemon{
		config: lconfig,
	}, nil
}

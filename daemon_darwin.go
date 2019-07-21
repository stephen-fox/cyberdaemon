package cyberdaemon

import (
	"os"
	"os/signal"
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

func (o *darwinDaemon) ExecuteCommand(command Command) (string, error) {
	if command == GetStatus {
		status, err := o.Status()
		if err != nil {
			return "", err
		}

		return status.printableStatus(), nil
	}

	switch command {
	case Start:
		err := launchctlutil.Start(o.config.GetLabel(), o.config.GetKind())
		if err != nil {
			return "", err
		}

		return "", nil
	case Stop:
		err := launchctlutil.Stop(o.config.GetLabel(), o.config.GetKind())
		if err != nil {
			return "", err
		}

		return "", nil
	case Install:
		err := launchctlutil.Install(o.config)
		if err != nil {
			return "", err
		}

		return "", nil
	case Uninstall:
		filePath, err := o.config.GetFilePath()
		if err != nil {
			return "", err
		}

		err = launchctlutil.Remove(filePath, o.config.GetKind())
		if err != nil {
			return "", err
		}

		return "", nil
	}

	return "", &CommandError{
		isUnknown: true,
		command:   command,
	}
}

func (o *darwinDaemon) BlockAndRun(logic ApplicationLogic) error {
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

	// TODO: Make macOS options customizable.
	lconfig, err := launchctlutil.NewConfigurationBuilder().
		SetKind(launchctlutil.UserAgent).
		SetLabel(config.Name).
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

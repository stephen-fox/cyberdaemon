package cyberdaemon

import (
	"fmt"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"
)

type windowsDaemon struct {
	config Config
}

func (o *windowsDaemon) Status() (Status, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", err
	}
	defer m.Disconnect()

	s, err := m.OpenService(o.config.Name)
	if err != nil {
		return "", err
	}
	defer s.Close()

	winStatus, err := s.Query()
	if err != nil {
		return "", err
	}

	switch winStatus.State {
	case svc.StopPending:
		return Stopping, nil
	case svc.Stopped:
		return Stopped, nil
	case svc.StartPending:
		return Starting, nil
	case svc.Running:
		return Running, nil
	case svc.ContinuePending:
		return Resuming, nil
	case svc.PausePending:
		return Pausing, nil
	case svc.Paused:
		return Paused, nil
	}

	return Unknown, nil
}

func (o *windowsDaemon) Install() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	c := mgr.Config{
		DisplayName: o.config.Name,
		Description: o.config.Description,
		// TODO: Make service start type customizable.
		StartType:   mgr.StartAutomatic,
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	// TODO: Support custom arguments.
	s, err := m.CreateService(o.config.Name, exePath, c)
	if err != nil {
		return err
	}
	defer s.Close()

	err = eventlog.InstallAsEventCreate(o.config.Name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return err
	}

	return nil
}

func (o *windowsDaemon) Uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(o.config.Name)
	if err != nil {
		return err
	}
	defer s.Close()

	err = s.Delete()
	if err != nil {
		return err
	}

	err = eventlog.Remove(o.config.Name)
	if err != nil {
		return err
	}

	return nil
}

func (o *windowsDaemon) Start() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(o.config.Name)
	if err != nil {
		return err
	}
	defer s.Close()

	err = s.Start()
	if err != nil {
		return err
	}

	return nil
}

func (o *windowsDaemon) Stop() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(o.config.Name)
	if err != nil {
		return err
	}
	defer s.Close()

	err = stopAndWait(s)
	if err != nil {
		return err
	}

	return nil
}

func (o *windowsDaemon) RunUntilExit(logic ApplicationLogic) error {
	isInteractive, err := svc.IsAnInteractiveSession()
	if err != nil {
		return err
	}

	if isInteractive {
		err = logic.Start()
		if err != nil {
			return err
		}

		interrupts := make(chan os.Signal)

		signal.Notify(interrupts, os.Interrupt)

		<-interrupts

		signal.Stop(interrupts)

		return logic.Stop()
	}

	wrapper := serviceWrapper{
		name:     o.config.Name,
		appLogic: logic,
		errMutex: &sync.Mutex{},
	}

	err = wrapper.runAndBlock()
	if err != nil {
		return err
	}

	return nil
}

type serviceWrapper struct {
	name     string
	appLogic ApplicationLogic
	errMutex *sync.Mutex
	lastErr  error
}

// runAndBlock based on windowsDaemon.Run() method by kardianos et al:
//
//  github.com/kardianos/service in service_windows.go
//  commit: b1866cf76903d81b491fb668ba14f4b1322b2ca7
func (o *serviceWrapper) runAndBlock() error {
	o.setStartStopError(nil)

	// Return error messages from start and stop routines
	// that get executed in the Execute method.
	// Guarded with a mutex as it may run a different thread
	// (callback from windows).
	runErr := svc.Run(o.name, o)

	startStopErr := o.startStopErr()
	if startStopErr != nil {
		return startStopErr
	}

	if runErr != nil {
		return runErr
	}

	return nil
}

// Execute based on windowsDaemon.Execute() method by kardianos et al:
//
//  github.com/kardianos/service in service_windows.go
//  commit: b1866cf76903d81b491fb668ba14f4b1322b2ca7
func (o *serviceWrapper) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop|svc.AcceptShutdown

	changes <- svc.Status{
		State: svc.StartPending,
	}

	if err := o.appLogic.Start(); err != nil {
		o.setStartStopError(err)
		return true, 1
	}

	changes <- svc.Status{
		State:   svc.Running,
		Accepts: cmdsAccepted,
	}

loop:
	for {
		c := <-r
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{
				State: svc.StopPending,
			}
			if err := o.appLogic.Stop(); err != nil {
				o.setStartStopError(err)
				return true, 2
			}
			break loop
		default:
			continue loop
		}
	}

	return false, 0
}

func (o *serviceWrapper) setStartStopError(err error) {
	o.errMutex.Lock()
	o.lastErr = err
	o.errMutex.Unlock()
}

func (o *serviceWrapper) startStopErr() error {
	o.errMutex.Lock()
	defer o.errMutex.Unlock()

	return o.lastErr
}

func NewDaemon(config Config) (Daemon, error) {
	return &windowsDaemon{
		config: config,
	}, nil
}

// stopAndWait based on stopAndWait by takama et al:
//
//  github.com/takama/daemon in daemon_windows.go
//  commit: 7b0f9893e24934bbedef065a1768c33779951e7d
func stopAndWait(s *mgr.Service) error {
	// First stop the service. Then wait for the service to
	// actually stop before starting it.
	status, err := s.Control(svc.Stop)
	if err != nil {
		return err
	}

	timeDuration := time.Millisecond * 50

	timeout := defaultServiceStopTimeout()
	if timeout == 0 {
		timeout = time.Millisecond * 20000
	}

	onTimeout := time.After(timeout + (timeDuration * 2))
	tick := time.NewTicker(timeDuration)
	defer tick.Stop()

	for status.State != svc.Stopped {
		select {
		case <-tick.C:
			status, err = s.Query()
			if err != nil {
				return err
			}
		case <-onTimeout:
			return fmt.Errorf("service failed to stop after %s", timeout.String())
		}
	}

	return nil
}

// defaultServiceStopTimeout based on getStopTimeout by takama et al:
//
//  github.com/takama/daemon in daemon_windows.go
//  commit: 7b0f9893e24934bbedef065a1768c33779951e7d
func defaultServiceStopTimeout() time.Duration {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control`, registry.READ)
	if err != nil {
		return 0
	}

	sv, _, err := key.GetStringValue("WaitToKillServiceTimeout")
	if err != nil {
		return 0
	}

	v, err := strconv.Atoi(sv)
	if err != nil {
		return 0
	}

	return time.Millisecond * time.Duration(v)
}

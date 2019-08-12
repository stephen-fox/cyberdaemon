package cyberdaemon

import (
	"fmt"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"
)

const (
	notInstalledErr = "The specified service does not exist as an installed service."
)

type windowsController struct {
	config       ControllerConfig
	winStartType uint32
}

func (o *windowsController) Status() (Status, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", err
	}
	defer m.Disconnect()

	s, err := m.OpenService(o.config.DaemonID)
	if err != nil {
		if err.Error() == notInstalledErr {
			return NotInstalled, nil
		}

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

func (o *windowsController) Install() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	c := mgr.Config{
		DisplayName: o.config.DaemonID,
		Description: o.config.Description,
		StartType:   o.winStartType,
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	// TODO: Support custom arguments.
	s, err := m.CreateService(o.config.DaemonID, exePath, c)
	if err != nil {
		return err
	}
	defer s.Close()

	if o.config.StartType == StartImmediately {
		err := s.Start()
		if err != nil {
			s.Delete()
			return err
		}
	}

	err = eventlog.InstallAsEventCreate(o.config.DaemonID, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return err
	}

	return nil
}

func (o *windowsController) Uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(o.config.DaemonID)
	if err != nil {
		return err
	}
	defer s.Close()

	// Attempt to stop the service before removing it.
	// Windows does not stop the service's process when
	// the service is deleted. Do not bother checking
	// the error because there is nothing to do if the
	// stop fails.
	stopAndWait(s)

	err = s.Delete()
	if err != nil {
		return err
	}

	err = eventlog.Remove(o.config.DaemonID)
	if err != nil {
		return err
	}

	return nil
}

func (o *windowsController) Start() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(o.config.DaemonID)
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

func (o *windowsController) Stop() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(o.config.DaemonID)
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

type windowsDaemonizer struct {
	logConfig LogConfig
}

func (o *windowsDaemonizer) RunUntilExit(application Application) error {
	isInteractive, err := svc.IsAnInteractiveSession()
	if err != nil {
		return err
	}

	if isInteractive {
		err = application.Start()
		if err != nil {
			return err
		}

		interrupts := make(chan os.Signal)
		signal.Notify(interrupts, os.Interrupt)
		<-interrupts
		signal.Stop(interrupts)

		return application.Stop()
	}

	if o.logConfig.UseNativeLogger {
		events, err := eventlog.Open(application.WindowsDaemonID())
		if err != nil {
			return err
		}
		originalLogFlags := log.Flags()
		if o.logConfig.NativeLogFlags > 0 {
			log.SetFlags(o.logConfig.NativeLogFlags)
		} else {
			// Timestamps are provided by Windows event log by
			// default. Set log flags to 0, thus disabling the
			// go logger's timestamps.
			log.SetFlags(0)
		}
		log.SetOutput(&eventLogWriter{
			fn: events.Info,
		})
		defer log.SetFlags(originalLogFlags)
		defer log.SetOutput(os.Stderr)
		defer events.Close()
	}

	wrapper := serviceWrapper{
		name:     application.WindowsDaemonID(),
		app:      application,
		errMutex: &sync.Mutex{},
	}

	err = wrapper.runAndBlock()
	if err != nil {
		return err
	}

	return nil
}

type eventLogWriter struct {
	fn func(eventId uint32, message string) error
}

func (o eventLogWriter) Write(p []byte) (n int, err error) {
	err = o.fn(0, string(p))
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

type serviceWrapper struct {
	name     string
	app      Application
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

	if err := o.app.Start(); err != nil {
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
			if err := o.app.Stop(); err != nil {
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

func NewController(controllerConfig ControllerConfig) (Controller, error) {
	var winStartType uint32
	switch controllerConfig.StartType {
	case StartImmediately, StartOnLoad:
		winStartType = mgr.StartAutomatic
	default:
		winStartType = mgr.StartManual
	}

	return &windowsController{
		config:       controllerConfig,
		winStartType: winStartType,
	}, nil
}

func NewDaemonizer(logConfig LogConfig) Daemonizer {
	return &windowsDaemonizer{
		logConfig: logConfig,
	}
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

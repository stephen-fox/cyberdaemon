package cyberdaemon

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

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

func NewDaemonizer(logConfig LogConfig) Daemonizer {
	return &darwinDaemonizer{
		logConfig: logConfig,
	}
}

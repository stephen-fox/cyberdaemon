package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"time"

	"github.com/stephen-sfox/cyberdaemon"
)

const (
	appName     = "cyberdaemon-filewriter-example"
	description = "An example daemon created by the cyberdaemon library."

	daemonCommandArg = "command"
	example = "example.exe -" + daemonCommandArg

	usage = appName + `

[USAGE]
Compile this application as an executable (if you are on Windows, make sure it
has .exe as its suffix). You can then install it as a service by running:
	'` + example + ` install'

Once installed, it will create a temporary file in C:\\ and write the current
time to it every few seconds. The service can be stopped or started by running:
	'` + example + ` stop'
or:
	'` + example + ` start'

The service can be uninstalled by running:
	'` + example + ` uninstall'`
)

func main() {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalln(err.Error())
	}

	defaultWorkDir := path.Join(path.Dir(exePath), appName)

	command := flag.String(daemonCommandArg, "", "The daemon command to execute")
	workDir := flag.String("", defaultWorkDir, "Directory where files will be written")
	help := flag.Bool("h", false, "Displays this help page")

	flag.Parse()

	if *help {
		fmt.Print(usage)
		os.Exit(0)
	}

	err = os.MkdirAll(*workDir, 0755)
	if err != nil {
		log.Fatalln(err.Error())
	}

	logFile, err := os.OpenFile(path.Join(*workDir, "filewriter.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalln(err.Error())
	}
	defer logFile.Close()

	daemon, err := cyberdaemon.NewDaemon(cyberdaemon.Config{
		Name:        appName,
		Description: description,
	})
	if err != nil {
		log.Fatalln(err.Error())
	}

	if len(*command) > 0 {
		output, err := daemon.ExecuteCommand(cyberdaemon.Command(*command))
		if err != nil {
			log.Fatalln(err.Error())
		}

		if len(output) > 0 {
			log.Println(output)
		}

		return
	}

	// TODO: Do this earlier in a way that does not conflict the app
	//  and the Windows service trying to access the same file.
	log.SetOutput(io.MultiWriter(logFile, os.Stderr))

	err = daemon.BlockAndRun(&logic{
		dir:  *workDir,
		stop: make(chan chan struct{}),
	})
	if err != nil {
		log.Fatalln(err.Error())
	}
}

type logic struct {
	dir  string
	stop chan chan struct{}
}

func (o *logic) Start() error {
	f, err := os.OpenFile(path.Join(o.dir, "filewriter-output.txt"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	go updateFileLoop(f, o.stop)

	return nil
}

func (o *logic) Stop() error {
	stopTimeout := 5 * time.Second
	onStopTimeout := time.NewTimer(stopTimeout)
	rejoin := make(chan struct{})

	log.Println("stopping")

	select {
	case o.stop <- rejoin:
		onStopTimeout.Stop()
		rejoinTimeout := 2 * time.Second
		onRejoinTimeout := time.NewTimer(rejoinTimeout)

		select {
		case <-rejoin:
			log.Println("rejoined on stop")
			onRejoinTimeout.Stop()
		case <-onRejoinTimeout.C:
			return fmt.Errorf("app logic did not stop after %s", rejoinTimeout.String())
		}
	case <-onStopTimeout.C:
		return fmt.Errorf("app logic did not respond to stop after %s", stopTimeout.String())
	}

	return nil
}

func updateFileLoop(f *os.File, stop chan chan struct{}) {
	defer f.Close()
	defer os.Remove(f.Name())

	log.Printf("target file is '%s'", f.Name())

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, err := fmt.Fprintln(f, time.Now().String())
			if err != nil {
				log.Printf("failed to write to file - %s", err.Error())
			}
		case rejoin := <-stop:
			log.Printf("received stop signal")
			rejoin <- struct{}{}
			return
		}
	}
}

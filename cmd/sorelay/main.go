package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pborman/getopt/v2"

	"github.com/rupor-github/win-gpg-agent/assuan/client"
	"github.com/rupor-github/win-gpg-agent/config"
	"github.com/rupor-github/win-gpg-agent/misc"
	"github.com/rupor-github/win-gpg-agent/util"
)

var (
	title   = "sorelay"
	tooltip = "Socket relay program for WSL"
	verStr  = fmt.Sprintf("%s (%s) %s", misc.GetVersion(), runtime.Version(), misc.GetGitHash())
	// Arguments.
	cli         = getopt.New()
	aConfigName = title + ".conf"
	aShowHelp   bool
	aShowVer    bool
	aDebug      bool
	aAssuan     bool
)

func main() {

	util.NewLogWriter(title, 0, false)

	// configuration will be picked up at the same place where executable is
	expath, err := os.Executable()
	if err == nil {
		aConfigName = filepath.Join(filepath.Dir(expath), aConfigName)
	}

	cli.SetProgram("sorelay.exe")
	cli.SetParameters("path-to-socket")
	cli.FlagLong(&aAssuan, "assuan", 'a', "Open Assuan socket instead of Unix one")
	cli.FlagLong(&aConfigName, "config", 'c', "Configuration file", "path")
	cli.FlagLong(&aShowVer, "version", 0, "Show version information")
	cli.FlagLong(&aShowHelp, "help", 'h', "Show help")
	cli.FlagLong(&aDebug, "debug", 'd', "Turn on debugging")

	if err := cli.Getopt(os.Args, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Unsupported options in %+v: %s", os.Args, err.Error())
	}

	if aShowHelp {
		fmt.Fprintf(os.Stderr, "\n%s\n\n\t%s\n\n", tooltip, verStr)
		cli.PrintUsage(os.Stderr)
		os.Exit(0)
	}

	if aShowVer {
		fmt.Fprintf(os.Stderr, "\n%s\n", verStr)
		os.Exit(0)
	}

	if cli.NArgs() != 1 {
		fmt.Fprintf(os.Stderr, "Single path to socket should be specified as positional argument, we have %d parameters instead", cli.NArgs())
		os.Exit(1)
	}
	socketName := cli.Arg(0)

	// Read configuration
	cfg, err := config.Load(aConfigName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to load configuration from %s: %s\n", aConfigName, err.Error())
		os.Exit(1)
	}
	if aDebug {
		cfg.GUI.Debug = aDebug
	}
	util.NewLogWriter(title, 0, cfg.GUI.Debug)

	log.Printf("Dialing %s", socketName)

	var conn net.Conn
	if aAssuan {
		conn, err = client.Dial(socketName)
	} else {
		conn, err = net.Dial("unix", socketName)
	}
	if err != nil {
		log.Printf("Unable to dial socket \"%s\": %s", socketName, err.Error())
		os.Exit(1)
	}
	defer conn.Close()

	log.Printf("Connected to %s", socketName)

	go func() {
		l, err := io.Copy(conn, os.Stdin)
		if err != nil && !util.IsNetClosing(err) {
			log.Printf("Copy from stdin to %s failed: %s", socketName, err.Error())
			os.Exit(1)
		}
		log.Printf("Copied from stdin to %s - %d bytes (stdin EOF)", socketName, l)
		os.Exit(0)
	}()

	l, err := io.Copy(os.Stdout, conn)
	if err != nil && !util.IsNetClosing(err) {
		log.Printf("Copy from %s to stdout failed: %s", socketName, err.Error())
		return
	}
	log.Printf("Copied from %s to stdout - %d bytes (socket EOF)", socketName, l)
}

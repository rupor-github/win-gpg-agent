package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/allan-simon/go-singleinstance"
	"github.com/pborman/getopt/v2"
	clip "github.com/rupor-github/gclpr/server"

	"github.com/rupor-github/win-gpg-agent/agent"
	"github.com/rupor-github/win-gpg-agent/config"
	"github.com/rupor-github/win-gpg-agent/misc"
	"github.com/rupor-github/win-gpg-agent/systray"
	"github.com/rupor-github/win-gpg-agent/util"
)

var (
	title       = "agent-gui"
	tooltip     = "GUI wrapper for gpg-agent"
	cli         = getopt.New()
	aConfigName = title + ".conf"
	usageString string
	aShowHelp   bool
	aDebug      bool
	gpgAgent    *agent.Agent
	clipCancel  context.CancelFunc
	clipCtx     context.Context
	clipHelp    string
)

const (
	envGPGHomeName = "GNUPG_HOME"
	envGUIHomeName = "AGENT_HOME"
	envPipeName    = "SSH_AUTH_SOCK"
)

func onReady() {

	log.Print("Entering systray")

	systray.SetIcon(systray.MakeIntResource(1000))
	systray.SetTitle(title)
	systray.SetTooltip(tooltip)

	miStat := systray.AddMenuItem("Status", "Shows application state")
	miHelp := systray.AddMenuItem("About", "Shows application help")
	systray.AddSeparator()
	miQuit := systray.AddMenuItem("Exit", "Exits application")

	go func() {
		for {
			select {
			case <-miHelp.ClickedCh:
				util.ShowOKMessage(util.MsgInformation, title, usageString)
			case <-miStat.ClickedCh:
				if gpgAgent != nil {
					help := gpgAgent.Status() + "\n\n" + clipHelp
					util.ShowOKMessage(util.MsgInformation, title, help)
				}
			case <-miQuit.ClickedCh:
				log.Print("Requesting exit")
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	// stop servicing clipboard and uri requests
	clipCancel()
	// and all gpg related translations
	if err := gpgAgent.Stop(); err != nil {
		log.Printf("Problem stopping gpg agent: %s", err.Error())
	}
	log.Print("Exiting systray")
}

func onSession(e systray.SessionEvent) {
	switch e {
	case systray.SesLock:
		gpgAgent.SessionLock()
	case systray.SesUnlock:
		gpgAgent.SessionUnlock()
	}
}

func run() error {

	// Eventually gpg-agent on Windows will directly support Windows openssh server (Oh, hear the call! — Good hunting all) - https://dev.gnupg.org/T3883.
	// Until then we need to create specific translation layers. In addition assuan S.gpg-agent.ssh is presently broken under Windows (at least in
	// GnuPG 2.2.25), so we have to resort to putty support instead to transport data from/to named pipe (Windows openssh at least up to 8.1) and AF_UNIX
	// socket (WSL). NOTE: WSL2 requires additional layer of translation using socat on Linux side and either HYPER-V socket server or helper on Windows end
	// since AF_UNIX interop is not (yet? ever?) implemented.

	// Transact on pipe for Windows openssh
	if err := gpgAgent.Serve(agent.ConnectorPipeSSH); err != nil {
		return err
	}
	defer gpgAgent.Close(agent.ConnectorPipeSSH)

	// Transact on AF_UNIX socket for ssh
	if err := gpgAgent.Serve(agent.ConnectorSockAgentSSH); err != nil {
		return err
	}
	defer gpgAgent.Close(agent.ConnectorSockAgentSSH)

	// Transact on AF_UNIX socket for gpg agent
	if err := gpgAgent.Serve(agent.ConnectorSockAgent); err != nil {
		return err
	}
	defer gpgAgent.Close(agent.ConnectorSockAgent)

	// Transact on AF_UNIX socket for gpg agent
	if err := gpgAgent.Serve(agent.ConnectorSockAgentExtra); err != nil {
		return err
	}
	defer gpgAgent.Close(agent.ConnectorSockAgentExtra)

	if gpgAgent.Cfg.GUI.SetEnv {
		if err := util.PrepareUserEnvironmentVariable("WSL_"+envGPGHomeName, gpgAgent.Cfg.GPG.Home, true, true); err != nil {
			return fmt.Errorf("unable to add %s to user environment: %w", "WSL_"+envGPGHomeName, err)
		}
		defer func() {
			if err := util.CleanUserEnvironmentVariable("WSL_"+envGPGHomeName, true); err != nil {
				log.Printf("Unable to delete %s from user environment: %s", "WSL_"+envGPGHomeName, err.Error())
			}
		}()
		if err := util.PrepareUserEnvironmentVariable("WIN_"+envGPGHomeName, util.PrepareWindowsPath(gpgAgent.Cfg.GPG.Home), true, false); err != nil {
			return fmt.Errorf("unable to add %s to user environment: %w", "WIN_"+envGPGHomeName, err)
		}
		defer func() {
			if err := util.CleanUserEnvironmentVariable("WIN_"+envGPGHomeName, true); err != nil {
				log.Printf("Unable to delete %s from user environment: %s", "WIN_"+envGPGHomeName, err.Error())
			}
		}()

		if err := util.PrepareUserEnvironmentVariable("WSL_"+envGUIHomeName, gpgAgent.Cfg.GUI.Home, true, true); err != nil {
			return fmt.Errorf("unable to add %s to user environment: %w", "WSL_"+envGUIHomeName, err)
		}
		defer func() {
			if err := util.CleanUserEnvironmentVariable("WSL_"+envGUIHomeName, true); err != nil {
				log.Printf("Unable to delete %s from user environment: %s", "WSL_"+envGUIHomeName, err.Error())
			}
		}()
		if err := util.PrepareUserEnvironmentVariable("WIN_"+envGUIHomeName, util.PrepareWindowsPath(gpgAgent.Cfg.GUI.Home), true, false); err != nil {
			return fmt.Errorf("unable to add %s to user environment: %w", "WIN_"+envGUIHomeName, err)
		}
		defer func() {
			if err := util.CleanUserEnvironmentVariable("WIN_"+envGUIHomeName, true); err != nil {
				log.Printf("Unable to delete %s from user environment: %s", "WIN_"+envGUIHomeName, err.Error())
			}
		}()

		if err := util.PrepareUserEnvironmentVariable(envPipeName, gpgAgent.Cfg.GUI.PipeName, false, false); err != nil {
			return fmt.Errorf("unable to add %s to user environment: %w", envPipeName, err)
		}
		defer func() {
			if err := util.CleanUserEnvironmentVariable(envPipeName, false); err != nil {
				log.Printf("Unable to delete %s from user environment: %s", envPipeName, err.Error())
			}
		}()
	}

	if err := gpgAgent.Start(); err != nil {
		return err
	}

	systray.Run(onReady, onExit, onSession)
	return nil
}

func buildUsageString() string {
	var buf = new(strings.Builder)
	fmt.Fprintf(buf, "\n%s\n\nVersion:\n\t%s (%s)\n\t%s\n\n", tooltip, misc.GetVersion(), runtime.Version(), misc.GetGitHash())
	cli.PrintUsage(buf)
	return buf.String()
}

func clipServe(cfg *config.Config) {
	clipCtx, clipCancel = context.WithCancel(context.Background())
	if len(cfg.GUI.Clp.Keys) > 0 {
		var (
			pkey  [32]byte
			pkeys = make(map[[32]byte]struct{})
		)
		for i, k := range cfg.GUI.Clp.Keys {
			pk, err := hex.DecodeString(k)
			if err != nil || len(pk) != 32 {
				log.Printf("Bad gclpr public key %d. Ignoring", i)
				continue
			}
			log.Printf("gclpr found public key: %s", k)
			copy(pkey[:], pk)
			pkeys[pkey] = struct{}{}
		}
		if len(pkeys) > 0 {
			// we have possible clients for remote clipboard
			clipHelp = fmt.Sprintf("gclpr is serving %d key(s) on port %d", len(pkeys), cfg.GUI.Clp.Port)
			go func() {
				if err := clip.Serve(clipCtx, cfg.GUI.Clp.Port, cfg.GUI.Clp.LE, pkeys); err != nil {
					log.Printf("gclpr serve() returned error: %s", err.Error())
					clipHelp = "gclpr is not running"
				}
			}()
		}
	}
}

func main() {

	util.NewLogWriter(title, 0, false)

	// Process arguments
	cli.SetProgram("agent-gui.exe")
	cli.SetParameters("")
	cli.FlagLong(&aConfigName, "config", 'c', "Configuration file", "path")
	cli.FlagLong(&aShowHelp, "help", 'h', "Show help")
	cli.FlagLong(&aDebug, "debug", 'd', "Turn on debugging")

	usageString = buildUsageString()

	// configuration will be picked up at the same place where executable is
	expath, err := os.Executable()
	if err == nil {
		aConfigName = filepath.Join(filepath.Dir(expath), aConfigName)
	}

	if err := cli.Getopt(os.Args, nil); err != nil {
		util.ShowOKMessage(util.MsgError, title, err.Error())
		os.Exit(1)
	}

	if aShowHelp {
		util.ShowOKMessage(util.MsgInformation, title, usageString)
		os.Exit(0)
	}

	// Read configuration
	cfg, err := config.Load(aConfigName)
	if err != nil {
		util.ShowOKMessage(util.MsgError, title, err.Error())
		os.Exit(1)
	}
	if aDebug {
		cfg.GUI.Debug = aDebug
	}
	util.NewLogWriter(title, 0, cfg.GUI.Debug)

	if err := os.MkdirAll(cfg.GUI.Home, 0700); err != nil {
		util.ShowOKMessage(util.MsgError, title, err.Error())
		os.Exit(1)
	}

	// Check if our Windows is modern enough to support AF_UNIX sockets - needed by WSL
	if ok, err := util.IsProperWindowsVer(); err != nil {
		util.ShowOKMessage(util.MsgError, title, err.Error())
		os.Exit(1)
	} else if !ok {
		util.ShowOKMessage(util.MsgError, title, "This Windows version does not support AF_UNIX sockets")
		os.Exit(1)
	}

	// Only allow single instance of gui to run
	lockName := filepath.Join(os.TempDir(), title+".lock")
	inst, err := singleinstance.CreateLockFile(lockName)
	if err != nil {
		log.Print("Application already running")
		os.Exit(0)
	}
	defer func() {
		// Not necessary at all
		inst.Close()
		os.Remove(lockName)
	}()

	// serve gclpr if requested
	clipServe(cfg)

	// We want to fully control gpg-agent, so if it is running - either we left it from previous run or it is not ours
	// Both cases should never happen so try to kill it just in case...
	if err := util.KillRunningAgent(); err != nil {
		util.ShowOKMessage(util.MsgError, title, err.Error())
		os.Exit(1)
	}

	// Now - start our own instance of gpg-agent
	gpgAgent, err = agent.NewAgent(cfg)
	if err != nil {
		util.ShowOKMessage(util.MsgError, title, err.Error())
		os.Exit(1)
	}

	// Enter main processing loop
	if err := run(); err != nil {
		util.ShowOKMessage(util.MsgError, title, err.Error())
	}
}

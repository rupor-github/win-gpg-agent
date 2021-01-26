package agent

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/multierr"
	"golang.org/x/sys/windows"

	"github.com/rupor-github/win-gpg-agent/assuan/client"
	"github.com/rupor-github/win-gpg-agent/config"
	"github.com/rupor-github/win-gpg-agent/util"
)

// Agent structure wraps running gpg-agent process.
type Agent struct {
	Cfg       *config.Config
	Ver, Exe  string
	locked    int32
	cmd       *exec.Cmd
	cmdOutput bytes.Buffer
	cancel    context.CancelFunc
	ctx       context.Context
	wg        sync.WaitGroup
	conns     []*Connector
}

// NewAgent initializes Agent structure.
func NewAgent(cfg *config.Config) (*Agent, error) {

	a := &Agent{Cfg: cfg}

	fname := filepath.Join(a.Cfg.GPG.Path, "bin", util.GPGAgentName+".exe")
	cmd := exec.Command(fname, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("unable to run \"%s\": %w", fname, err)
	}
	words := strings.Split(strings.Split(string(out), "\n")[0], " ")
	if len(words) == 3 {
		a.Ver = words[2]
	}
	a.Exe = fname

	a.conns = make([]*Connector, maxConnector)

	var locked *int32
	if !a.Cfg.GUI.IgnoreSessionLock {
		locked = &a.locked
	}

	a.conns[ConnectorSockAgent] = NewConnector(ConnectorSockAgent, a.Cfg.GPG.Home, a.Cfg.GUI.Home, util.SocketAgentName, locked, &a.wg)
	a.conns[ConnectorSockAgentExtra] = NewConnector(ConnectorSockAgentExtra, a.Cfg.GPG.Home, a.Cfg.GUI.Home, util.SocketAgentExtraName, locked, &a.wg)
	a.conns[ConnectorSockAgentBrowser] = NewConnector(ConnectorSockAgentBrowser, a.Cfg.GPG.Home, a.Cfg.GUI.Home, util.SocketAgentBrowserName, locked, &a.wg)
	a.conns[ConnectorSockAgentSSH] = NewConnector(ConnectorSockAgentSSH, a.Cfg.GPG.Home, a.Cfg.GUI.Home, util.SocketAgentSSHName, locked, &a.wg)
	a.conns[ConnectorPipeSSH] = NewConnector(ConnectorPipeSSH, "", "", a.Cfg.GUI.PipeName, locked, &a.wg)
	a.conns[ConnectorSockAgentCygwinSSH] = NewConnector(ConnectorSockAgentCygwinSSH, "", a.Cfg.GUI.Home, util.SocketAgentSSHCygwinName, locked, &a.wg)

	util.WaitForFileDeparture(time.Second*5,
		a.conns[ConnectorSockAgent].PathGPG(),
		a.conns[ConnectorSockAgentExtra].PathGPG(),
		a.conns[ConnectorSockAgentBrowser].PathGPG(),
		a.conns[ConnectorSockAgentSSH].PathGPG())

	a.ctx, a.cancel = context.WithCancel(context.Background())

	return a, nil
}

// Status returns string with currently running agent configuration.
func (a *Agent) Status() string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "\n\n---------------------------\nGnuPG version:\n---------------------------\n%s", a.Ver)
	fmt.Fprintf(&buf, "\n\n---------------------------\ngpg-agent command line:\n---------------------------\n%s", a.cmd.String())
	fmt.Fprintf(&buf, "\n\n---------------------------\ngpg-agent Assuan sockets directory:\n---------------------------\n%s", a.Cfg.GPG.Home)
	fmt.Fprintf(&buf, "\n\n---------------------------\nagent-gui AF_UNIX and Cygwin sockets directory:\n---------------------------\n%s", a.Cfg.GUI.Home)
	fmt.Fprintf(&buf, "\n\n---------------------------\nagent-gui SSH named pipe:\n---------------------------\n%s", a.Cfg.GUI.PipeName)

	return buf.String()
}

// SessionLock sets flag to indicate that user session is presently locked.
func (a *Agent) SessionLock() {
	if a != nil {
		atomic.StoreInt32(&a.locked, 1)
		log.Print("Session locked")
	}
}

// SessionUnlock sets flag to indicate that user session is presently unlocked.
func (a *Agent) SessionUnlock() {
	if a != nil {
		atomic.StoreInt32(&a.locked, 0)
		log.Print("Session unlocked")
	}
}

func (a *Agent) forceCleanup() error {
	if a.cmd != nil && a.cmd.Process != nil {
		log.Print("Forcefully killing gpg-agent")
		return a.cmd.Process.Kill()
	}
	return nil
}

func sendAssuanCmd(sockPath string, transact func(*client.Session) error) error {
	conn, err := client.Dial(sockPath)
	if err != nil {
		return fmt.Errorf("unable to dial assuan socket \"%s\": %w", sockPath, err)
	}
	defer conn.Close()

	ses, err := client.Init(conn)
	if err != nil {
		return fmt.Errorf("unable to init assuan session on \"%s\": %w", sockPath, err)
	}
	defer ses.Close()

	if err := transact(ses); err != nil {
		return err
	}
	return nil
}

// Start executes gpg-agent using configuration values.
func (a *Agent) Start() error {

	const DETACHED_PROCESS = 0x00000008

	expath, err := os.Executable()
	if err != nil {
		return err
	}

	args := []string{
		"--homedir", a.Cfg.GPG.Home,
		"--ssh-fingerprint-digest", "SHA256",
		"--use-standard-socket",  // in case we are dealing with older versions
		"--enable-ssh-support",   // presently useless under Windows
		"--enable-putty-support", // so we have to use this instead, but it does not work in 64 bits builds under Windows...
		"--pinentry-program", filepath.Join(filepath.Dir(expath), "pinentry.exe"),
		"--daemon",
	}
	if len(a.Cfg.GPG.Config) > 0 && util.FileExists(a.Cfg.GPG.Config) {
		args = append(args, "--options", a.Cfg.GPG.Config)
	}
	if len(a.Cfg.GPG.Args) > 0 {
		args = append(args, a.Cfg.GPG.Args...)
	}
	a.cmd = exec.Command(a.Exe, args...)
	a.cmd.SysProcAttr = &windows.SysProcAttr{CreationFlags: DETACHED_PROCESS}
	a.cmd.Stdout = &a.cmdOutput
	a.cmd.Stderr = &a.cmdOutput

	log.Printf("Executing: %s", a.cmd.String())
	if err := a.cmd.Start(); err != nil {
		return err
	}

	sockPath := a.conns[ConnectorSockAgent].PathGPG()
	if !util.WaitForFileArrival(time.Second*5, sockPath) {
		return multierr.Combine(
			fmt.Errorf("unable to access socket: %s", sockPath),
			a.forceCleanup(),
		)
	}

	if err := sendAssuanCmd(sockPath,
		func(ses *client.Session) error {
			if err := ses.Reset(); err != nil {
				return fmt.Errorf("unable to RESET assuan session on \"%s\": %w", sockPath, err)
			}
			return nil
		},
	); err != nil {
		return multierr.Combine(err, a.forceCleanup())
	}

	// Always terminate gracefully - see all in flight conversations to completion.
	go func() {
		<-a.ctx.Done()
		a.wg.Wait()
	}()

	return nil
}

// GetConnector returns pointer to requested connector or nil.
func (a *Agent) GetConnector(ct ConnectorType) *Connector {
	if a == nil || ct > maxConnector || len(a.conns) == 0 {
		return nil
	}
	return a.conns[ct]
}

// Serve handles serving requests for a particular ConnectorType.
func (a *Agent) Serve(ct ConnectorType) error {
	if a == nil || ct > maxConnector {
		return fmt.Errorf("gui agent has not been initialized properly")
	}
	return a.conns[ct].Serve(a.Cfg.GUI.Deadline)
}

// Close stops serving requests for a particular ConnectorType.
func (a *Agent) Close(ct ConnectorType) {
	if a == nil || ct > maxConnector {
		return
	}
	a.conns[ct].Close()
}

// Stop stops all connectors and gpg-agent cleanly.
func (a *Agent) Stop() error {

	if a == nil || a.cmd == nil {
		return nil
	}

	defer func() {
		// FIXME: what if gpg-agent is chatty? Do we want to buffer it forever?
		output := a.cmdOutput.String()
		if len(output) > 0 {
			log.Printf("gpg-agent output[\n%s]\n", output)
		}
	}()

	// stop serving go routines
	for _, c := range a.conns {
		c.Close()
	}
	// let in-flight requests to finish gracefully
	a.cancel()

	// tell gpg-agent to exit
	sockPath := a.conns[ConnectorSockAgent].PathGPG()
	if err := sendAssuanCmd(sockPath,
		func(ses *client.Session) error {
			if _, err := ses.SimpleCmd("KILLAGENT", ""); err != nil {
				return fmt.Errorf("unable to send KILLAGENT on \"%s\": %w", sockPath, err)
			}
			return nil
		},
	); err != nil {
		return multierr.Combine(err, a.forceCleanup())
	}

	if err := a.cmd.Wait(); err != nil {
		return err
	}
	return nil
}

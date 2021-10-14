package agent

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"

	"github.com/rupor-github/win-gpg-agent/assuan/client"
	"github.com/rupor-github/win-gpg-agent/util"
)

// ConnectorType to define what we support.
type ConnectorType int

// All possible Connector Types.
const (
	ConnectorSockAgent ConnectorType = iota
	ConnectorSockAgentExtra
	ConnectorSockAgentBrowser
	ConnectorSockAgentSSH
	ConnectorPipeSSH
	ConnectorSockAgentCygwinSSH
	ConnectorExtraPort
	maxConnector
)

func (ct ConnectorType) String() string {
	switch ct {
	case ConnectorSockAgent:
		return "gpg-agent socket"
	case ConnectorSockAgentExtra:
		return "gpg-agent extra socket"
	case ConnectorSockAgentBrowser:
		return "gpg-agent browser socket"
	case ConnectorSockAgentSSH:
		return "ssh-agent socket"
	case ConnectorPipeSSH:
		return "ssh-agent named pipe"
	case ConnectorSockAgentCygwinSSH:
		return "ssh-agent cygwin socket"
	case ConnectorExtraPort:
		return "gpg-agent extra socket on local port"
	default:
	}
	return fmt.Sprintf("unknown connector type %d", ct)
}

// Connector keeps parameters to be able to serve particular ConnectorType.
type Connector struct {
	index    ConnectorType
	pathGPG  string
	pathGUI  string
	name     string
	locked   *int32
	wg       *sync.WaitGroup
	listener net.Listener
}

// NewConnector initializes Connector of particular ConnectorType.
func NewConnector(index ConnectorType, pathGPG, pathGUI, name string, locked *int32, wg *sync.WaitGroup) *Connector {
	return &Connector{
		index:   index,
		pathGPG: pathGPG,
		pathGUI: pathGUI,
		name:    name,
		locked:  locked,
		wg:      wg,
	}
}

// Close stops serving on Connector.
func (c *Connector) Close() {
	if c == nil || c.listener == nil {
		return
	}
	if err := c.listener.Close(); err != nil {
		if !util.IsNetClosing(err) && !errors.Is(err, winio.ErrPipeListenerClosed) {
			log.Printf("Error closing listener on connector for %s: %s", c.index, err)
		}
	}
	if c.index != ConnectorPipeSSH && c.index != ConnectorExtraPort && len(c.PathGUI()) != 0 {
		if err := os.Remove(c.PathGUI()); err != nil {
			log.Printf("Error closing connector for %s: %s", c.index, err.Error())
		}
	}
}

// PathGPG returns path to gpg socket being served.
func (c *Connector) PathGPG() string {
	return filepath.Join(c.pathGPG, c.name)
}

// PathGUI returns path to unix socket being served.
func (c *Connector) PathGUI() string {
	return filepath.Join(c.pathGUI, c.name)
}

// Name returns name part of socket/pipe being served.
func (c *Connector) Name() string {
	return c.name
}

// Serve serves requests on Connector.
func (c *Connector) Serve(deadline time.Duration) error {
	switch c.index {
	case ConnectorSockAgent:
		fallthrough
	case ConnectorSockAgentExtra:
		return c.serveAssuanSocket(deadline)
	case ConnectorSockAgentSSH:
		return c.serveSSHSocket()
	case ConnectorPipeSSH:
		return c.serveSSHPipe()
	case ConnectorSockAgentCygwinSSH:
		return c.serveSSHCygwinSocket()
	case ConnectorExtraPort:
		return c.serveExtraPortSocket(deadline)
	default:
	}
	log.Printf("Connector for %s is not supported", c.index)
	return nil
}

func (c *Connector) handleAssuanRequest(socketName string, conn net.Conn, deadline time.Duration) {

	defer c.wg.Done()
	defer conn.Close()

	id := time.Now().UnixNano() // create unique id for debug tracing
	log.Printf("[%d] Accepted request from %s", id, socketName)

	socketNameAssuan := c.PathGPG()
	connAssuan, err := client.Dial(socketNameAssuan)
	if err != nil {
		log.Printf("[%d] Unable to dial assuan socket \"%s\": %s", id, socketNameAssuan, err.Error())
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer connAssuan.Close()
		log.Printf("[%d] Copying from %s to %s", id, socketName, socketNameAssuan)
		for c.locked == nil || atomic.LoadInt32(c.locked) == 0 {
			if deadline != 0 {
				_ = conn.SetDeadline(time.Now().Add(deadline))
			}
			l, err := io.Copy(connAssuan, conn)
			if err != nil {
				if errors.Is(err, os.ErrDeadlineExceeded) {
					if l > 0 {
						log.Printf("[%d] Copied from %s to %s - %d bytes, continuing", id, socketName, socketNameAssuan, l)
						continue
					}
					log.Printf("[%d] No activity on connection from %s to %s, exiting", id, socketName, socketNameAssuan)
					return
				}
				if !util.IsNetClosing(err) {
					log.Printf("[%d] Error copying from %s to %s - %d: %s", id, socketName, socketNameAssuan, l, err.Error())
					return
				}
			}
			log.Printf("[%d] Copied from %s to %s - %d bytes", id, socketName, socketNameAssuan, l)
			return
		}
		log.Print("Session is locked")
	}()

	log.Printf("[%d] Copying from %s to %s", id, socketNameAssuan, socketName)
	for c.locked == nil || atomic.LoadInt32(c.locked) == 0 {
		if deadline != 0 {
			_ = connAssuan.SetDeadline(time.Now().Add(deadline))
		}
		l, err := io.Copy(conn, connAssuan)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				if l > 0 {
					log.Printf("[%d] Copied from %s to %s - %d bytes, continuing", id, socketNameAssuan, socketName, l)
					continue
				}
				log.Printf("[%d] No activity on connection from %s to %s, exiting", id, socketNameAssuan, socketName)
				return
			}
			if !util.IsNetClosing(err) {
				log.Printf("[%d] Error copying from %s to %s - %d: %s", id, socketNameAssuan, socketName, l, err.Error())
				return
			}
		}
		log.Printf("[%d] Copied from %s to %s - %d bytes", id, socketNameAssuan, socketName, l)
		return
	}
	log.Print("Session is locked")
}

func (c *Connector) serveAssuanSocket(deadline time.Duration) error {

	if c == nil || len(c.pathGPG) == 0 || len(c.pathGUI) == 0 {
		return fmt.Errorf("gpg agent has not been initialized properly")
	}
	socketName := c.PathGUI()
	if len(socketName) > util.MaxNameLen {
		return fmt.Errorf("socket name is too long: %d, max allowed: %d", len(socketName), util.MaxNameLen)
	}

	_, err := os.Stat(socketName)
	if err == nil || !os.IsNotExist(err) {
		if err = os.Remove(socketName); err != nil {
			return fmt.Errorf("failed to unlink socket %s: %w", socketName, err)
		}
	}

	c.listener, err = net.Listen("unix", socketName)
	if err != nil {
		return fmt.Errorf("could not open socket %s: %w", socketName, err)
	}

	go func() {
		log.Printf("Serving %s on %s", c.index, socketName)
		for {
			conn, err := c.listener.Accept()
			if err != nil {
				if !util.IsNetClosing(err) {
					log.Printf("Quiting - unable to serve on unix socket: %s", err.Error())
				}
				return
			}
			c.wg.Add(1)
			go c.handleAssuanRequest(socketName, conn, deadline)
		}
	}()
	return nil
}

func (c *Connector) serveExtraPortSocket(deadline time.Duration) error {

	if c == nil || len(c.pathGPG) == 0 || len(c.pathGUI) == 0 {
		return fmt.Errorf("gpg agent has not been initialized properly")
	}

	var err error

	socketName := c.pathGUI
	c.listener, err = net.Listen("tcp", socketName)
	if err != nil {
		return fmt.Errorf("could not open socket %s: %w", socketName, err)
	}

	go func() {
		log.Printf("Serving %s on %s", c.index, socketName)
		for {
			conn, err := c.listener.Accept()
			if err != nil {
				if !util.IsNetClosing(err) {
					log.Printf("Quiting - unable to serve on TCP socket: %s", err.Error())
				}
				return
			}
			c.wg.Add(1)
			go c.handleAssuanRequest(socketName, conn, deadline)
		}
	}()
	return nil
}

func (c *Connector) serveSSHPipe() error {

	if c == nil || len(c.name) == 0 {
		return fmt.Errorf("gpg agent has not been initialized properly")
	}

	var err error
	cfg := &winio.PipeConfig{}
	c.listener, err = winio.ListenPipe(c.Name(), cfg)
	if err != nil {
		return fmt.Errorf("unable to listen on pipe %s: %w", c.Name(), err)
	}

	go func() {
		log.Printf("Serving %s on %s", c.index, c.Name())
		for {
			conn, err := c.listener.Accept()
			if err != nil {
				if !errors.Is(err, winio.ErrPipeListenerClosed) {
					log.Printf("Quiting - unable to serve on named pipe: %s", err)
				}
				return
			}
			c.wg.Add(1)
			go func() {
				defer c.wg.Done()
				defer conn.Close()
				id := time.Now().UnixNano() // create unique id for debug tracing
				log.Printf("[%d] Accepted request from %s", id, c.Name())
				if err := serveSSH(id, conn, c.locked); err != nil {
					log.Printf("[%d] SSH handler returned error: %s", id, err.Error())
				}
			}()
		}
	}()
	return nil
}

func (c *Connector) serveSSHSocket() error {

	if c == nil {
		return fmt.Errorf("gpg agent has not been initialized properly")
	}
	socketName := c.PathGUI()
	if len(socketName) > util.MaxNameLen {
		return fmt.Errorf("socket name is too long: %d, max allowed: %d", len(socketName), util.MaxNameLen)
	}

	_, err := os.Stat(socketName)
	if err == nil || !os.IsNotExist(err) {
		if err = os.Remove(socketName); err != nil {
			return fmt.Errorf("failed to unlink socket %s: %w", socketName, err)
		}
	}

	c.listener, err = net.Listen("unix", socketName)
	if err != nil {
		return fmt.Errorf("could not open socket %s: %w", socketName, err)
	}

	go func() {
		log.Printf("Serving %s on %s", c.index, socketName)
		for {
			conn, err := c.listener.Accept()
			if err != nil {
				if !util.IsNetClosing(err) {
					log.Printf("Quiting - unable to serve on unix socket: %s", err)
				}
				return
			}
			c.wg.Add(1)
			go func() {
				defer c.wg.Done()
				defer conn.Close()
				id := time.Now().UnixNano() // create unique id for debug tracing
				log.Printf("[%d] Accepted request from %s", id, socketName)
				if err := serveSSH(id, conn, c.locked); err != nil {
					log.Printf("[%d] SSH handler returned error: %s", id, err.Error())
				}
			}()
		}
	}()
	return nil
}

func (c *Connector) serveSSHCygwinSocket() error {

	if c == nil {
		return fmt.Errorf("gpg agent has not been initialized properly")
	}
	socketName := c.PathGUI()
	if len(socketName) > util.MaxNameLen {
		return fmt.Errorf("socket name is too long: %d, max allowed: %d", len(socketName), util.MaxNameLen)
	}

	_, err := os.Stat(socketName)
	if err == nil || !os.IsNotExist(err) {
		if err = os.Remove(socketName); err != nil {
			return fmt.Errorf("failed to unlink socket %s: %w", socketName, err)
		}
	}

	c.listener, err = net.Listen("tcp", "localhost:0")
	if err != nil {
		return fmt.Errorf("could not open cygwin socket: %w", err)
	}

	port := c.listener.Addr().(*net.TCPAddr).Port
	nonce, err := util.CygwinCreateSocketFile(socketName, port)
	if err != nil {
		return err
	}

	go func() {
		log.Printf("Serving %s on %s:%d with nonce: %s)", c.index, socketName, port, util.CygwinNonceString(nonce))
		for {
			conn, err := c.listener.Accept()
			if err != nil {
				if !util.IsNetClosing(err) {
					log.Printf("Quiting - unable to serve on Cygwin socket: %s", err)
				}
				return
			}
			if err = util.CygwinPerformHandshake(conn, nonce); err != nil {
				log.Printf("Unable to perform handshake on Cygwin socket: %s", err)
			}
			c.wg.Add(1)
			go func() {
				defer c.wg.Done()
				defer conn.Close()
				id := time.Now().UnixNano() // create unique id for debug tracing
				log.Printf("[%d] Accepted request from %s", id, socketName)
				if err := serveSSH(id, conn, c.locked); err != nil {
					log.Printf("[%d] SSH handler returned error: %s", id, err.Error())
				}
			}()
		}
	}()
	return nil
}

func makeInheritSaWithSid() *windows.SecurityAttributes {
	var sa windows.SecurityAttributes
	u, err := user.Current()
	if err == nil {
		sd, err := windows.SecurityDescriptorFromString("O:" + u.Uid)
		if err == nil {
			sa.SecurityDescriptor = sd
		}
	}
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1
	return &sa
}

var mapCounter uint64

func queryPageant(req []byte) ([]byte, error) {

	const (
		invalidHandleValue = ^windows.Handle(0)
		pageReadWrite      = 0x4
		fileMapWrite       = 0x2
		pageantMagic       = 0x804e50ba
	)

	hwnd := win.FindWindow(windows.StringToUTF16Ptr("Pageant"), windows.StringToUTF16Ptr("Pageant"))
	if hwnd == 0 {
		return nil, errors.New("could not find Pageant window")
	}

	mapName := fmt.Sprintf("pgnt%08x", atomic.AddUint64(&mapCounter, 1))

	fileMap, err := windows.CreateFileMapping(
		invalidHandleValue,
		makeInheritSaWithSid(),
		pageReadWrite,
		0,
		util.MaxAgentMsgLen,
		windows.StringToUTF16Ptr(mapName))
	if err != nil {
		return nil, err
	}
	//nolint:errcheck
	defer windows.CloseHandle(fileMap)

	sharedMemory, err := windows.MapViewOfFile(fileMap, fileMapWrite, 0, 0, 0)
	if err != nil {
		return nil, err
	}
	//nolint:errcheck
	defer windows.UnmapViewOfFile(sharedMemory)

	sharedMemoryArray := (*[util.MaxAgentMsgLen]byte)(unsafe.Pointer(sharedMemory))
	binary.BigEndian.PutUint32(sharedMemoryArray[:4], uint32(len(req)))
	copy(sharedMemoryArray[4:], req)

	mapNameWithNul := mapName + "\000"

	// copyDataStruct is used to pass data in the WM_COPYDATA message.
	type copyDataStruct struct {
		dwData uintptr
		cbData uint32
		lpData uintptr
	}

	cds := copyDataStruct{
		dwData: pageantMagic,
		cbData: uint32(((*reflect.StringHeader)(unsafe.Pointer(&mapNameWithNul))).Len),
		lpData: ((*reflect.StringHeader)(unsafe.Pointer(&mapNameWithNul))).Data,
	}
	ret := win.SendMessage(hwnd, win.WM_COPYDATA, 0, uintptr(unsafe.Pointer(&cds)))
	if ret == 0 {
		return nil, errors.New("unable to send WM_COPYDATA")
	}

	len := binary.BigEndian.Uint32(sharedMemoryArray[:4])
	result := make([]byte, len)
	copy(result, sharedMemoryArray[4:len+4])

	return result, nil
}

func serveSSH(id int64, from io.ReadWriter, locked *int32) error {

	const (
		agentFailure = 5
		agentSuccess = 6
	)

	var length [4]byte
	for {
		if _, err := io.ReadFull(from, length[:]); err != nil {
			if errors.Is(err, io.EOF) {
				return err
			}
			return nil // Done
		}
		l := binary.BigEndian.Uint32(length[:])
		if l == 0 {
			return fmt.Errorf("agent: request size is 0")
		}
		if l > util.MaxAgentMsgLen-4 {
			// We also cap requests.
			return fmt.Errorf("agent: request too large: %d", l)
		}

		req := make([]byte, l)
		if _, err := io.ReadFull(from, req); err != nil {
			return err
		}

		var (
			resp []byte
			err  error
		)
		if locked != nil && atomic.LoadInt32(locked) == 1 {
			log.Print("Session is locked")
			resp = []byte{agentFailure}
		} else {
			resp, err = queryPageant(req)
			if err != nil {
				log.Printf("[%d] Unable to process ssh request via Pageant: %s", id, err.Error())
				resp = []byte{agentFailure}
			}
			if len(resp) > util.MaxAgentMsgLen-4 {
				return fmt.Errorf("agent: reply too large: %d bytes", len(resp))
			}
			if len(resp) == 0 {
				resp = []byte{agentSuccess}
			}
		}

		binary.BigEndian.PutUint32(length[:], uint32(len(resp)))
		if _, err := from.Write(length[:]); err != nil {
			return err
		}
		if _, err := from.Write(resp); err != nil {
			return err
		}
	}
}

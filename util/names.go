package util

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

// Shared names.
const (
	SSHAgentPipeName = "\\\\.\\pipe\\openssh-ssh-agent"
	MaxNameLen       = windows.UNIX_PATH_MAX

	// openssh-portable has it at 256 * 1024.
	// gpg-agent is using 16 * 1024.
	// Putty seems to have it at 8 + 1024.
	MaxAgentMsgLen = 256 * 1024

	GPGAgentName             = "gpg-agent"
	WinAgentName             = "agent-gui"
	SocketAgentName          = "S." + GPGAgentName
	SocketAgentBrowserName   = "S." + GPGAgentName + ".browser"
	SocketAgentExtraName     = "S." + GPGAgentName + ".extra"
	SocketAgentSSHName       = "S." + GPGAgentName + ".ssh"
	SocketAgentSSHCygwinName = "S." + GPGAgentName + ".ssh.cyg"
)

// PrepareWindowsPath prepares Windows path for use on unix shell line without quoting.
func PrepareWindowsPath(path string) string {
	return filepath.ToSlash(path)
}

// FileExists check if file exists.
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return info.Mode().IsRegular()
}

// WaitForFileArrival checks for files existence once a second for requested waiting period.
func WaitForFileArrival(period time.Duration, filenames ...string) bool {

	l := len(filenames)
	if l == 0 {
		return true
	}

	fns := make([]string, l)
	copy(fns, filenames)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	tCur := time.Now()
	tEnd := tCur.Add(period)
	for ; tCur.Before(tEnd); tCur = <-ticker.C {
		for i := 0; i < len(fns); i++ {
			if len(fns[i]) == 0 {
				continue
			}
			if FileExists(fns[i]) {
				fns[i] = ""
				l--
			}
		}
		if l == 0 {
			return true
		}
	}
	return false
}

// WaitForFileDeparture attempts to remove files once a second for requested waiting period.
func WaitForFileDeparture(period time.Duration, filenames ...string) {

	l := len(filenames)
	if l == 0 {
		return
	}

	fns := make([]string, l)
	copy(fns, filenames)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	tCur := time.Now()
	tEnd := tCur.Add(period)
	for ; tCur.Before(tEnd) && l > 0; tCur = <-ticker.C {
		for i := 0; i < len(fns); i++ {
			if len(fns[i]) == 0 {
				continue
			}
			if !FileExists(fns[i]) {
				fns[i] = ""
				l--
				continue
			}
			if err := os.Remove(fns[i]); err == nil {
				fns[i] = ""
				l--
			} else {
				log.Printf("Departing file removal problem: %s", err.Error())
			}
		}
	}
}

// IsNetClosing exists because ErrNetClosing is not exported. This is probably going to change in 1.16.
func IsNetClosing(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection")
}

// CygwinNonceString converts binary nonce to printable string in net order.
func CygwinNonceString(nonce [16]byte) string {
	var buf [35]byte
	dst := buf[:]
	for i := 0; i < 4; i++ {
		b := nonce[i*4 : i*4+4]
		hex.Encode(dst[i*9:i*9+8], []byte{b[3], b[2], b[1], b[0]})
		if i != 3 {
			dst[9*i+8] = '-'
		}
	}
	return string(buf[:])
}

// CygwinCreateSocketFile creates CygWin socket file with proper content and attributes.
func CygwinCreateSocketFile(fname string, port int) (nonce [16]byte, err error) {
	if _, err = rand.Read(nonce[:]); err != nil {
		return
	}
	if err = ioutil.WriteFile(fname, []byte(fmt.Sprintf("!<socket >%d s %s", port, CygwinNonceString(nonce))), 0600); err != nil {
		return
	}
	var cpath *uint16
	if cpath, err = windows.UTF16PtrFromString(fname); err != nil {
		return
	}
	err = windows.SetFileAttributes(cpath, windows.FILE_ATTRIBUTE_SYSTEM|windows.FILE_ATTRIBUTE_READONLY)
	return
}

// CygwinPerformHandshake exchanges handshake data.
func CygwinPerformHandshake(conn io.ReadWriter, nonce [16]byte) error {

	var nonceR [16]byte
	if _, err := conn.Read(nonceR[:]); err != nil {
		return err
	}
	if !bytes.Equal(nonce[:], nonceR[:]) {
		log.Printf("Wrong nonce received - expecting %x but got %x", nonce[:], nonceR[:])
		return errors.New("invalid nonce received")
	}
	if _, err := conn.Write(nonce[:]); err != nil {
		return err
	}

	// read client pid:uid:gid
	buf := make([]byte, 12)
	if _, err := conn.Read(buf); err != nil {
		return err
	}

	// Send back our info, making sure that gid:uid are the same as received
	binary.LittleEndian.PutUint32(buf, uint32(os.Getpid()))
	if _, err := conn.Write(buf); err != nil {
		return err
	}
	return nil
}
